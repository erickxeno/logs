package logs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"unsafe"
)

/*
This function directly modifies the buf and trim the padding or whitespace.
	Note that the padding must be valid because it doesn't check it here.
*/
func trimPadding(buf *[]byte, padding []byte, trimSuffixWhitespace ...bool) {
	lenBuf, lenPadding := len(*buf), len(padding)
	if lenBuf == 0 || lenBuf < lenPadding {
		*buf = (*buf)[:0]
		return
	}
	if (*buf)[lenBuf-1] != ' ' && lenPadding > 0 && bytes.Compare((*buf)[lenBuf-lenPadding:lenBuf], padding) != 0 {
		return
	}
	newLen := lenBuf - lenPadding
	*buf = (*buf)[:newLen]
	if len(trimSuffixWhitespace) > 0 && trimSuffixWhitespace[0] {
		var i int
		for i = newLen - 1; i >= 0; i-- {
			if (*buf)[i] != ' ' {
				break
			}
		}
		*buf = (*buf)[:i+1]
	}
}

// SecMark adds double brackets to the key-value pairs and return the generated string.
// This is a function that directly exported to users. So we need to check the type of the parameters
func SecMark(key, val interface{}) string {
	keyStr := ValueToStr(key)
	valStr := ValueToStr(val)
	if enableSecMark {
		return "{{" + keyStr + "=" + valStr + "}}"
	}
	return keyStr + "=" + valStr
}

func ValueToStr(o interface{}) string {
	switch v := o.(type) {
	case nil:
		return ""
	case fmt.Stringer:
		value := reflect.ValueOf(o)
		if !value.IsValid() || value.Kind() == reflect.Ptr && value.IsNil() {
			return fmt.Sprintf("%#v", o)
		}
		return v.String()
	case error:
		value := reflect.ValueOf(o)
		if !value.IsValid() || value.Kind() == reflect.Ptr && value.IsNil() {
			return fmt.Sprintf("%#v", o)
		}
		return v.Error()
	case int:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		return v
	default:
		byteBuf := make([]byte, 0, 128)
		buf := (*jsonBuf)(unsafe.Pointer(&byteBuf))
		e := json.NewEncoder(buf)
		e.SetEscapeHTML(false)
		err := e.Encode(v)
		if err != nil {
			return fmt.Sprintf("%#v", o)
		}
		return string(byteBuf[:len(byteBuf)-1])
	}
}

type Lazier func() interface{}

// Lazy is a helper function to avoid
func Lazy(f func() interface{}) Lazier {
	return f
}

func getHostEnv() string {
	if hostEnv := os.Getenv("TCE_HOST_ENV"); hostEnv != "" {
		return hostEnv
	}
	return "-"
}

type RegionType int32

const (
	REGION_NORMAL RegionType = iota
	REGION_1
	REGION_2
)

func DetermineRegion() RegionType {
	return REGION_NORMAL
}
