package queue

import "errors"

var (
	// Stops retrying the task, doesn't archive
	ErrSkipRetry = errors.New("skip retry")
	// Stops retrying the task, saves it in an archive
	ErrDeadLetter = errors.New("deadletter")
)
