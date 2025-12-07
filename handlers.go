package main

import (
	"fmt"
	"log"
	"maps"
	"net"
	"path/filepath"
)

type Handler func(*Value, *AppState) *Value // type defn for the map

var Handlers = map[string]Handler{
	"COMMAND": command,
	"GET":     get,
	"SET":     set,
	"DEL":     del,
	"EXISTS":  exists,
	"KEYS":    keys,
	"SAVE":    save,
	"BGSAVE":  bgsave,
	"DBSIZE":  dbsize,
	"FLUSHDB": flushdb,
} // map to store the commands and their implementations

func handle(conn net.Conn, v *Value, state *AppState) {
	cmd := v.array[0].bulk       // it's a command like GET, SET, etc
	handler, ok := Handlers[cmd] // handler is the functional implementation of cmd in a map, stores cmd and its functional implementation

	if !ok {
		fmt.Println("invalid command: ", cmd)
		return
	}

	reply := handler(v, state) // calling the function of cmd with v as argument
	w := NewWriter(conn)       // creating a new writer with conn object
	w.Write(reply)             // converting reply to resp protocol
	w.Flush()                  // flushing to the CLI
}

func command(v *Value, state *AppState) *Value {
	return &Value{typ: STRING, str: "OK"}
}

func get(v *Value, state *AppState) *Value {
	args := v.array[1:]
	if len(args) != 1 {
		return &Value{typ: ERROR, err: "ERR invalid number of arguments for 'GET' function"}
	}

	name := args[0].bulk

	DB.mu.RLock()
	val, ok := DB.store[name]
	DB.mu.RUnlock()

	if !ok {
		return &Value{typ: NULL}
	}
	return &Value{typ: BULK, bulk: val}
}

func set(v *Value, state *AppState) *Value {
	args := v.array[1:]
	if len(args) != 2 {
		return &Value{typ: ERROR, err: "ERR invalid number of arguments for 'SET' function"}
	}
	key := args[0].bulk
	val := args[1].bulk

	DB.mu.Lock()
	DB.store[key] = val

	if state.conf.aofEnabled {
		state.aof.w.Write(v)

		if state.conf.aofFsync == Always {
			state.aof.w.Flush()
		}
	}

	if len(state.conf.rdb) > 0 {
		IncrRDBTrackers()
	}

	DB.mu.Unlock()

	return &Value{typ: STRING, str: "OK"}
}

func del(v *Value, state *AppState) *Value {
	args := v.array[1:]
	var n int

	DB.mu.Lock()
	for _, arg := range args {
		_, ok := DB.store[arg.bulk]
		delete(DB.store, arg.bulk)
		if ok {
			n++
		}
	}
	DB.mu.Unlock()

	return &Value{typ: INTEGER, num: n}
}

func exists(v *Value, state *AppState) *Value {
	args := v.array[1:]
	var n int

	DB.mu.RLock()
	for _, arg := range args {
		_, ok := DB.store[arg.bulk]
		if ok {
			n++
		}
	}
	DB.mu.RUnlock()

	return &Value{typ: INTEGER, num: n}
}

func keys(v *Value, state *AppState) *Value {
	args := v.array[1:]
	if len(args) != 1 {
		return &Value{typ: ERROR, err: "ERR invalid number of arguments for 'KEYS' command"}
	}
	pattern := args[0].bulk

	DB.mu.RLock()
	var matches []string

	for key := range DB.store {
		matched, err := filepath.Match(pattern, key)
		if err != nil {
			log.Printf("error matching keys: (pattern: %s), (key: %s) - %v", pattern, key, err)
			continue
		}

		if matched {
			matches = append(matches, key)
		}
	}
	DB.mu.RUnlock()

	reply := Value{typ: ARRAY}

	for _, m := range matches {
		reply.array = append(reply.array, Value{typ: BULK, bulk: m})
	}
	return &reply
}

func save(v *Value, state *AppState) *Value {
	SaveRDB(state)
	return &Value{typ: STRING, str: "OK"}
}

func bgsave(v *Value, state *AppState) *Value {
	// uses copy-on-write algorithm can't implement in go cuz of garbage collector
	if state.bgsaveRunning {
		return &Value{typ: ERROR, err: "ERR background saving already in progress"}
	}

	c := make(map[string]string, len(DB.store))

	DB.mu.RLock()
	maps.Copy(c, DB.store)
	DB.mu.RUnlock()

	state.bgsaveRunning = true
	state.dbCopy = c

	go func() {
		defer func() {
			state.bgsaveRunning = false
			state.dbCopy = nil
		}()

		SaveRDB(state)
	}()

	return &Value{typ: STRING, str: "OK"}
}

func dbsize(v *Value, state *AppState) *Value {
	DB.mu.RLock()
	size := len(DB.store)
	DB.mu.RUnlock()

	return &Value{typ: INTEGER, num: size}
}

func flushdb(v *Value, state *AppState) *Value {
	DB.mu.Lock()
	DB.store = map[string]string{}
	DB.mu.Unlock()

	return &Value{typ: STRING, str: "OK"}
}
