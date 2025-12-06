package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"path"
	"time"
)

type SnapshotTracker struct {
	keys   int
	ticker time.Ticker
	rdb    *RDBSnapshot
}

func NewSnapshotTracker(rdb *RDBSnapshot) *SnapshotTracker {
	return &SnapshotTracker{
		keys:   0,
		ticker: *time.NewTicker(time.Second * time.Duration(rdb.Secs)),
		rdb:    rdb,
	}
}

var trackers = []*SnapshotTracker{}

func InitRDBTrackers(state *AppState) {
	for _, rdb := range state.conf.rdb {
		tracker := NewSnapshotTracker(&rdb)
		trackers = append(trackers, tracker)

		go func() {
			defer tracker.ticker.Stop()

			for range tracker.ticker.C {
				if tracker.keys >= tracker.rdb.KeysChanged {
					log.Printf("keys changed: %d - keys required to change: %d", tracker.keys, tracker.rdb.KeysChanged)
					SaveRDB(state)
				}
				tracker.keys = 0
			}
		}()
	}
}

func IncrRDBTrackers() {
	for _, t := range trackers {
		t.keys++
	}
}

func SaveRDB(state *AppState) {
	fp := path.Join(state.conf.dir, state.conf.rdbFn)
	f, err := os.OpenFile(fp, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("error opening rdb file: ", err)
		return
	}

	defer f.Close()

	err = gob.NewEncoder(f).Encode(&DB.store)
	if err != nil {
		fmt.Println("error saving rdb file: ", err)
		return
	}
	log.Println("saved RDB file")
}

func SyncRDB(conf *Config) {
	fp := path.Join(conf.dir, conf.rdbFn)
	f, err := os.OpenFile(fp, os.O_CREATE|os.O_RDONLY, 0644)
	if err != nil {
		fmt.Println("error opening rdb file: ", err)
		return
	}
	defer f.Close()

	err = gob.NewDecoder(f).Decode(&DB.store)
	if err != nil {
		fmt.Println("error reading rdb file: ", err)
		return
	}
}
