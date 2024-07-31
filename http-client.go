package main

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

type Request struct {
	client *http.Client
}

func NewRequest() *Request {
	ct := http.Client{Timeout: time.Second * 1, Transport: http.DefaultTransport}
	return &Request{client: &ct}
}
func (r *Request) Call(link string, jsonData []byte) ([]byte, error) {
	var buff = new(bytes.Buffer)
	buff.Write(jsonData)
	resp, err := r.client.Post(link, "application/json", buff)
	if err != nil {
		return nil, err
	}
	all, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return all, nil
}
func (r *Request) Get(link string) ([]byte, error) {
	resp, err := r.client.Get(link)
	if err != nil {
		return nil, err
	}
	all, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return all, nil
}
