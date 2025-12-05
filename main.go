package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
)

type ValueType string

const (
	ARRAY  ValueType = "*"
	BULK   ValueType = "$"
	STRING ValueType = "+"
	ERROR  ValueType = "-"
	NULL   ValueType = ""
)

type Value struct {
	typ   ValueType
	bulk  string
	err   string
	str   string
	array []Value
}

func (v *Value) readArray(reader io.Reader) {
	buf := make([]byte, 4)
	reader.Read(buf)

	arrLen, err := strconv.Atoi(string(buf[1]))
	if err != nil {
		fmt.Println(err)
		return
	}

	for range arrLen {
		bulk := v.readBulk(reader)
		v.array = append(v.array, bulk)
	}
}

func (v *Value) readBulk(reader io.Reader) Value {
	buf := make([]byte, 4)
	reader.Read(buf)

	n, err := strconv.Atoi(string(buf[1]))
	if err != nil {
		fmt.Println(err)
		return Value{}
	}

	bulkBuf := make([]byte, n+2)
	reader.Read(bulkBuf)

	bulk := string(bulkBuf[:n])

	return Value{typ: BULK, bulk: bulk}
}

func main() {
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

type Handler func(*Value) *Value // type defn for the map

var Handlers = map[string]Handler{
	"COMMAND": command,
	"GET":     get,
	"SET":     set,
} // map to store the commands and their implementations

var DB = map[string]string{}

func handle(conn net.Conn, v *Value) {
	cmd := v.array[0].bulk       // it's a command like GET, SET, etc
	handler, ok := Handlers[cmd] // handler is the functional implementation of cmd in a map, stores cmd and its functional implementation

	if !ok {
		fmt.Println("invalid command: ", cmd)
		return
	}

	reply := handler(v)  // calling the function of cmd with v as argument
	w := NewWriter(conn) // creating a new writer with conn object
	w.Write(reply)       // converting reply to resp protocol
}

func command(v *Value) *Value {
	return &Value{typ: STRING, str: "OK"}
}

func get(v *Value) *Value {
	args := v.array[1:]
	if len(args) != 1 {
		return &Value{typ: ERROR, err: "ERR invalid number of arguments for 'GET' function"}
	}

	name := args[0].bulk
	val, ok := DB[name]
	if !ok {
		return &Value{typ: NULL}
	}
	return &Value{typ: BULK, bulk: val}
}

func set(v *Value) *Value {
	args := v.array[1:]
	if len(args) != 2 {
		return &Value{typ: ERROR, err: "ERR invalid number of arguments for 'SET' function"}
	}

	key := args[0].bulk
	val := args[1].bulk
	DB[key] = val
	return &Value{typ: STRING, str: "OK"}
}

type Writer struct {
	writer io.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{writer: bufio.NewWriter(w)} // wrapping conn with bufio.Writer
}

func (w *Writer) Write(v *Value) {
	var reply string
	switch v.typ {
	case STRING:
		reply = fmt.Sprintf("%s%s\r\n", v.typ, v.str)
	case BULK:
		reply = fmt.Sprintf("%s%d\r\n%s\r\n", v.typ, len(v.bulk), v.bulk)
	case ERROR:
		reply = fmt.Sprintf("%s%s\r\n", v.typ, v.bulk)
	case NULL:
		reply = "$-1\r\n"
	}

	w.writer.Write([]byte(reply))
	w.writer.(*bufio.Writer).Flush() // flushing the writer
}
