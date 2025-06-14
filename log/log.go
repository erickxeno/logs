package log

import (
	"os"

	"github.com/erickxeno/logs"
)

var (
	// Logger is the default global logger.
	Logger *logs.CLogger
	// V1 is the alias of the Logger.
	V1 *logs.CLogger
	// CompatLogger is the default compatible logger.
	CompatLogger *logs.CompatLogger
)

func init() {
	Logger, V1 = logs.V1, logs.V1
	CompatLogger = logs.NewCompatLoggerFrom(V1)
}

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// SetDefaultLogger resets the default logger with specified options,
// it is not thread-safe and please only call it in program initialization.
// Libraries should not config the default logger and leaves it to the application user,
// otherwise loggers would cover each other.
func SetDefaultLogger(ops ...logs.Option) {
	logs.SetDefaultLogger(ops...)
	Logger, V1 = logs.V1, logs.V1
	CompatLogger = logs.NewCompatLoggerFrom(V1)
}

func Flush() error {
	if err := Logger.Flush(); err != nil {
		return err
	}
	return nil
}

func Close() error {
	if err := Logger.Close(); err != nil {
		return err
	}
	return nil
}

// SecMark add double brackets to the key-value pairs and return the generated string.
// This is a function that directly exported to users. So we need to check the type of the parameters
func SecMark(key, val interface{}) string {
	return logs.SecMark(key, val)
}

func SetSecMark(isEnabled bool) {
	logs.SetSecMark(isEnabled)
}

func IsSecMarkEnabled() bool {
	return logs.IsSecMarkEnabled()
}

func SetCurrentVersion(version string) {
	logs.SetCurrentVersion(version)
}
