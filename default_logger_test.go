package logs

import (
	"context"
	"testing"
	"time"
)

func TestSetLevel(t *testing.T) {
	defer Flush()
	SetLevel(InfoLevel)
	Info("test1, level: %d", GetLevel())
	SetLevel(ErrorLevel)
	Info("test2, level: %d", GetLevel())
}

func testCallDepth() {
	Info("test1, level: %d", GetLevel())
}

func TestAddCallDepth(t *testing.T) {
	defer ResetCallDepth()
	testCallDepth()
	AddCallDepth(1)
	testCallDepth()
	AddCallDepth(1)
	testCallDepth()
	ResetCallDepth()
	testCallDepth()
}

func TestSetDefaultLogCallDepthOffset(t *testing.T) {
	defer ResetCallDepth()
	SetDefaultLogCallDepthOffset(1)
	testCallDepth()
	ResetCallDepth()
	testCallDepth()

	AddCallDepth(1)
	testCallDepth()
	ResetCallDepth()
	testCallDepth()
}

func TestSetDefaultLogPrefixFileDepth(t *testing.T) {
	defer SetDefaultLogPrefixFileDepth(0)
	Info("test1")
	SetDefaultLogPrefixFileDepth(1)
	Info("test2")
	SetDefaultLogPrefixFileDepth(2)
	Info("test3")
}

func TestSetLogPrefixTimePrecision(t *testing.T) {
	defer SetLogPrefixTimePrecision(TimePrecisionMillisecond)
	Info("test1")
	SetLogPrefixTimePrecision(TimePrecisionMicrosecond)
	time.Sleep(1 * time.Second)
	Info("test2")
	SetLogPrefixTimePrecision(TimePrecisionSecond)
	time.Sleep(1 * time.Second)
	Info("test3")
	SetLogPrefixTimePrecision(TimePrecisionMillisecond)
	time.Sleep(1 * time.Second)
	Info("test4")
}

func TestSetDefaultLogPrefixWithoutHost(t *testing.T) {
	defer SetDefaultLogPrefixWithoutHost(false)
	Info("test1")
	SetDefaultLogPrefixWithoutHost(true)
	Info("test2")
}

func TestSetDefaultLogPrefixWithoutPSM(t *testing.T) {
	defer SetDefaultLogPrefixWithoutPSM(false)
	Info("test1")
	SetDefaultLogPrefixWithoutPSM(true)
	Info("test2")
}

func TestSetDefaultLogPrefixWithoutCluster(t *testing.T) {
	defer SetDefaultLogPrefixWithoutCluster(false)
	Info("test1")
	SetDefaultLogPrefixWithoutCluster(true)
	Info("test2")
}

func TestSetDefaultLogPrefixWithoutStage(t *testing.T) {
	defer SetDefaultLogPrefixWithoutStage(false)
	Info("test1")
	SetDefaultLogPrefixWithoutStage(true)
	Info("test2")
}

func TestSetDefaultLogPrefixWithoutSpanID(t *testing.T) {
	defer SetDefaultLogPrefixWithoutSpanID(false)
	Info("test1")
	SetDefaultLogPrefixWithoutSpanID(true)
	Info("test2")
}

func TestWithoutMultiOptions(t *testing.T) {
	defer SetDefaultLogPrefixWithoutHost(false)
	defer SetDefaultLogPrefixWithoutPSM(false)
	defer SetDefaultLogPrefixWithoutCluster(false)
	defer SetDefaultLogPrefixWithoutStage(false)
	defer SetDefaultLogPrefixWithoutSpanID(false)

	SetDefaultLogPrefixWithoutHost(true)
	SetDefaultLogPrefixWithoutPSM(true)
	SetDefaultLogPrefixWithoutCluster(true)
	SetDefaultLogPrefixWithoutStage(true)
	SetDefaultLogPrefixWithoutSpanID(true)
	Info("test1")
	ctx := context.Background()
	ctx = context.WithValue(ctx, LogIDCtxKey, "LOG_ID")
	CtxInfo(ctx, "test2")
}
