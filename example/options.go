package example

import (
	"github.com/erickxeno/logs"
	"github.com/erickxeno/logs/log"
	"github.com/erickxeno/logs/writer"
)

var logger *logs.CLogger

func init() {
	// logger sets 3 writers: console, agent and an async file writer
	// and wrap FileWriter with an asynchronous writer,
	// it also allows FileWriter to omit logs in async processing while blocked in the asynchronous sending.
	options := []logs.Option{
		logs.SetWriter(
			logs.DebugLevel,
			writer.NewConsoleWriter(),
			writer.NewAgentWriter(),
			writer.NewAsyncWriter(
				// set file writer only keeps 12 files and removes other older files.
				writer.NewFileWriter("test/test.log", writer.Hourly, writer.SetKeepFiles(12)),
				true,
			),
		),
		// adjust call depth in file location showing
		logs.SetCallDepth(2),
	}
	// create a logger from above options
	logger = logs.NewCLogger(options...)
	// use it
	logger.Info().Str("hello, world").Emit()

	// it also can reset the default logger
	// it is dangerous to reset the default logger
	// please let this be configured by RPC framework
	// or you know what you do clearly
	log.SetDefaultLogger(options...)
}
