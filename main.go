package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
)

const UNIX_TS_EPOCH int64 = -62135596800 // this is the unix timestamp of 1970-01-01 00:00:00 UTC, used to check if a key has expired

func main() {
	log.Println("reading conf file")
	conf := readConf("./redis.conf")

	state := NewAppState(conf)

	if conf.aofEnabled {
		log.Println("syncing AOF records")
		state.aof.Sync()
	}

	if len(conf.rdb) > 0 {
		SyncRDB(conf)
		InitRDBTrackers(state)
	}

	l, err := net.Listen("tcp", ":6379")
	if err != nil {
		log.Fatal("cannot connect on port :6379")
	}
	defer l.Close()
	log.Println("listening on :6379")

	var wg sync.WaitGroup // wait group to prevent pre-mature closing of main loop

	for { // infinite loop to accept connections
		conn, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		log.Println("connection accepted")

		wg.Add(1) // incrementing the start of a routine
		go func() {
			handleConn(conn, state)
			wg.Done()
		}()
	}
	wg.Wait() // won't matter since infinite loop
}

func handleConn(conn net.Conn, state *AppState) {
	log.Println("accepted new connections: ", conn.LocalAddr().String())

	c := NewClient(conn)
	r := bufio.NewReader(conn)

	// removing client from monitor array
	defer func() {
		new := state.monitors[:0]
		for _, mon := range state.monitors {
			if mon != c {
				new = append(new, mon)
			}
		}
		state.monitors = new
	}()

	for {
		v := Value{typ: ARRAY}
		if err := v.readArray(r); err != nil {
			log.Println(err)
			break
		}
		handle(c, &v, state)
	}
	log.Println("connection closed: ", conn.LocalAddr().String())
}
