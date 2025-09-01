// Package queue
package queue

import (
	"context"
	"encoding/json"
	"time"
)

type Queue string
type Task string

type ShutdownFunc = func(context.Context) error

type HandlerFunc = func(context.Context, []byte) error

type Job struct {
	ID        string    `json:"id"`
	Task      Task      `json:"task"`
	CreatedAt time.Time `json:"created_at"`
	Payload   []byte    `json:"payload"`
	Options   []Option  `json:"-"`
}

func NewJob(id string, task Task, payload []byte, options ...Option) Job {
	return Job{
		ID:        id,
		Task:      task,
		CreatedAt: time.Now().UTC(),
		Payload:   payload,
		Options:   options,
	}
}

func Marshal(d any) ([]byte, error) {
	return json.Marshal(d)
}

func Unmarshal(d []byte, out any) error {
	return json.Unmarshal(d, out)
}
