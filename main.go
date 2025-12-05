package main

import (
	"fmt"
	"log"
	"net"
	"os"
)

func main() {
	log.Println("reading conf file")
	readConf("./redis.conf")

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
		handle(conn, &v)
	}
}
