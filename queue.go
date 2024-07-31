package main

import (
	"container/list"
	"sync"
	"time"
)

type Queue struct {
	list    *list.List
	lock    *sync.RWMutex
	waitNum int32
	flag    chan bool
}

func NewQueue() *Queue {
	return &Queue{list: list.New(), lock: new(sync.RWMutex), flag: make(chan bool)}
}

// Offer 写入
func (queue *Queue) Offer(value interface{}) {
	queue.lock.Lock()
	queue.list.PushFront(value)
	if queue.waitNum > 0 {
		queue.waitNum--
		queue.lock.Unlock()
		queue.flag <- true
	} else {
		queue.lock.Unlock()
	}
}

// DequeueTimer 定时返回
func (queue *Queue) DequeueTimer(timer *time.Timer) (value interface{}, hasValue bool) {
	for {
		queue.lock.Lock()
		ele := queue.list.Back()
		if ele != nil {
			queue.list.Remove(ele)
			queue.lock.Unlock()
			timer.Stop()
			return ele.Value, true
		} else {
			queue.waitNum++
			queue.lock.Unlock()
			select {
			case <-queue.flag:
				{
					continue
				}
			case <-timer.C:
				{
					queue.lock.Lock()
					if queue.waitNum > 0 {
						queue.waitNum--
						queue.lock.Unlock()
					} else {
						queue.lock.Unlock()
					}
					timer.Stop()
					return nil, false
				}
			}
		}
	}
}
