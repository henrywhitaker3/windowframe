// Package queue
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type Queue string
type Task string

type ShutdownFunc = func(context.Context) error

type HandlerFunc = func(context.Context, []byte) error

type job struct {
	Task      Task      `json:"task"`
	CreatedAt time.Time `json:"created_at"`
	Payload   []byte    `json:"payload"`
}

func newJob(task Task, payload any) (job, error) {
	by, err := Marshal(payload)
	if err != nil {
		return job{}, fmt.Errorf("marshal job payload: %w", err)
	}
	return job{
		Task:      task,
		CreatedAt: time.Now().UTC(),
		Payload:   by,
	}, nil
}

func Marshal(d any) ([]byte, error) {
	return json.Marshal(d)
}

func Unmarshal(d []byte, out any) error {
	return json.Unmarshal(d, out)
}
