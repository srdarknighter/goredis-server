package main

import (
	"errors"
	"log"
	"sort"
	"sync"
	"time"
)

type Database struct {
	store map[string]*Item
	mu    sync.RWMutex
	mem   int64
}

func NewDatabase() *Database {
	return &Database{
		store: map[string]*Item{},
		mu:    sync.RWMutex{},
	}
}

func (db *Database) evictKeys(state *AppState, requiredMem int64) error {
	if state.conf.eviction == NoEviction {
		return errors.New("memory limit reached")
	}

	samples := sampleKeys(state)

	enoughMemFreed := func() bool {
		if db.mem+requiredMem <= state.conf.maxmem {
			return true
		} else {
			return false
		}
	}

	evictUntilMemFreed := func(samples []sample) int {
		var n int
		for _, s := range samples {
			log.Println("evicting: ", s.k)
			db.Delete(s.k)
			n++
			if enoughMemFreed() {
				break
			}
		}
		return n
	}

	switch state.conf.eviction {
	case AllKeysRandom:
		evictionKeys := evictUntilMemFreed(samples)
		state.generalStats.evicted_keys += evictionKeys
	case AllKeysLRU:
		sort.Slice(samples, func(i int, j int) bool {
			return samples[i].v.LastAccess.After(samples[j].v.LastAccess)
		})
		evictionKeys := evictUntilMemFreed(samples)
		state.generalStats.evicted_keys += evictionKeys
	case AllKeysLFU:
		sort.Slice(samples, func(i int, j int) bool {
			return samples[i].v.Accesses < samples[j].v.Accesses
		})
		evictionKeys := evictUntilMemFreed(samples)
		state.generalStats.evicted_keys += evictionKeys
	}
	return nil
}

func (db *Database) tryExpire(k string, i *Item, state *AppState) bool {
	if i.shouldExpire() {
		DB.mu.Lock()
		DB.Delete(k)
		DB.mu.Unlock()
		state.generalStats.expired_keys++
		return true
	}

	return false
}

func (db *Database) Get(k string, state *AppState) (i *Item, ok bool) {
	db.mu.RLock()
	item, ok := db.store[k]
	if !ok {
		db.mu.RUnlock()
		return item, ok
	}

	expired := db.tryExpire(k, item, state)
	if expired {
		db.mu.RUnlock()
		return &Item{}, false
	}
	item.Accesses++
	item.LastAccess = time.Now()
	db.mu.RUnlock()

	log.Printf("item %s accessed %d times at: %v", k, item.Accesses, item.LastAccess)

	return item, ok
}

func (db *Database) Set(k string, v string, state *AppState) error {
	if old, ok := db.store[k]; ok {
		oldmem := old.approxMemUsage(k)
		db.mem -= oldmem
	}

	key := &Item{V: v}
	kmem := key.approxMemUsage(k)

	outOfMem := state.conf.maxmem > 0 && db.mem+kmem > state.conf.maxmem
	if outOfMem {
		err := db.evictKeys(state, db.mem+kmem)
		if err != nil {
			return err
		}
	}

	db.store[k] = key
	db.mem += kmem
	log.Println("memory: ", db.mem)

	if db.mem > state.peakMem {
		state.peakMem = db.mem
	}

	return nil
}

func (db *Database) Delete(k string) {
	key, ok := db.store[k]
	if !ok {
		return
	}
	kmem := key.approxMemUsage(k)
	delete(DB.store, k)
	db.mem -= kmem
	log.Println("memory: ", db.mem)
}

var DB = NewDatabase()
