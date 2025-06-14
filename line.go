package logs

import (
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
)

// Line creates a cached source code file line information by sync.Once
// to avoid reflect call of each log printing.
type Line struct {
	sync.Once
	literal []byte
}

func (l *Line) load(callDepth int, fullPath bool) []byte {
	l.Do(func() {
		if len(l.literal) != 0 {
			return
		}
		_, file, line, ok := runtime.Caller(callDepth + preparedCallDepthOffset)
		if !fullPath {
			file = filepath.Base(file)
		}
		if ok {
			l.literal = append(l.literal, file+":"+strconv.Itoa(line)...)
		}
	})
	return l.literal
}

// If user does not want to define log's Line struct for every log, can use LineWithOnlyFilename to generate a global
// Line var in its file, and use this in all log.XXX().Line() calls within this file
func LineWithOnlyFilename() *Line {
	l := &Line{}
	_, file, _, _ := runtime.Caller(1)
	file = filepath.Base(file)
	l.literal = append(l.literal, file...)
	l.Do(func() {})
	return l
}

func CustomLine(value []byte) *Line {
	l := &Line{literal: value}
	l.Do(func() {})
	return l
}
