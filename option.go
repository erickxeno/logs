package logs

import (
	"context"

	"github.com/erickxeno/logs/writer"
)

type funcNameInfo int32

const (
	noPrintFunc funcNameInfo = iota
	withPkgName
	funcNameOnly
)

// Option provides some options to config CLogger.
type Option func(*CLogger)

// SetPSM sets psm to CLogger, logger would read env.PSM as default.
func SetPSM(psm string) Option {
	return func(logger *CLogger) {
		logger.psm = psm
	}
}

// SetWriter sets CLogger outputs to which writers.
func SetWriter(level Level, ws ...writer.LogWriter) Option {
	return func(logger *CLogger) {
		logger.minLevel = FatalLevel
		logger.writers = logger.writers[:0]
		for _, w := range ws {
			logger.addWriter(level, w)
		}
	}
}

// SetPadding sets CLogger padding between each element.
func SetPadding(padding string) Option {
	return func(logger *CLogger) {
		logger.padding = []byte(padding)
	}
}

// SetCallDepth sets CLogger call depth while logging file location.
func SetCallDepth(c int) Option {
	return func(logger *CLogger) {
		logger.callDepth = c
	}
}

// SetTracing sets a compatible tracing writer to trace level logs,
// it is only used in compatible cases,
func SetTracing() Option {
	return func(logger *CLogger) {
		logger.addWriter(TraceLevel, writer.NewTraceAgentWriter())
	}
}

// SetMiddleware adds the middleware to the logger
func SetMiddleware(m ...Middleware) Option {
	return func(logger *CLogger) {
		logger.middlewares = append(logger.middlewares, m...)
	}
}

// UpdateMiddleware sets middleware to the logger
func UpdateMiddleware(m ...Middleware) Option {
	return func(logger *CLogger) {
		logger.middlewares = logger.middlewares[:0]
		logger.middlewares = append(logger.middlewares, m...)
	}
}

// SetFullPath sets print the full path of log file location.
func SetFullPath(fullpath bool) Option {
	return func(logger *CLogger) {
		logger.fullPath = fullpath
	}
}

// SetZoneInfo sets if the time string include zone info
// example:
//
//	include=false:  2021-07-15 15:19:14,161
//	include=true:   2021-07-15 15:19:14,161 +0800
func SetZoneInfo(include bool) Option {
	return func(logger *CLogger) {
		logger.includeZoneInfo = include
	}
}

// AppendWriter sets CLogger outputs to which writers.
// It will not remove existing writers.
func AppendWriter(level Level, ws ...writer.LogWriter) Option {
	return func(logger *CLogger) {
		for _, w := range ws {
			logger.addWriter(level, w)
		}
	}
}

// ConfigSecMark sets if the logger needs to add double brackets to key-value pairs
// example:
//
//	isEnabled=false:  count=100
//	isEnabled=true:   {{count=100}}
//
// If you also specify the current version of your app, otherwise it will use
// the image tag by default
func ConfigSecMark(isEnabled bool) Option {
	return func(logger *CLogger) {
		enableSecMark = isEnabled
		if isEnabled {
			logger.fullPath = true // why this?
		}
	}
}

// SetCurrentVersion sets the current version. Call it in program initialization.
func SetCurrentVersion(version string) {
	currVersion = version
}

// Deprecated: KVList is always enabled.
func SetEnableKVList(isEnabled bool) Option {
	return func(logger *CLogger) {}
}

// SetSecMark updates enableSecMark. Call it in program initialization.
func SetSecMark(isEnabled bool) {
	enableSecMark = isEnabled
}

func IsSecMarkEnabled() bool {
	return enableSecMark
}

// SetFatalOSExit sets whether log sdk calls os.Exit after printing fatal log.
func SetFatalOSExit(isEnabled bool) Option {
	return func(logger *CLogger) {
		logger.exitWhenFatal = isEnabled
	}
}

// SetKVPosition sets the positions of the kv list in console and file writers' output.
func SetKVPosition(position KVPosition) Option {
	return func(logger *CLogger) {
		logger.kvPosition = position
	}
}

// SetConvertErrorToKV sets if it converts an error to a KV pair.
func SetConvertErrorToKV(isEnabled bool) Option {
	return func(logger *CLogger) {
		logger.convertErrToKV = isEnabled
	}
}

// SetConvertObjectToKV sets if it converts an object to a KV pair.
func SetConvertObjectToKV(isEnabled bool) Option {
	return func(logger *CLogger) {
		logger.convertObjToKV = isEnabled
	}
}

// SetDeduplicateCtxKVs sets if it converts the kvlist in the ctx to a string.
// if isEnabled is true, it keeps the kvlist and will deduplicate the KVs in StreamLog 2.0
// Else it allows duplicate kvs and convert the kvs to a string and append it to the message body.
func SetDeduplicateCtxKVs(isEnabled bool) Option {
	return func(logger *CLogger) {
		convertCtxKVListToStr = !isEnabled
	}
}

// SetDisplayFuncName sets if the logger will print the function name.
// It will attach a kv: func={function name} to the logs.
// It withPackage is true, function name will look like {pkg name}.{func name}, e.g.,
// func=github.com/facebookgo/ensure.TestFunc
func SetDisplayFuncName(withPackage bool) Option {
	return func(logger *CLogger) {
		if withPackage {
			logger.funcNameInfo = withPkgName
		} else {
			logger.funcNameInfo = funcNameOnly
		}
	}
}

// SetLazyHandleCtx sets if the logger lazily handles the ctx, e.g., extrating kvs from ctx.
func SetLazyHandleCtx() Option {
	return func(logger *CLogger) {
		logger.lazyHandleCtx = true
	}
}

// SetDisplayEnvInfo sets if the logger will add a KV pair: ("_env", {env}) to all logs.
func SetDisplayEnvInfo(displayEnvInfo bool) Option {
	return func(logger *CLogger) {
		logger.addEnv = displayEnvInfo
	}
}

// SetEnableDynamicLevel sets whether to enable dynamic log level in compatLogger.
// It only works for compatible logger.
func SetEnableDynamicLevel(isEnabled bool) Option {
	return func(logger *CLogger) {
		logger.compatLoggerDynamicLevel = isEnabled
	}
}

// AppendKVInMsg directly append kv to the log body.
func AppendKVInMsg() kvOption {
	return func(conf *logConf) {
		conf.kvStatus = kvInMsg
	}
}

// ConvertObjToKV convets an obj to a KV
func ConvertObjToKV() objOption {
	return func(conf *logConf) {
		conf.objStatus = convertObjToKV
	}
}

// ConvertErrToKV converts an error to a KV
func ConvertErrToKV() errOption {
	return func(conf *logConf) {
		conf.errStatus = convertErrToKV
	}
}

// WithDynamicLoggerLevel temporarily sets the logger level for a log.
// It can allow user to temporily print low-level logs.
func WithDynamicLoggerLevel(level Level) loggerOption {
	return func(conf *loggerConf) {
		ctx := context.WithValue(context.Background(), DynamicLogLevelKey, level)
		conf.ctx = ctx
	}
}

// WithCtx attach a context to a specific logger
// It can also temporarily change the level of the logger
func WithCtx(ctx context.Context) loggerOption {
	return func(conf *loggerConf) {
		conf.ctx = ctx
	}
}

// CompatibleLoggerOption provides some options to config CompatLogger
type CompatibleLoggerOption func(*CompatLogger)

// WithCallDepthOffset sets the call depth offset for the compatibale logger.
func WithCallDepthOffset(offset int) CompatibleLoggerOption {
	return func(logger *CompatLogger) {
		logger.callDepthOffset = offset
	}
}

// WithDynamicLevel sets whether the dyancmic level is enabled for the compatibale logger.
func WithDynamicLevel(isEnabled bool) CompatibleLoggerOption {
	return func(logger *CompatLogger) {
		logger.enableDynamicLevel = isEnabled
	}
}

// SetLogPrefixFileDepth sets how many directory levels to include in file path.
// - When logPrefixFileDepth is 0, returns only the filename
// - When logPrefixFileDepth is 1, returns the last directory name and filename
// - When logPrefixFileDepth is n, returns the last n directory names and filename
func SetLogPrefixFileDepth(depth int) Option {
	return func(logger *CLogger) {
		logger.logPrefixFileDepth = depth
	}
}
