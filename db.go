package main

import (
	"errors"
	"log"
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

	evictUntilMemFreed := func(samples []sample) bool {
		for _, s := range samples {
			log.Println("evicting: ", s.k)
			db.Delete(s.k)
			if enoughMemFreed() {
				break
			}
		}
		return false
	}

	switch state.conf.eviction {
	case AllKeysRandom:
		evictUntilMemFreed(samples)
	}
	return nil
}

func (i *Item) shouldExpire() bool {
	return (i.exp.Unix() != UNIX_TS_EPOCH && time.Until(i.exp).Seconds() <= 0)
}

func (db *Database) tryExpire(k string, i *Item) bool {
	if i.shouldExpire() {
		DB.mu.Lock()
		DB.Delete(k)
		DB.mu.Unlock()
		return true
	}

	return false
}

func (db *Database) Get(k string) (i *Item, ok bool) {
	db.mu.RLock()
	item, ok := db.store[k]
	if !ok {
		return item, ok
	}

	expired := db.tryExpire(k, item)
	if expired {
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

type Item struct {
	V          string
	exp        time.Time
	LastAccess time.Time
	Accesses   int
}

func (k *Item) approxMemUsage(name string) int64 {
	stringHeader := 16
	expHeader := 24
	mapEntrySize := 32

	return int64(stringHeader + len(name) + stringHeader + len(k.V) + expHeader + mapEntrySize)
}

type Transaction struct {
	cmds []*TxCommand
}

func NewTransaction() *Transaction {
	return &Transaction{}
}

type TxCommand struct {
	v       *Value
	handler Handler
}
