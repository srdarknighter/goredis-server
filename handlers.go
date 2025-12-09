package main

import (
	"log"
	"maps"
	"path/filepath"
	"strconv"
	"time"
)

type Handler func(*Client, *Value, *AppState) *Value // type defn for the map

var Handlers = map[string]Handler{
	"COMMAND":      command,
	"GET":          get,
	"SET":          set,
	"DEL":          del,
	"EXISTS":       exists,
	"KEYS":         keys,
	"SAVE":         save,
	"BGSAVE":       bgsave,
	"DBSIZE":       dbsize,
	"FLUSHDB":      flushdb,
	"AUTH":         auth,
	"EXPIRE":       expire,
	"TTL":          ttl,
	"BGREWRITEAOF": bgrewriteaof,
	"MULTI":        multi,
	"EXEC":         _exec,
	"DISCARD":      discard,
} // map to store the commands and their implementations

var SafeCmds = []string{
	"COMMAND",
	"AUTH",
}

func handle(c *Client, v *Value, state *AppState) {
	cmd := v.array[0].bulk       // it's a command like GET, SET, etc
	handler, ok := Handlers[cmd] // handler is the functional implementation of cmd in a map, stores cmd and its functional implementation
	w := NewWriter(c.conn)       // creating a new writer with conn object

	if !ok {
		w.Write(&Value{typ: ERROR, err: "ERR invalid command"})
		w.Flush()
		return
	}

	if state.conf.requirepass && !c.authenticated && !contains(SafeCmds, cmd) {
		w.Write(&Value{typ: ERROR, err: "NOAUTH authentication required"})
		w.Flush()
		return
	}

	//queue the command if in a transaction
	if state.tx != nil && cmd != "EXEC" && cmd != "DISCARD" {
		txcmd := TxCommand{v: v, handler: handler}
		state.tx.cmds = append(state.tx.cmds, &txcmd)
		w.Write(&Value{typ: STRING, str: "QUEUED"})
		w.Flush()
		return
	}

	reply := handler(c, v, state) // calling the function of cmd with v as argument
	w.Write(reply)                // converting reply to resp protocol
	w.Flush()                     // flushing to the CLI
}

func command(c *Client, v *Value, state *AppState) *Value {
	return &Value{typ: STRING, str: "OK"}
}

func get(c *Client, v *Value, state *AppState) *Value {
	args := v.array[1:]
	if len(args) != 1 {
		return &Value{typ: ERROR, err: "ERR invalid number of arguments for 'GET' function"}
	}

	name := args[0].bulk

	item, ok := DB.Get(name)
	if !ok {
		return &Value{typ: NULL}
	}

	return &Value{typ: BULK, bulk: item.V}
}

func set(c *Client, v *Value, state *AppState) *Value {
	args := v.array[1:]
	if len(args) != 2 {
		return &Value{typ: ERROR, err: "ERR invalid number of arguments for 'SET' function"}
	}
	key := args[0].bulk
	val := args[1].bulk

	DB.mu.Lock()
	err := DB.Set(key, val, state)
	if err != nil {
		DB.mu.Unlock()
		return &Value{typ: ERROR, err: "Error " + err.Error()}
	}

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

func del(c *Client, v *Value, state *AppState) *Value {
	args := v.array[1:]
	var n int

	DB.mu.Lock()
	for _, arg := range args {
		_, ok := DB.store[arg.bulk]
		DB.Delete(arg.bulk)
		if ok {
			n++
		}
	}
	DB.mu.Unlock()

	return &Value{typ: INTEGER, num: n}
}

func exists(c *Client, v *Value, state *AppState) *Value {
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

func keys(c *Client, v *Value, state *AppState) *Value {
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

func save(c *Client, v *Value, state *AppState) *Value {
	SaveRDB(state)
	return &Value{typ: STRING, str: "OK"}
}

func bgsave(c *Client, v *Value, state *AppState) *Value {
	// uses copy-on-write algorithm can't implement in go cuz of garbage collector
	if state.bgsaveRunning {
		return &Value{typ: ERROR, err: "ERR background saving already in progress"}
	}

	cp := make(map[string]*Item, len(DB.store))

	DB.mu.RLock()
	maps.Copy(cp, DB.store)
	DB.mu.RUnlock()

	state.bgsaveRunning = true
	state.dbCopy = cp

	go func() {
		defer func() {
			state.bgsaveRunning = false
			state.dbCopy = nil
		}()

		SaveRDB(state)
	}()

	return &Value{typ: STRING, str: "OK"}
}

func dbsize(c *Client, v *Value, state *AppState) *Value {
	DB.mu.RLock()
	size := len(DB.store)
	DB.mu.RUnlock()

	return &Value{typ: INTEGER, num: size}
}

func flushdb(c *Client, v *Value, state *AppState) *Value {
	DB.mu.Lock()
	DB.store = map[string]*Item{}
	DB.mu.Unlock()

	return &Value{typ: STRING, str: "OK"}
}

func auth(c *Client, v *Value, state *AppState) *Value {
	args := v.array[1:]
	if len(args) != 1 {
		return &Value{typ: ERROR, err: "ERR invalid number of keywords for 'AUTH'"}
	}

	p := args[0].bulk
	if state.conf.password == p {
		c.authenticated = true
		return &Value{typ: STRING, str: "OK"}
	} else {
		c.authenticated = false
		return &Value{}
	}
}

func expire(c *Client, v *Value, state *AppState) *Value {
	args := v.array[1:]
	if len(args) != 2 {
		return &Value{typ: ERROR, err: "ERR invalid number of arguments for 'EXPIRE' function"}
	}

	k := args[0].bulk
	exp := args[1].bulk

	expSecs, err := strconv.Atoi(exp)
	if err != nil {
		return &Value{typ: ERROR, err: "ERR invalid expiration time in 'EXPIRE' function"}
	}
	DB.mu.RLock()
	key, ok := DB.store[k]
	if !ok {
		return &Value{typ: INTEGER, num: 0}
	}
	key.exp = time.Now().Add(time.Duration(expSecs) * time.Second)
	DB.mu.RUnlock()

	return &Value{typ: INTEGER, num: 1}
}

func ttl(c *Client, v *Value, state *AppState) *Value {
	args := v.array[1:]
	if len(args) != 1 {
		return &Value{typ: ERROR, err: "ERR invalid number of arguments for 'TTL' function"}
	}

	k := args[0].bulk

	DB.mu.RLock()
	key, ok := DB.store[k]
	if !ok {
		return &Value{typ: INTEGER, num: -2}
	}
	exp := key.exp
	DB.mu.RUnlock()

	if exp.Unix() == UNIX_TS_EPOCH {
		return &Value{typ: INTEGER, num: -1}
	}

	expSecs := int(time.Until(exp).Seconds())
	if expSecs <= 0 {
		DB.mu.Lock()
		DB.Delete(k)
		DB.mu.Unlock()
		return &Value{typ: INTEGER, num: -2}
	}
	return &Value{typ: INTEGER, num: expSecs}
}

func bgrewriteaof(c *Client, v *Value, state *AppState) *Value {
	go func() {
		DB.mu.RLock()
		cp := make(map[string]*Item, len(DB.store))
		maps.Copy(cp, DB.store)
		DB.mu.RUnlock()

		state.aof.Rewrite(cp)
	}()

	return &Value{typ: STRING, str: "Background AOF rewriting started"}
}

func multi(c *Client, v *Value, state *AppState) *Value {
	if state.tx != nil {
		return &Value{typ: ERROR, err: "ERR nested multi-bulk transactions are not allowed"}
	}

	state.tx = NewTransaction()
	return &Value{typ: STRING, str: "OK"}
}

func _exec(c *Client, v *Value, state *AppState) *Value {
	if state.tx == nil {
		return &Value{typ: ERROR, err: "ERR EXEC without MULTI"}
	}

	replies := make([]Value, len(state.tx.cmds))

	for i, cmd := range state.tx.cmds {
		reply := cmd.handler(c, cmd.v, state)
		replies[i] = *reply
	}

	reply := Value{typ: ARRAY, array: replies}
	state.tx = nil
	return &reply
}

func discard(c *Client, v *Value, state *AppState) *Value {
	if state.tx == nil {
		return &Value{typ: ERROR, err: "ERR DISCARD without MULTI"}
	}

	state.tx = nil
	return &Value{typ: STRING, str: "OK"}
}
