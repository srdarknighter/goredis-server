package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

func main() {
	log.Println("reading conf file")
	conf := readConf("./redis.conf")

	state := NewAppState(conf)

	if conf.aofEnabled {
		log.Println("syncing AOF records")
		state.aof.Sync()
	}

	l, err := net.Listen("tcp", ":6379")
	if err != nil {
		log.Fatal("cannot connect on port :6379")
	}
	defer l.Close()
	log.Println("listening on :6379")

	conn, err := l.Accept()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer conn.Close()
	log.Println("connection accepted")

	for {
		v := Value{typ: ARRAY}
		v.readArray(conn)
		handle(conn, &v, state)
	}
}

type AppState struct { // defines the app state with conf + aof rules
	conf *Config
	aof  *Aof
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
