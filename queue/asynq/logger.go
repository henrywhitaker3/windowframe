package asynq

import (
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
)

type asynqLogger struct {
	log *slog.Logger
}

func (a *asynqLogger) Info(args ...any) {
	if len(args) == 0 || a.log == nil {
		return
	}
	a.log.Info(fmt.Sprintf("%v", args[0]), args[1:]...)
}

func (a *asynqLogger) Debug(args ...any) {
	if len(args) == 0 || a.log == nil {
		return
	}
	a.log.Debug(fmt.Sprintf("%v", args[0]), args[1:]...)
}

func (a *asynqLogger) Error(args ...any) {
	if len(args) == 0 || a.log == nil {
		return
	}
	a.log.Error(fmt.Sprintf("%v", args[0]), args[1:]...)
}

func (a *asynqLogger) Fatal(args ...any) {
	if len(args) == 0 || a.log == nil {
		return
	}
	a.log.Error(fmt.Sprintf("%v", args[0]), args[1:]...)
}

func (a *asynqLogger) Warn(args ...any) {
	if len(args) == 0 || a.log == nil {
		return
	}
	a.log.Warn(fmt.Sprintf("%v", args[0]), args[1:]...)
}

var _ asynq.Logger = &asynqLogger{}
