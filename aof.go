package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path"
)

type Aof struct {
	w    *Writer
	f    *os.File
	conf *Config
}

func NewAof(conf *Config) *Aof {
	// function to initialize aof rules
	aof := Aof{conf: conf}

	fp := path.Join(aof.conf.dir, aof.conf.aofFn)

	f, err := os.OpenFile(fp, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		fmt.Println("cannot open: ", fp)
		return &aof
	}

	aof.w = NewWriter(f)
	aof.f = f

	return &aof
}

func (aof *Aof) Sync(maxmem int64, evictionpolicy Eviction, memsamples int) {
	r := bufio.NewReader(aof.f)
	for {
		v := Value{}
		err := v.readArray(r)
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Println("unexpected error while reading AOF records: ", err)
			break
		}

		blankState := NewAppState(&Config{
			maxmem:        maxmem,
			eviction:      evictionpolicy,
			maxmemSamples: memsamples,
		})
		blankClient := Client{}
		set(&blankClient, &v, blankState)
	}
}

// write all set commands to file
func (aof *Aof) Rewrite(cp map[string]*Item) {
	// reroute future AOF records to buffer temporarily
	var buf bytes.Buffer
	aof.w = NewWriter(&buf)

	// clear the file contents
	if err := aof.f.Truncate(0); err != nil {
		log.Println("aof rewrite trunaction failed: ", err)
		return
	}

	if _, err := aof.f.Seek(0, 0); err != nil {
		log.Println("aof rewrite seek failed: ", err)
		return
	}

	fwriter := NewWriter(aof.f)

	for k, v := range cp {
		cmd := Value{typ: BULK, bulk: "SET"}
		key := Value{typ: BULK, bulk: k}
		val := Value{typ: BULK, bulk: v.V}

		arr := Value{typ: ARRAY, array: []Value{cmd, key, val}}
		fwriter.Write(&arr)
	}
	fwriter.Flush()

	if _, err := buf.WriteTo(aof.f); err != nil {
		log.Println("aof - cannot write to file: ", err)
	}
	// reroute AOF future records to file

	aof.w = NewWriter(aof.f)
}
