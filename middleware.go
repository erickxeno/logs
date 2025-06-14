package logs

import "github.com/erickxeno/logs/writer"

// ContentSetter allows to rewrite the log content.
type ContentSetter interface {
	SetBody([]byte)
	SetKVList([]*writer.KeyValue)
}

// RewritableLog provides some methods to get and set logs.
type RewritableLog interface {
	writer.StructuredLog
	ContentSetter
}

// Middleware function is a hook to get and update logs before writing to each writers.
type Middleware func(log RewritableLog) RewritableLog
