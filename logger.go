package logs

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/erickxeno/logs/writer"
)

type leveledWriter struct {
	writer.LogWriter
	MinLevel Level
}

type logger struct {
	writers                  []leveledWriter
	middlewares              []Middleware
	padding                  []byte
	closed                   int32
	callDepth                int
	minLevel                 Level
	fullPath                 bool
	includeZoneInfo          bool
	exitWhenFatal            bool // This flag indicates whether it calls os.Exit(1) when it prints fatal logs
	kvPosition               KVPosition
	convertErrToKV           bool
	convertObjToKV           bool
	funcNameInfo             funcNameInfo // This field indicates whether and how to print the function name.
	lazyHandleCtx            bool
	addEnv                   bool
	compatLoggerDynamicLevel bool
	logPrefixFileDepth       int // Controls how many directory levels to include in file path
	logPrefixWithoutHost     bool
	logPrefixWithoutPSM      bool
	logPrefixWithoutCluster  bool
	logPrefixWithoutStage    bool
	logPrefixWithoutSpanID   bool

	rateLimiters  writer.RateLimiters
	countLimiters writer.RateLimiters
}

func NewLogger() *logger {
	return &logger{
		middlewares:        make([]Middleware, 0),
		callDepth:          defaultCallDepth,
		minLevel:           FatalLevel,
		logPrefixFileDepth: 0,
		rateLimiters:       writer.NewRateLimiterMap(),
		countLimiters:      writer.NewCountLimiterMap(),
	}
}

func (l *logger) addWriter(level Level, w writer.LogWriter) {
	if level < l.minLevel {
		l.minLevel = level
	}
	l.writers = append(l.writers, leveledWriter{LogWriter: w, MinLevel: level})
}

func (l *logger) newLog(level Level, ops ...loggerOption) *Log {
	// If the level is less than the minLevel of the writers
	// We don't need to process this log, it won't output anything
	if atomic.LoadInt32(&l.closed) != 0 {
		return nil
	}

	conf := loggerConf{}
	if len(ops) > 0 {
		conf = genLoggerConf(ops)
	}

	minLevel, getDynamicLevel := l.CtxLevel(conf.ctx)
	if level < minLevel {
		return nil
	}

	lg := newLog(level, l)
	if conf.ctx != nil {
		lg.ctx = conf.ctx
		lg.enableDynamicLevel = getDynamicLevel
		if !l.lazyHandleCtx {
			lg.handleCtx()
		}
	}
	return lg
}

func (l *logger) CtxLevel(ctx context.Context) (Level, bool) {
	if ctx == nil {
		return l.GetLevel(), false
	}

	val := ctx.Value(DynamicLogLevelKey)

	if val != nil {
		if dynamicLevel, ok := val.(Level); ok {
			return dynamicLevel, true
		}
		if dynamicLevel, ok := val.(int); ok {
			return Level(dynamicLevel), true
		}
	}
	return l.GetLevel(), false
}

// GetLevel gets the minimal level specified in logger writers.
func (l *logger) GetLevel() Level {
	level := atomic.LoadInt32((*int32)(&l.minLevel))
	return Level(level)
}

// Trace gets (or creates) a Log instance from the logPool
// And sets some of its fields: level and logger
func (l *logger) Trace(ops ...loggerOption) *Log {
	return l.newLog(TraceLevel, ops...)
}

func (l *logger) Debug(ops ...loggerOption) *Log {
	return l.newLog(DebugLevel, ops...)
}

func (l *logger) Info(ops ...loggerOption) *Log {
	return l.newLog(InfoLevel, ops...)
}

func (l *logger) Notice(ops ...loggerOption) *Log {
	return l.newLog(NoticeLevel, ops...)
}

func (l *logger) Warn(ops ...loggerOption) *Log {
	return l.newLog(WarnLevel, ops...)
}

func (l *logger) Error(ops ...loggerOption) *Log {
	return l.newLog(ErrorLevel, ops...)
}

func (l *logger) Fatal(ops ...loggerOption) *Log {
	return l.newLog(FatalLevel, ops...)
}

func (l *logger) Flush() error {
	err := make([]error, 0)
	for _, w := range l.writers {
		writerError := w.LogWriter.Flush()
		if writerError != nil {
			err = append(err, writerError)
		}
	}
	if len(err) != 0 {
		return fmt.Errorf("flush error: %#v", err)
	}
	return nil
}

func (l *logger) Close() error {
	if !atomic.CompareAndSwapInt32(&l.closed, 0, 1) {
		return nil
	}

	// store the first close error
	err := make([]error, 0)
	// when err happen, continue close other writer
	for _, w := range l.writers {
		closeError := w.Close()
		if closeError != nil {
			err = append(err, closeError)
		}
	}
	if len(err) != 0 {
		return fmt.Errorf("logger close error: %#v", err)
	}
	return nil
}

// SetLevel sets the minimal level for the logger. It is safe to increase the level.
// Please not decrease the level directly. Use SetLevelForWriters instead.
func (l *logger) SetLevel(newLevel Level) {
	atomic.StoreInt32((*int32)(&l.minLevel), int32(newLevel))
}

// SetLevelForWriters updates the minimal level for the writer. It may also update the loggers' level.
func (l *logger) SetLevelForWriters(newLevel Level, logWriters ...writer.LogWriter) {
	for _, lw := range logWriters {
		for i, _ := range l.writers {
			if l.writers[i].LogWriter == lw {
				atomic.StoreInt32((*int32)(&l.writers[i].MinLevel), int32(newLevel))
				if l.GetLevel() > newLevel {
					l.SetLevel(newLevel)
				}
				break
			}
		}
	}
}

func (l *logger) GetWriter() []leveledWriter {
	return l.writers
}
func (w *leveledWriter) getLevel() Level {
	level := atomic.LoadInt32((*int32)(&w.MinLevel))
	return Level(level)
}
