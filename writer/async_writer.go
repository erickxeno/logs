package writer

import (
	"fmt"
	"os"
	"sync"
	osTime "time"

	"golang.org/x/time/rate"
)

const (
	closeTimeout           = osTime.Second
	defaultAsyncBufferSize = 4096 // Increased from 1024 to 4096 for better high-throughput scenarios
)

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

// NewAsyncWriter creates a AsyncWriter with default buffer size (4096).
// For high-throughput scenarios, consider using NewAsyncWriterWithChanLen with a larger buffer.
// omit allows AsyncWriter to drop logs if the buffer is full (non-blocking mode).
// If omit is false, Write() will block when the buffer is full.
func NewAsyncWriter(w LogWriter, omit bool) LogWriter {
	return NewAsyncWriterWithChanLen(w, defaultAsyncBufferSize, omit)
}

// NewAsyncWriterWithChanLen creates a AsyncWriter with custom buffer size.
// chanSize is the buffer size of the internal channel. Choose based on your throughput:
//   - Low throughput (< 1000 logs/s): 1024
//   - Medium throughput (1000-10000 logs/s): 4096 (default)
//   - High throughput (> 10000 logs/s): 8192 or higher
// omit allows AsyncWriter to drop logs if the buffer is full (non-blocking mode).
func NewAsyncWriterWithChanLen(w LogWriter, chanSize int, omit bool) LogWriter {
	if chanSize <= 0 {
		chanSize = defaultAsyncBufferSize
	}
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
