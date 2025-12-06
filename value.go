package main

import (
	"fmt"
	"io"
	"log"
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

func (v *Value) readArray(reader io.Reader) error {
	buf := make([]byte, 4)
	_, err := reader.Read(buf)
	if err != nil {
		return err
	}

	arrLen, err := strconv.Atoi(string(buf[1]))
	if err != nil {
		return err
	}

	for range arrLen {
		bulk, err := v.readBulk(reader)
		if err != nil {
			log.Println(err)
			break
		}
		v.array = append(v.array, bulk)
	}

	return nil
}

func (v *Value) readBulk(reader io.Reader) (Value, error) {
	buf := make([]byte, 4)
	reader.Read(buf)

	n, err := strconv.Atoi(string(buf[1]))
	if err != nil {
		fmt.Println(err)
		return Value{}, err
	}

	bulkBuf := make([]byte, n+2)
	reader.Read(bulkBuf)

	bulk := string(bulkBuf[:n])

	return Value{typ: BULK, bulk: bulk}, nil
}
