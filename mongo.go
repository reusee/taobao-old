package main

import (
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type Mongo struct {
	session                          *mgo.Session
	db                               *mgo.Database
	jobsColle, itemsColle, rawsColle *mgo.Collection
}

func NewMongo() (m *Mongo, err error) {
	defer ct(&err)
	// database
	session, err := mgo.Dial("127.0.0.1")
	ce(err, "connect to db")
	db := session.DB("taobao")

	now := time.Now()
	date := sp("%04d%02d%02d", now.Year(), now.Month(), now.Day())

	// collections
	jobsColle := db.C("jobs_" + date)
	err = jobsColle.Create(&mgo.CollectionInfo{
		Extra: bson.M{
			"compression": "zlib",
		},
	})
	ce(ignoreExistsColle(err), "create jobs collection")
	err = jobsColle.EnsureIndex(mgo.Index{
		Key:    []string{"cat", "page"},
		Unique: true,
		Sparse: true,
	})
	ce(err, "ensure jobs collection index")
	err = jobsColle.EnsureIndexKey("done")
	ce(err, "ensure jobs collection done key")
	err = jobsColle.EnsureIndexKey("cat")
	ce(err, "ensure jobs collection cat key")
	err = jobsColle.EnsureIndexKey("page")
	ce(err, "ensure jobs collection page key")

	itemsColle := db.C("items_" + date)
	err = itemsColle.Create(&mgo.CollectionInfo{
		Extra: bson.M{
			"compression": "zlib",
		}})
	ce(ignoreExistsColle(err), "create items collection")
	err = itemsColle.EnsureIndex(mgo.Index{
		Key:    []string{"nid"},
		Unique: true,
		Sparse: true,
	})
	ce(err, "ensure items collection index")

	rawsColle := db.C("raws_" + date)
	err = rawsColle.Create(&mgo.CollectionInfo{
		Extra: bson.M{
			"compression": "zlib",
		},
	})
	ce(ignoreExistsColle(err), "create raws collection")
	err = rawsColle.EnsureIndex(mgo.Index{
		Key:    []string{"cat", "page"},
		Unique: true,
		Sparse: true,
	})
	ce(err, "ensure raws index")

	return &Mongo{
		session:    session,
		db:         db,
		jobsColle:  jobsColle,
		itemsColle: itemsColle,
		rawsColle:  rawsColle,
	}, nil
}

func (m *Mongo) Close() {
	m.session.Close()
}

func (m *Mongo) AddJob(job Job) error {
	return allowDup(m.jobsColle.Insert(job))
}

func (m *Mongo) DoneJob(job Job) error {
	return m.jobsColle.Update(bson.M{"cat": job.Cat, "page": job.Page},
		bson.M{"$set": bson.M{"done": true}})
}

func (m *Mongo) GetJobs() (jobs []Job, err error) {
	err = m.jobsColle.Find(bson.M{"done": false}).All(&jobs)
	return
}

func (m *Mongo) AddItem(item Item, job Job) (err error) {
	defer ct(&err)
	err = m.itemsColle.Insert(item)
	ce(allowDup(err), "insert item")
	err = m.itemsColle.Update(bson.M{
		"nid": item.Nid,
	}, bson.M{
		"$addToSet": bson.M{
			"sources": Source{
				Cat:  job.Cat,
				Page: job.Page,
			},
		},
	})
	ce(err, "add source to item")
	return
}
