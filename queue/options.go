package queue

import (
	"fmt"
	"time"
)

type OptionType int

const (
	OnQueueOpt OptionType = iota
	AtOpt
	AfterOpt
	IDOpt
)

type Option interface {
	Type() OptionType
	String() string
	Value() any
}

type (
	atOption    time.Time
	afterOption time.Duration
	iDOption    string
)

func (o Queue) Type() OptionType {
	return OnQueueOpt
}
func (o Queue) String() string {
	return fmt.Sprintf("OnQueue(%s)", string(o))
}
func (o Queue) Value() any {
	return Queue(o)
}

func OnQueue(q Queue) Option {
	return q
}

func (a atOption) Type() OptionType {
	return AtOpt
}
func (a atOption) String() string {
	return fmt.Sprintf("At(%s)", time.Time(a))
}
func (a atOption) Value() any {
	return time.Time(a)
}

func At(t time.Time) Option {
	return atOption(t)
}

func (a afterOption) Type() OptionType {
	return AfterOpt
}
func (a afterOption) String() string {
	return fmt.Sprintf("After(%s)", time.Duration(a).String())
}
func (a afterOption) Value() any {
	return time.Duration(a)
}

func After(d time.Duration) Option {
	return afterOption(d)
}

func (i iDOption) Type() OptionType {
	return IDOpt
}
func (i iDOption) String() string {
	return fmt.Sprintf("ID(%s)", string(i))
}
func (i iDOption) Value() any {
	return string(i)
}

func WithID(id string) Option {
	return iDOption(id)
}

func QueueFromOptions(opts []Option) Queue {
	out := Queue("default")
	for _, opt := range opts {
		if opt.Type() == OnQueueOpt {
			out = opt.Value().(Queue)
			break
		}
	}
	return out
}
