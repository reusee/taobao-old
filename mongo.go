package main

import (
	"strconv"
	"strings"
	"time"

	"github.com/reusee/mgo"

	"gopkg.in/mgo.v2/bson"
)

type Mongo struct {
	session                          *mgo.Session
	db                               *mgo.Database
	date                             string
	jobsColle, itemsColle, rawsColle *mgo.Collection
	catsColle                        *mgo.Collection
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

	catsColle := db.C("cats")
	err = catsColle.EnsureIndex(mgo.Index{
		Key:    []string{"cat"},
		Unique: true,
	})
	ce(err, "ensure index")

	return &Mongo{
		session:    session,
		db:         db,
		date:       date,
		jobsColle:  jobsColle,
		itemsColle: itemsColle,
		rawsColle:  rawsColle,
		catsColle:  catsColle,
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

func (m *Mongo) AddCat(cat Cat) error {
	_, err := m.catsColle.Upsert(bson.M{"cat": cat.Cat}, cat)
	return err
}

func (m *Mongo) Stats() {
	itemsColle := m.db.C("items_" + m.date)

	catStatsColle := m.db.C("cat_stats_" + m.date)
	err := catStatsColle.Create(&mgo.CollectionInfo{
		Extra: bson.M{
			"compression": "zlib",
		},
	})
	ce(ignoreExistsColle(err), "create cat_stats")
	err = catStatsColle.EnsureIndex(mgo.Index{
		Key:    []string{"cat"},
		Unique: true,
		Sparse: true,
	})
	type CatStat struct {
		Cat    int
		Count  int
		Amount float64
	}
	catStats := make(map[int]*CatStat)

	query := itemsColle.Find(nil)
	iter := query.Iter()
	var item Item
	for iter.Next(&item) {
		item.View_sales = strings.Replace(item.View_sales, "人收货", "", -1)
		item.View_sales = strings.Replace(item.View_sales, "人付款", "", -1)
		count, err := strconv.Atoi(item.View_sales)
		ce(err, sp("parse count %s", item.View_sales))
		price, err := strconv.ParseFloat(item.View_price, 64)
		ce(err, sp("parse price %s", item.View_price))
		amount := price * float64(count)
		for _, src := range item.Sources {
			cat := src.Cat
			if _, ok := catStats[cat]; !ok {
				catStats[cat] = &CatStat{
					Cat: cat,
				}
			}
			catStats[cat].Count += count
			catStats[cat].Amount += amount
		}

	}

	for _, stat := range catStats {
		_, err = catStatsColle.Upsert(bson.M{"cat": stat.Cat}, stat)
		ce(err, "insert cat stat")
	}

}

func ignoreExistsColle(err error) error {
	if err, ok := err.(*mgo.QueryError); ok {
		if err.Message == "collection already exists" {
			return nil
		}
	}
	return err
}

func allowDup(err error) error {
	if err, ok := err.(*mgo.LastError); ok {
		if err.Code == 11000 {
			return nil
		}
	}
	return err
}
