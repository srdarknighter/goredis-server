package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
)

type ValueType string

const (
	ARRAY   ValueType = "*"
	BULK    ValueType = "$"
	STRING  ValueType = "+"
	INTEGER ValueType = ":"
	ERROR   ValueType = "-"
	NULL    ValueType = ""
)

type Value struct {
	typ   ValueType
	bulk  string
	err   string
	str   string
	num   int
	array []Value
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.Trim(line, "\r\n"), nil
}

// changing this up since we are only reading one key in aof file sync
// since we are reading one array at a time and instantiating a new bufio reader for every array
// we loose continuity in the file

func (v *Value) readArray(r *bufio.Reader) error {
	line, err := readLine(r)
	if err != nil {
		return err
	}

	if line[0] != '*' {
		return errors.New("expected array")
	}

	arrLen, err := strconv.Atoi(string(line[1:]))
	if err != nil {
		return err
	}

	for range arrLen {
		bulk, err := v.readBulk(r)
		if err != nil {
			log.Println(err)
			break
		}
		v.array = append(v.array, bulk)
	}

	return nil
}

func (v *Value) readBulk(r *bufio.Reader) (Value, error) {
	line, err := readLine(r)
	if err != nil {
		log.Println("error in reading bulk", err)
		return Value{}, err
	}

	n, err := strconv.Atoi(string(line[1:]))
	if err != nil {
		fmt.Println(err)
		return Value{}, err
	}

	buf := make([]byte, n+2)
	if _, err := io.ReadFull(r, buf); err != nil {
		return Value{}, nil
	}

	bulk := string(buf[:n])

	return Value{typ: BULK, bulk: bulk}, nil
}
