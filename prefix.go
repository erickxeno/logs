package logs

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unsafe"

	"github.com/erickxeno/time"
)

const (
	funcNameKey = "func"
)

type prefixedLog struct {
	Log
}

func (l *prefixedLog) Time() *prefixedLog {
	if l == nil {
		return nil
	}
	l.executors = append(l.executors, func(l *Log) {
		current := time.Current()
		l.time = current.Time
		if l.includeZone {
			l.buf = append(l.buf, current.ReadOnlyDataWithZone()...)
		} else {
			l.buf = append(l.buf, current.ReadOnlyDataWithoutZone()...)
		}

		l.buf = append(l.buf, ' ')
	})
	return l
}

func (l *prefixedLog) Version() *prefixedLog {
	if l == nil {
		return nil
	}
	// l.executors = append(l.executors, func(l *Log) {
	// 	l.appendStrings(version, " ") // now no need log version
	// })
	return l
}

func (l *prefixedLog) Location() *prefixedLog {
	if l == nil {
		return nil
	}
	l.executors = append(l.executors, func(l *Log) {
		var fileLine string
		if len(l.loc) > 0 {
			fileLine = *(*string)(unsafe.Pointer(&l.loc))
		} else if l.line != nil {
			f := l.line.load(l.logger.callDepth+l.callDepthOffset, l.logger.fullPath || enableSecMark)
			fileLine = *(*string)(unsafe.Pointer(&f))
		} else {
			var pc uintptr
			var file string
			var line int
			var ok bool
			pc, file, line, ok = runtime.Caller(l.logger.callDepth + l.callDepthOffset)

			if ok {
				if !(l.logger.fullPath || enableSecMark) {
					if l.logger.logPrefixFileDepth > 0 {
						parts := strings.Split(file, string(filepath.Separator))
						base := parts[len(parts)-1]
						if len(parts) > 1 {
							start := len(parts) - 1 - l.logger.logPrefixFileDepth
							if start < 0 {
								start = 0
							}
							dirs := parts[start : len(parts)-1]
							file = filepath.Join(append(dirs, base)...)
						} else {
							file = base
						}
					} else {
						file = filepath.Base(file)
					}
				}
				fileLine = file + ":" + strconv.Itoa(line)
				printStatus := l.logger.funcNameInfo
				if printStatus != noPrintFunc {
					fn := runtime.FuncForPC(pc)
					funcName := fn.Name()
					switch printStatus {
					case funcNameOnly:
						funcName = filepath.Ext(funcName)
						funcName = strings.TrimPrefix(funcName, ".")
					}
					l.StrKV(funcNameKey, funcName)
				}
			} else {
				fileLine = "?:?"
			}
		}
		l.appendStrings(fileLine, " ")
		l.loc = l.buf[len(l.buf)-len(fileLine)-1 : len(l.buf)-1]
	})
	return l
}

func (l *prefixedLog) Level() *prefixedLog {
	if l == nil {
		return nil
	}
	l.executors = append(l.executors, func(l *Log) {
		l.appendStrings(l.level.String(), " ")
	})
	return l
}

func (l *prefixedLog) Host() *prefixedLog {
	if l == nil {
		return nil
	}
	if l.logger.logPrefixWithoutHost {
		return l
	}
	l.executors = append(l.executors, func(l *Log) {
		// TODO
		l.appendStrings("127.0.0.1", " ")
	})
	return l
}

func (l *prefixedLog) PSM(psm string) *prefixedLog {
	if l == nil {
		return nil
	}
	if l.logger.logPrefixWithoutPSM {
		return l
	}
	l.psm = append(l.psm, psm...)
	l.executors = append(l.executors, func(l *Log) {
		l.buf = append(l.buf, l.psm...)
		l.buf = append(l.buf, ' ')
	})
	return l
}

func (l *prefixedLog) LogID() *prefixedLog {
	if l == nil {
		return nil
	}
	l.executors = append(l.executors, func(l *Log) {
		l.appendStrings(logIDFromContext(l.ctx), " ")
	})
	return l
}

func (l *prefixedLog) SpanID() *prefixedLog {
	if l == nil {
		return nil
	}
	if l.logger.logPrefixWithoutSpanID {
		return l
	}
	l.executors = append(l.executors, func(l *Log) {
		l.buf = strconv.AppendUint(l.buf, spanIDFromContext(l.ctx), 10)
		l.buf = append(l.buf, ' ')
	})
	return l
}

func (l *prefixedLog) Cluster() *prefixedLog {
	if l == nil {
		return nil
	}
	if l.logger.logPrefixWithoutCluster {
		return l
	}
	l.executors = append(l.executors, func(l *Log) {
		// TODO
		l.appendStrings("-cluster-", " ")
	})
	return l
}

func (l *prefixedLog) Stage() *prefixedLog {
	if l == nil {
		return nil
	}
	if l.logger.logPrefixWithoutStage {
		return l
	}
	l.executors = append(l.executors, func(l *Log) {
		// TODO
		l.appendStrings("-stage-", " ")
	})
	return l
}

func (l *prefixedLog) End() *Log {
	if l == nil {
		return nil
	}
	return (*Log)(unsafe.Pointer(l))
}
