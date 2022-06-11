package personalmessagequeue

import (
	log "github.com/sirupsen/logrus"
	"time"
)

type PersonalMessageQueue struct {
	timeQueue []time.Time
	capacity  int
}

func NewPersonalMessageQueue(capacity int) *PersonalMessageQueue {
	pmq := &PersonalMessageQueue{
		[]time.Time{},
		capacity,
	}
	go pmq.routinelyLogVelocity()
	return pmq
}

func (pmq *PersonalMessageQueue) routinelyLogVelocity() {
	for {
		time.Sleep(10 * time.Second)
		log.Infof("current personal velocity: %f messages/second", pmq.Velocity())
	}
}

func (pmq *PersonalMessageQueue) Push(t time.Time) {
	if len(pmq.timeQueue) == pmq.capacity {
		pmq.pop()
	}
	pmq.timeQueue = append(pmq.timeQueue, t)
}

func (pmq *PersonalMessageQueue) pop() {
	pmq.timeQueue = pmq.timeQueue[1:]
}

func (pmq *PersonalMessageQueue) length() int {
	return len(pmq.timeQueue)
}

func (pmq *PersonalMessageQueue) Velocity() float64 {
	mqSlice := pmq.timeQueue
	if len(mqSlice) == 0 {
		return 0
	}
	count := 0
	start := time.Now().Add(-30 * time.Second)
	for _, t := range pmq.timeQueue {
		if t.After(start) {
			count++
		}
	}
	timeSpanSeconds := 30.0
	return float64(count) / timeSpanSeconds
}
