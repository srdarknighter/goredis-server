package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

const UNIX_TS_EPOCH int64 = -62135596800

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

	for {
		v := Value{typ: ARRAY}
		if err := v.readArray(conn); err != nil {
			log.Println(err)
			break
		}
		handle(c, &v, state)
	}
	log.Println("connection closed: ", conn.LocalAddr().String())
}

type Client struct {
	conn          net.Conn
	authenticated bool
}

func NewClient(conn net.Conn) *Client {
	return &Client{
		conn: conn,
	}
}

type AppState struct { // defines the app state with conf + aof rules
	conf          *Config
	aof           *Aof
	bgsaveRunning bool
	dbCopy        map[string]*Key
}

func NewAppState(conf *Config) *AppState {
	state := AppState{
		conf: conf,
	}

	if conf.aofEnabled {
		state.aof = NewAof(conf)

		if conf.aofFsync == EverySec {
			go func() {
				t := time.NewTicker(time.Second)
				defer t.Stop()

				for range t.C {
					state.aof.w.Flush()
				}
			}()
		}
	}

	return &state
}
