package writer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	osTime "time"
)

// RotationWindow allows to claim which rotation window provider uses.
type RotationWindow int8

const (
	// Daily means rotate daily
	Daily RotationWindow = iota
	// Hourly means rotate hourly
	Hourly
)

// FileWriter provides a file rotated output to loggers,
// it is thread-safe and uses memory buffer to boost file writing performance.
type FileWriter struct {
	file           *rotatedFile
	filename       string
	rotationWindow RotationWindow
	fileCountLimit int

	currentTimeSeg osTime.Time
	// Store the next rotation time as Unix timestamp (int64) for atomic access
	// This allows us to check if rotation is needed without locking
	nextRotationTime int64 // atomic
	sync.RWMutex
}

func NewFileWriter(filename string, window RotationWindow, options ...FileOption) LogWriter {
	w := &FileWriter{
		filename:       filename,
		rotationWindow: window,
	}
	file, err := w.loadFile()
	if err != nil {
		panic(err)
	}
	w.file = newRotatedFile(file)
	// Initialize nextRotationTime based on rotation window
	w.updateNextRotationTime(w.currentTimeSeg)
	for _, op := range options {
		op(w)
	}
	return w
}

func (w *FileWriter) loadFile() (io.WriteCloser, error) {
	timedName, currentTimeSeg, err := timedFilename(w.filename)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(filepath.Dir(timedName), os.ModeDir|os.ModePerm)
	if err != nil {
		return nil, err
	}
	var file *os.File
	if env := os.Getenv("IS_PROD_RUNTIME"); len(env) == 0 {
		file, err = os.OpenFile(timedName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	} else {
		file, err = os.OpenFile(timedName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	}
	if err != nil {
		return nil, err
	}
	if _, err := os.Lstat(w.filename); err == nil {
		_ = os.Remove(w.filename)
	}
	_ = os.Symlink(filepath.Base(timedName), w.filename)
	w.currentTimeSeg = currentTimeSeg
	return file, nil
}

func (w *FileWriter) checkIfNeedRotate(logTime osTime.Time) error {
	var needRotate bool

	switch w.rotationWindow {
	case Daily:
		if w.currentTimeSeg.YearDay() != logTime.YearDay() {
			needRotate = true
		}
	case Hourly:
		if w.currentTimeSeg.Hour() != logTime.Hour() || w.currentTimeSeg.YearDay() != logTime.YearDay() {
			needRotate = true
		}
	}

	if needRotate {
		defer func() {
			go w.cleanFiles(w.fileCountLimit)
		}()
		if err := w.rotate(); err != nil {
			return err
		}
		// Update next rotation time after successful rotation
		w.updateNextRotationTime(w.currentTimeSeg)
	}
	return nil
}

func (w *FileWriter) cleanFiles(limit int) {
	if limit <= 0 {
		return
	}
	logs := make([]string, 0)
	_ = filepath.Walk(filepath.Dir(w.filename), func(path string, info os.FileInfo, err error) error {
		if strings.HasPrefix(path, w.filename+".") {
			logs = append(logs, path)
		}
		return nil
	})

	if len(logs) <= limit {
		return
	}
	sort.Slice(logs, func(i, j int) bool {
		return getFileDate(logs[i]).After(getFileDate(logs[j]))
	})
	for _, f := range logs[limit:] {
		_ = os.Remove(f)
	}
}

func (w *FileWriter) rotate() error {
	file, err := w.loadFile()
	if err != nil {
		return err
	}
	w.file.Rotate(file)
	return nil
}

// updateNextRotationTime calculates and stores the next rotation time atomically
func (w *FileWriter) updateNextRotationTime(currentTime osTime.Time) {
	var nextTime osTime.Time
	switch w.rotationWindow {
	case Daily:
		// Next rotation at midnight
		nextTime = currentTime.Truncate(24 * osTime.Hour).Add(24 * osTime.Hour)
	case Hourly:
		// Next rotation at next hour
		nextTime = currentTime.Truncate(osTime.Hour).Add(osTime.Hour)
	}
	// Store as Unix timestamp for atomic access
	atomic.StoreInt64(&w.nextRotationTime, nextTime.Unix())
}

// needsRotation checks if rotation is needed using atomic operation (lock-free fast path)
func (w *FileWriter) needsRotation(logTime osTime.Time) bool {
	nextRotation := atomic.LoadInt64(&w.nextRotationTime)
	return logTime.Unix() >= nextRotation
}

func (w *FileWriter) Write(log RecyclableLog) error {
	defer log.Recycle()

	// Fast path: check if rotation is needed using atomic operation (no lock)
	logTime := log.GetTime()
	if w.needsRotation(logTime) {
		// Slow path: acquire lock only when rotation is actually needed
		w.Lock()
		// Double-check after acquiring lock (another goroutine might have rotated)
		if w.needsRotation(logTime) {
			err := w.checkIfNeedRotate(logTime)
			if err != nil {
				w.Unlock()
				_, _ = fmt.Fprintf(os.Stderr, "write file %s error: %s\n", w.filename, err)
			} else {
				w.Unlock()
			}
		} else {
			w.Unlock()
		}
	}

	_, err := w.file.Write(log.GetContent())
	return err
}

func (w *FileWriter) Close() error {
	return w.file.Close()
}

func (w *FileWriter) Flush() error {
	return w.file.Flush()
}

func timedFilename(filename string) (string, osTime.Time, error) {
	var now osTime.Time
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return "", now, err
	}
	now = osTime.Now()
	return absPath + "." + now.Format(LogFileSuffixDateFormat), now, nil
}

type FileOption func(writer *FileWriter)

func SetKeepFiles(n int) FileOption {
	return func(writer *FileWriter) {
		writer.fileCountLimit = n
	}
}
