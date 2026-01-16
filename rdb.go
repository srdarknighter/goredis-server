package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
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
	f, err := os.Open(fp)
	if err != nil {
		fmt.Println("error opening rdb file: ", err)
		f.Close()
		return
	}

	defer f.Close()

	var buf bytes.Buffer
	if state.bgsaveRunning {
		err = gob.NewEncoder(&buf).Encode(&state.dbCopy)
	} else {
		DB.mu.RLock()
		err = gob.NewEncoder(&buf).Encode(&DB.store) // since we lock the file here for writers, other clients can't put data into it, better to use 'BGSAVE'
		DB.mu.RUnlock()
	}

	if err != nil {
		fmt.Println("error encoding rdb file: ", err)
		return
	}

	data := buf.Bytes()
	bsum, err := Hash(&buf)
	if err != nil {
		log.Println("rdb - cannot compute buf checksum: ", err)
		return
	}

	_, err = f.Write(data)
	if err != nil {
		log.Println("rdb - cannot write to file: ", err)
		return
	}

	if err := f.Sync(); err != nil { // to prevent the os from keeping it in buffer temporarily
		log.Println("rdb - cannot flush file to disk: ", err)
		return
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil { // moving file ptr to beginning to calculate checksum
		log.Println("rdb - cannot seek file: ", err)
		return
	}

	fsum, err := Hash(f)
	if err != nil {
		log.Println("rdb - cannot compute file checksum: ", err)
		return
	}

	if bsum != fsum {
		log.Printf("rdb - buf and file checksums do not match:\nf=%s\nb=%s\n", fsum, bsum)
		return
	}

	log.Println("saved RDB file")

	state.rdbStats.rdb_last_save_ts = time.Now().Unix()
	state.rdbStats.rdb_saves++
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

func Hash(r io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
