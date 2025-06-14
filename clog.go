package logs

import (
	"context"
	"unsafe"
)

// CLogger is a common logging handler.
// It is also the default logger.
type CLogger struct {
	logger
	psm     string
	options []Option
}

// NewCLogger creates a new CLogger with options
func NewCLogger(options ...Option) *CLogger {
	logger := &CLogger{
		*NewLogger(),
		"-", // env.PSM()
		options,
	}
	logger.padding = []byte(" ")
	for _, op := range options {
		op(logger)
	}

	// Append the errorlog middleware.
	SetMiddleware(getErrorLogMiddleware(logger.psm))(logger)
	return logger
}

func (l *CLogger) GetOptions() []Option {
	return l.options
}

func (l *CLogger) prefix(log *Log) *Log {
	if log == nil {
		return nil
	}
	return (*prefixedLog)(unsafe.Pointer(log)).Level().Time().Version().Location().Host().PSM(l.psm).LogID().Cluster().Stage().SpanID().End()
}

// Trace starts a trace level log printing.
func (l *CLogger) Trace(ops ...loggerOption) *Log {
	return l.prefix(l.logger.Trace(ops...))
}

// Debug starts a debug level log printing.
func (l *CLogger) Debug(ops ...loggerOption) *Log {
	return l.prefix(l.logger.Debug(ops...))
}

// Notice starts a notice level log printing.
func (l *CLogger) Notice(ops ...loggerOption) *Log {
	return l.prefix(l.logger.Notice(ops...))
}

// Info starts a info level log printing.
func (l *CLogger) Info(ops ...loggerOption) *Log {
	return l.prefix(l.logger.Info(ops...))
}

// Warn starts a warn level log printing.
func (l *CLogger) Warn(ops ...loggerOption) *Log {
	return l.prefix(l.logger.Warn(ops...))
}

// Error starts a error level log printing.
func (l *CLogger) Error(ops ...loggerOption) *Log {
	return l.prefix(l.logger.Error(ops...))
}

// Fatal starts a fatal level log printing.
func (l *CLogger) Fatal(ops ...loggerOption) *Log {
	return l.prefix(l.logger.Fatal(ops...))
}

func (l *CLogger) CtxFlushNotice(ctx context.Context) {
	ntc := GetNotice(ctx)
	if ntc == nil {
		return
	}
	kvs := ntc.KVs()
	if len(kvs) == 0 {
		return
	}
	l.Notice().With(ctx).CallDepth(1).KVs(kvs...).Emit()
}
