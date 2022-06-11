package colorstate

import (
	"fmt"
	twitchirc "github.com/gempir/go-twitch-irc/v2"
	personalmessagequeue "harubot/personal-message-queue"
	"time"
)

const (
	ASCENDING = iota
	DESCENDING
)

type ColorState struct {
	direction  int
	colorIndex int
	colors     []string
}

func NewColorState(colors []string) *ColorState {
	c := &ColorState{
		direction:  ASCENDING,
		colorIndex: 0,
		colors:     colors,
	}
	return c
}

func (c *ColorState) getColor() string {
	return c.colors[c.colorIndex]
}

func (c *ColorState) changeColor() {
	var next int
	var nextDirection = c.direction
	if c.direction == ASCENDING {
		next = c.colorIndex + 1
		if next == len(c.colors)-1 {
			nextDirection = DESCENDING
		}
	} else {
		next = c.colorIndex - 1
		if next == 0 {
			nextDirection = ASCENDING
		}
	}
	c.direction = nextDirection
	c.colorIndex = next
}

func (c *ColorState) RoutinelyChangeColor(client *twitchirc.Client, selfUsername string, pmq *personalmessagequeue.PersonalMessageQueue) {
	for {
		time.Sleep(10 * time.Second)
		c.changeColor()
		client.Say(selfUsername, fmt.Sprintf("/color %s", c.getColor()))
		pmq.Push(time.Now())
	}
}
