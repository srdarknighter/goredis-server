package main

import "time"

type Item struct {
	V          string
	exp        time.Time
	LastAccess time.Time
	Accesses   int
}

func (i *Item) shouldExpire() bool {
	return (i.exp.Unix() != UNIX_TS_EPOCH && time.Until(i.exp).Seconds() <= 0)
}

func (k *Item) approxMemUsage(name string) int64 {
	stringHeader := 16
	expHeader := 24
	mapEntrySize := 32

	return int64(stringHeader + len(name) + stringHeader + len(k.V) + expHeader + mapEntrySize)
}
