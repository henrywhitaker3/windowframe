package log

type NullLogger struct{}

func (n NullLogger) Info(msg string, args ...any)  {}
func (n NullLogger) Debug(msg string, args ...any) {}
func (n NullLogger) Error(msg string, args ...any) {}

var _ Logger = NullLogger{}
