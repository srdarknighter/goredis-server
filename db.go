package main

import (
	"sync"
	"time"
)

type Database struct {
	store map[string]*Key
	mu    sync.RWMutex
}

func NewDatabase() *Database {
	return &Database{
		store: map[string]*Key{},
		mu:    sync.RWMutex{},
	}
}

func (db *Database) Set(k string, v string) {
	DB.store[k] = &Key{V: v}
}

func (db *Database) Delete(k string) {
	delete(DB.store, k)
}

var DB = NewDatabase()

type Key struct {
	V   string
	exp time.Time
}
