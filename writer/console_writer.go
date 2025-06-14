package writer

import (
	"io"
	"os"
)

// ConsoleWriter provides a console output writer.
type ConsoleWriter struct {
	io.WriteCloser
	isColorful bool
}

func NewConsoleWriter(options ...ConsoleOption) LogWriter {
	w := &ConsoleWriter{
		WriteCloser: os.Stdout,
		isColorful:  true,
	}

	for _, op := range options {
		op(w)
	}
	return w
}

func (w *ConsoleWriter) Write(l RecyclableLog) error {
	defer l.Recycle()
	var err error
	content := l.GetContent()

	if !w.isColorful {
		content = append(content, '\n')
		_, err = w.WriteCloser.Write(content)
	} else {
		packet := NewPacket(0)
		defer PutPacket(packet)
		*packet = append(*packet, colors[l.GetLevel()]...)
		*packet = append(*packet, content...)
		*packet = append(*packet, colorSuffix...)
		*packet = append(*packet, '\n')
		_, err = w.WriteCloser.Write(*(packet))
	}
	return err
}

func (w *ConsoleWriter) Close() error {
	return nil
}
func (w *ConsoleWriter) Flush() error {
	return nil
}

var colors = map[string][]byte{
	"Trace":  []byte("\033[1;34m"), // Trace    blue
	"Debug":  []byte("\033[1;34m"), // Debug    blue
	"Info":   []byte("\033[1;36m"), // Info     cyan
	"Notice": []byte("\033[1;32m"), // Notice   green
	"Warn":   []byte("\033[1;33m"), // Warn     yellow
	"Error":  []byte("\033[1;31m"), // Error    red
	"Fatal":  []byte("\033[1;35m"), // Fatal    magenta
	"?":      []byte("\033[1;37m"),
}
var colorSuffix = []byte("\033[0m")

type ConsoleOption func(writer *ConsoleWriter)

func SetColorful(isColorful bool) ConsoleOption {
	return func(writer *ConsoleWriter) {
		writer.isColorful = isColorful
	}
}
