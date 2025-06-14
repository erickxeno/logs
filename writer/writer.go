package writer

import (
	"context"
	"io"
	osTime "time"
)

// LogWriter provides a handler to output log
// there are four internal writers: ConsoleWriter, FileWriter, AgentWriter and NoopWriter.
type LogWriter interface {
	io.Closer
	Write(log RecyclableLog) error
	Flush() error
}

// RecyclableLog defines the log could be handled by writer,
// every writer should call Recycle method after printing.
type RecyclableLog interface {
	Recycler
	StructuredLog
}

// Recycler recycles the log instance to avoid memory allocation each printing.
type Recycler interface {
	Recycle()
}

// StructuredLog allows to get each part of the log.
type StructuredLog interface {
	GetContent() []byte
	GetBody() []byte
	GetTime() osTime.Time
	GetLine() string
	GetLevel() string
	GetContext() context.Context
	GetLocation() []byte
	GetPSM() string
	GetKVList() []*KeyValue
	GetKVListStr() []string
}

// NoopWriter does nothing in any cases, it is useful in writing tests.
type NoopWriter struct{}

func (w *NoopWriter) Write(l RecyclableLog) error {
	defer l.Recycle()
	return nil
}

func (w *NoopWriter) Close() error { return nil }
func (w *NoopWriter) Flush() error { return nil }
