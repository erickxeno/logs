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
	if atomic.CompareAndSwapInt64(&l.writingCount, 1, 0) {
		recycle(&l.Log)
	} else {
		atomic.AddInt64(&l.writingCount, -1)
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
	switch {
	case cap(l.buf)+cap(l.bodyBuf) <= 1<<16:
		l.strikes = 0
	case cap(l.buf)/2+cap(l.bodyBuf) <= len(l.buf)+len(l.bodyBuf):
		l.strikes = 0
	case l.strikes < 4:
		l.strikes++
	default:
		return
	}
	l.buf = l.buf[:0]
	l.bodyBuf = l.bodyBuf[:0]
	l.executors = l.executors[:0]
	l.ctx = nil
	l.psm = l.psm[:0]
	l.line = nil
	l.time = osTime.Time{}
	l.loc = l.loc[:0]
	l.padding = l.padding[:0]
	for _, kv := range l.kvlist {
		kv.Recycle()
	}
	l.kvlist = l.kvlist[:0]
	l.stackInfo = NoPrint
	l.enableDynamicLevel = false
	logPool.Put(l)
}
