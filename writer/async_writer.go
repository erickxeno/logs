package writer

import (
	"fmt"
	"os"
	"sync"
	osTime "time"

	"golang.org/x/time/rate"
)

const closeTimeout = osTime.Second

// AsyncWriter provides a asynchronous wrapper to another writer,
// it is useful to wrap another blocking writer like FileWriter,
// to the side chain and avoid some overheads in user thread.
type AsyncWriter struct {
	LogWriter
	done       *sync.WaitGroup
	ch         chan RecyclableLog
	flush      chan bool
	flushed    chan error
	omit       bool
	errorPrint *rate.Limiter
}

// NewAsyncWriter creates a AsyncWriter,
// omit allows AsyncWriter omits the log if the buffer is full or not.
func NewAsyncWriter(w LogWriter, omit bool) LogWriter {
	asyncWriter := &AsyncWriter{
		LogWriter:  w,
		done:       &sync.WaitGroup{},
		ch:         make(chan RecyclableLog, 1024),
		flush:      make(chan bool),
		flushed:    make(chan error),
		omit:       omit,
		errorPrint: rate.NewLimiter(rate.Every(osTime.Second), 1),
	}
	go asyncWriter.runWorker()
	return asyncWriter
}

// NewAsyncWriterWithChanLen creates a AsyncWriter,
// chanLen is the length of ch,
// omit allows AsyncWriter omits the log if the buffer is full or not.
func NewAsyncWriterWithChanLen(w LogWriter, chanSize int, omit bool) LogWriter {
	asyncWriter := &AsyncWriter{
		LogWriter:  w,
		done:       &sync.WaitGroup{},
		ch:         make(chan RecyclableLog, chanSize),
		flush:      make(chan bool),
		flushed:    make(chan error),
		omit:       omit,
		errorPrint: rate.NewLimiter(rate.Every(osTime.Second), 1),
	}
	go asyncWriter.runWorker()
	return asyncWriter
}

func (w *AsyncWriter) runWorker() {
	for {
		select {
		case log, ok := <-w.ch:
			if !ok {
				// the buf channel is closed
				return
			}
			err := w.LogWriter.Write(log)
			if err != nil && w.errorPrint.Allow() {
				_, _ = fmt.Fprintf(os.Stderr, "log async writes error: %s\n", err)
			}
			w.done.Done()
		case <-w.flush:
			for i := 0; i < len(w.ch); i++ {
				log := <-w.ch
				err := w.LogWriter.Write(log)
				if err != nil && w.errorPrint.Allow() {
					_, _ = fmt.Fprintf(os.Stderr, "log async writes error: %s\n", err)
				}
				w.done.Done()
			}
			w.flushed <- w.LogWriter.Flush()
		}
	}
}

func (w *AsyncWriter) Write(log RecyclableLog) error {
	w.done.Add(1)
	if w.omit {
		select {
		case w.ch <- log:
		default:
			w.done.Done()
			log.Recycle()
		}
	} else {
		w.ch <- log
	}
	return nil
}

func (w *AsyncWriter) Flush() error {
	w.flush <- true
	return <-w.flushed
}

func (w *AsyncWriter) Close() error {
	// Not close the w.ch to avoid panic.
	// Just try best to send all the data in the channel.
	// If it cannot finish the job in 1 second, exit.
	waitChan := make(chan bool, 1)
	go func() {
		w.done.Wait()
		waitChan <- true
	}()
	select {
	case <-waitChan:
	case <-osTime.After(closeTimeout):
	}
	return w.LogWriter.Close()
}
