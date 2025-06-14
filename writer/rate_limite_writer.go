package writer

// RateLimitWriter provides a wrapper that controls to write frequency to another writer.
// Be careful with use this wrapper. It may reduce the log performance.
type RateLimitWriter struct {
	LogWriter
	limit        int // limit is the max rate this writer can write per second.
	rateLimiters RateLimiters
}

// NewRateLimitWriter creates a RateLimitWriter.
// You need to provide the actual LogWriter, i.e., file, console, agent and the rate limit.
// The limit is the max write frequency in ONE second. If limit is less than 1, it discards all logs.
func NewRateLimitWriter(w LogWriter, limit int) *RateLimitWriter {
	rateLimitWriter := &RateLimitWriter{
		LogWriter:    w,
		limit:        limit,
		rateLimiters: NewRateLimiterMap(),
	}
	return rateLimitWriter
}

func (w *RateLimitWriter) Write(log RecyclableLog) error {
	if w.limit <= 0 {
		// discards all logs
		return nil
	}
	if w.rateLimiters.Allow(string(log.GetLocation()), w.limit) {
		return w.LogWriter.Write(log)
	}
	return nil
}
func (w *RateLimitWriter) Flush() error {
	// TODO not to do anything?
	return nil
}

func (w *RateLimitWriter) Close() error {
	// TODO not to call w.LogWriter.Close()?
	return nil
}
