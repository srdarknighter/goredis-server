package main

import (
	"fmt"
	"net"
)

type Handler func(*Value, *AppState) *Value // type defn for the map

var Handlers = map[string]Handler{
	"COMMAND": command,
	"GET":     get,
	"SET":     set,
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
	w.Flush()
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

	DB.mu.Unlock()

	return &Value{typ: STRING, str: "OK"}
}
