package logs

import (
	"context"
	"sync/atomic"
	osTime "time"
	"unsafe"

	"github.com/erickxeno/logs/writer"
)

type logReader struct {
	Log
}

func (l *logReader) Recycle() {
	// Optimized: Use atomic decrement and check, which is faster than CAS in most cases
	if atomic.AddInt64(&l.writingCount, -1) == 0 {
		recycle(&l.Log)
	}
}

func (l *logReader) GetContent() []byte {
	return l.buf
}

func (l *logReader) GetBody() []byte {
	return l.bodyBuf
}

func (l *logReader) SetBody(content []byte) {
	l.bodyBuf = l.bodyBuf[:0]
	l.bodyBuf = append(l.bodyBuf, content...)
}

func (l *logReader) SetKVList(kvlist []*writer.KeyValue) {
	l.kvlist = l.kvlist[:0]
	l.kvlist = append(l.kvlist, kvlist...)
}

func (l *logReader) GetTime() osTime.Time {
	return l.time
}

func (l *logReader) GetLine() string {
	if l.loc != nil && len(l.loc) > 0 {
		return *(*string)(unsafe.Pointer(&l.loc))
	}
	return "?:?"
}

func (l *logReader) GetLevel() string {
	return l.level.String()
}

func (l *logReader) GetContext() context.Context {
	return l.ctx
}

func (l *logReader) GetLocation() []byte {
	return l.loc
}

func (l *logReader) GetPSM() string {
	return *(*string)(unsafe.Pointer(&l.psm))
}
func (l *logReader) GetKVList() []*writer.KeyValue {
	return l.kvlist
}

func (l *logReader) GetKVListStr() []string {
	res := make([]string, len(l.kvlist)*2)
	for i, kv := range l.kvlist {
		res[2*i], res[2*i+1] = kv.ToKV()
	}
	return res
}

func recycle(l *Log) {
	// Optimized recycle logic: simplified capacity check to reduce branch misprediction
	totalCap := cap(l.buf) + cap(l.bodyBuf)
	totalLen := len(l.buf) + len(l.bodyBuf)

	// Fast path: if total capacity is reasonable, always recycle
	if totalCap <= 1<<16 { // 64KB threshold
		l.strikes = 0
	} else if totalCap <= 1<<18 && totalLen >= totalCap/2 { // 256KB threshold with 50% usage
		l.strikes = 0
	} else if l.strikes < 4 {
		l.strikes++
	} else {
		// Don't recycle oversized buffers that are rarely used
		return
	}

	// Reset all fields - optimized order to improve cache locality
	l.buf = l.buf[:0]
	l.bodyBuf = l.bodyBuf[:0]
	l.loc = l.loc[:0]
	l.psm = l.psm[:0]
	l.executors = l.executors[:0]
	l.padding = l.padding[:0]

	// Recycle KV list
	for _, kv := range l.kvlist {
		kv.Recycle()
	}
	l.kvlist = l.kvlist[:0]

	// Reset remaining fields
	l.ctx = nil
	l.line = nil
	l.time = osTime.Time{}
	l.stackInfo = NoPrint
	l.enableDynamicLevel = false

	logPool.Put(l)
}
