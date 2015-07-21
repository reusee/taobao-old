package taobao

import (
	"encoding/json"
	"net/http"

	_ "net/http/pprof"
)

type TraceInfo struct {
	What    string
	Entries []*Entry
}

func StartWeb() {
	http.Handle("/", http.FileServer(http.Dir("./web")))

	http.HandleFunc("/jobs.json", func(w http.ResponseWriter, r *http.Request) {
		var infos []TraceInfo
		jobTraceSet.Dump(w, func(trace *Trace) bool {
			return !trace.Flag("done")
		}, func(t *Trace) {
			infos = append(infos, TraceInfo{
				What:    t.what,
				Entries: t.Entries(),
			})
		})
		bs, err := json.Marshal(infos)
		ce(err, "marshal")
		w.Write(bs)
	})

	http.HandleFunc("/done_jobs.json", func(w http.ResponseWriter, r *http.Request) {
		var infos []TraceInfo
		jobTraceSet.Dump(w, func(trace *Trace) bool {
			return trace.Flag("done")
		}, func(t *Trace) {
			infos = append(infos, TraceInfo{
				What:    t.what,
				Entries: t.Entries(),
			})
		})
		bs, err := json.Marshal(infos)
		ce(err, "marshal")
		w.Write(bs)
	})

}
