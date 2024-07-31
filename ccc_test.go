package main

import (
	"testing"
	"time"
)

func TestName(t *testing.T) {
	var a = make(chan string, 10_000_000)
	t.Log(a)
	time.Sleep(time.Second * 10)
}
