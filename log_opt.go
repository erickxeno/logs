package logs

import "context"

const (
	normalKV int32 = iota
	kvInMsg
	normalObj int32 = iota
	convertObjToKV
	normalErr int32 = iota
	convertErrToKV
)

type logConf struct {
	kvStatus  int32
	objStatus int32
	errStatus int32
}
type kvOption func(conf *logConf)

type objOption func(conf *logConf)

type errOption func(conf *logConf)

// use this function, otherwise there will be a mem allocotion.
func genKvConf(ops []kvOption) logConf {
	var conf logConf
	for _, v := range ops {
		v(&conf)
	}
	return conf
}

func genObjConf(ops []objOption) logConf {
	var conf logConf
	for _, v := range ops {
		v(&conf)
	}
	return conf
}

func genErrConf(ops []errOption) logConf {
	var conf logConf
	for _, v := range ops {
		v(&conf)
	}
	return conf
}

type loggerConf struct {
	ctx context.Context
}

type loggerOption func(conf *loggerConf)

func genLoggerConf(ops []loggerOption) loggerConf {
	var conf loggerConf
	for _, v := range ops {
		v(&conf)
	}
	return conf
}
