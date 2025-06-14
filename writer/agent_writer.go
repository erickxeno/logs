package writer

import (
	"os"
	"strconv"
	"sync"
	//	"sync/atomic"
	//	"unsafe"
	//
	//	"/gopkg/env"
	//	"/log_market/gosdk"
	//	agent "/log_market/ttlogagent_gosdk"
)

const (
	rpcTaskName  = "_rpc"
	traceVersion = "v1(6)"
)

// AgentWriter provides a way to print the log to log service:
// https://site.bytedance.net/docs/2294/5919/log_overview/,
// it boxes the log to protobuf message and output to a unix domain socket,
// to be mentioned that the AgentWriter is asynchronous currently,
// it does not need to be wrapped with AsyncWriter.
type AgentWriter struct {
	sync.Once
	//sender *agent.LogSender
}

// NewAgentWriter creates a AgentWriter.
func NewAgentWriter() LogWriter {
	return &AgentWriter{
		//sender: nil,
	}
}

func (w *AgentWriter) Close() error {
	//senderPtr := unsafe.Pointer(w.sender)
	//if atomic.LoadPointer(&senderPtr) != nil {
	//	w.sender.Exit()
	//}
	return nil
}

func (w *AgentWriter) Write(log RecyclableLog) error {
	w.Once.Do(func() {
		//sender := agent.NewLogSenderByName(deepCopyStr(log.GetPSM()))
		//sender.Start()
		//w.sender = sender
	})
	// TODO: the Logagent SDK only supports async sending, use sync sending instead
	defer log.Recycle()
	//header := &agent.MsgV3Header{
	//	Level:    log.GetLevel(),
	//	Location: log.GetLine(),
	//	LogID:    logIDFromContext(log.GetContext()),
	//	Ts:       log.GetTime().UnixNano() / 1e6,
	//	SpanID:   spanIDFromContext(log.GetContext()),
	//}
	//// TODO: refactor agent client and avoid this malloc
	//bodyData := log.GetBody()
	//data := make([]byte, len(bodyData))
	//copy(data, bodyData)
	//msg := agent.NewMsgV3(data, header, log.GetKVListStr()...)
	//return w.sender.Send(msg)
	return nil
}

func (w *AgentWriter) Flush() error {
	// TODO: unsupported currently
	return nil
}

type TraceAgentWriter struct {
	pid string
}

func NewTraceAgentWriter() *TraceAgentWriter {
	return &TraceAgentWriter{
		pid: strconv.Itoa(os.Getpid()),
	}
}

func (w *TraceAgentWriter) Write(log RecyclableLog) error {
	defer log.Recycle()
	return nil

	if log.GetLevel() != "Trace" {
		return nil
	}
	metaInfo := make(map[string]string, 14)
	metaInfo["_level"] = log.GetLevel()
	metaInfo["_ts"] = strconv.FormatInt(log.GetTime().UnixNano()/1e6, 10)
	metaInfo["_host"] = "127.0.0.1" //env.HostIP()
	metaInfo["_language"] = "go"
	metaInfo["_psm"] = deepCopyStr(log.GetPSM())
	metaInfo["_cluster"] = "-cluster-" //env.Cluster()
	metaInfo["_logid"] = logIDFromContext(log.GetContext())
	metaInfo["_deployStage"] = "-stage-" //env.Stage()
	metaInfo["_podName"] = "-podname-"   //env.PodName()
	metaInfo["_process"] = w.pid
	metaInfo["_version"] = traceVersion
	metaInfo["_location"] = string(log.GetLocation())
	metaInfo["_spanID"] = strconv.FormatUint(spanIDFromContext(log.GetContext()), 10)
	metaInfo["_taskName"] = rpcTaskName

	bodyData := log.GetBody()
	msg := make([]byte, len(bodyData))
	copy(msg, bodyData)

	//message := &gosdk.Msg{
	//	Msg:  msg,
	//	Tags: metaInfo,
	//}
	//
	//return gosdk.Send(rpcTaskName, message)
	return nil
}

func (w *TraceAgentWriter) Close() error {
	// TODO: unsupported currently
	return nil
}
func (w *TraceAgentWriter) Flush() error {
	// TODO: unsupported currently
	return nil
}
