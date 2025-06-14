package writer

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

type (
	Ipv4 uint32
	Ipv6 [16]byte
)

const (
	// 私有协议内部用
	StringType byte = 0 //长度小于255的string
	BoolType   byte = 1
	IntType    byte = 2  //4字节, 8,16,32位有符号,byte,8,16位无符号
	LongType   byte = 3  //8字节, 64位有符号,int
	Uint64Type byte = 4  //8字节, 32,64位无符号
	DoubleType byte = 5  //8字节, float32,float64
	DateType   byte = 20 //暂不支持
	Ipv4Type   byte = 21
	Ipv6Type   byte = 22
	TextType   byte = 23 //长度大于255的string
	BytesType  byte = 24 //二进制字符流
	UUIdType   byte = 25 //128位
)

const (
	StringSplitByte          byte = 0x00
	LENGTH_BYTES                  = 4
	BYTELOG_IPV4_BYTES            = 4
	BYTELOG_IPV6_BYTES            = 16
	UUID_LENGTH                   = 32
	SEQ_ID_LENGTH                 = 16
	BYTELOG_FLAG_BYTES            = 4
	SHORT_STRING_MAX_LEN          = 0xFF
	LONG_STRING_MAX_LEN           = 0xFFFFFFFF
	oneMessageLimitByte           = 128 << 10 // set to 128k due to uds limit
	oneMessageLimitLogNumber      = 4096
	largeMessageLimitByte         = 10 << 20 // 10MB
)

const (
	hextable                   = "0123456789abcdef"
	equalByte                  = '='
	kvRecycleCapacityThreshold = 1024
)

var (
	ErrNoEnoughBytes   = errors.New("no enough bytes")
	emptyStrForByteLog = make([]byte, 3)
	emptyFourBytesData = make([]byte, 4)
)

// KeyValue a key-value pair.
// If the key-value are indexable, the key must be a short string and the value type may be short string, bool, int, or float.
// The value data is stored in bytes.
type KeyValue struct {
	Key       string
	Value     []byte
	ValueType byte  // String, Int, Long, Bool, Double...
	isLong    bool  // Is this a log key-value. Only Short KeyValues can be indexed.
	refCount  int64 // reference count for recycle purpose.
}

type jsonBuf struct {
	buf []byte
}

type VarString struct {
	length  uint8
	content string
}

func (b *jsonBuf) Write(p []byte) (n int, err error) {
	b.buf = append(b.buf, p...)
	return len(p), err
}

// NewKeyValue creates a key-value instance. It also does some necessary checks and encodes the value.
// This KeyValue is short by default: The lengths of the key and value cannot exceed 0xFF.
// You can pass an extra bool argument to mark it as a long KV.
func NewKeyValue(key, value interface{}, isLong ...bool) (*KeyValue, error) {
	isLongKV := false
	if len(isLong) > 0 && isLong[0] {
		isLongKV = true
	}
	kv := keyValuePool.Get().(*KeyValue)
	atomic.StoreInt64(&kv.refCount, 1)
	kv.isLong = isLongKV

	switch v := key.(type) {
	case string:
		kv.Key = v
	case fmt.Stringer:
		vp := reflect.ValueOf(key)
		if !vp.IsValid() || vp.Kind() == reflect.Ptr && vp.IsNil() {
			kv.Key = fmt.Sprintf("%#v", key)
		} else {
			kv.Key = v.String()
		}
	default:
		kv.Recycle()
		return nil, fmt.Errorf("invalid key: %v", key)
	}

	switch v := value.(type) {
	case string:
		kv.Value = append(kv.Value, v...)
		kv.ValueType = StringType
		if len(kv.Value) > math.MaxUint8 {
			kv.ValueType = TextType
		}
	case fmt.Stringer:
		vp := reflect.ValueOf(value)
		if !vp.IsValid() || vp.Kind() == reflect.Ptr && vp.IsNil() {
			kv.Value = append(kv.Value, fmt.Sprintf("%#v", value)...)
		} else {
			kv.Value = append(kv.Value, v.String()...)
		}
		kv.ValueType = StringType
		if len(kv.Value) > math.MaxUint8 {
			kv.ValueType = TextType
		}
	case bool:
		kv.ValueType = BoolType
		if v {
			kv.Value = EncodeUint8(kv.Value, 1)
		} else {
			kv.Value = EncodeUint8(kv.Value, 0)
		}
	case int8:
		kv.ValueType = IntType
		kv.Value = EncodeUint32(kv.Value, uint32(v))
	case int16:
		kv.ValueType = IntType
		kv.Value = EncodeUint32(kv.Value, uint32(v))
	case int32:
		kv.ValueType = IntType
		kv.Value = EncodeUint32(kv.Value, uint32(v))
	case uint8:
		kv.ValueType = IntType
		kv.Value = EncodeUint32(kv.Value, uint32(v))
	case uint16:
		kv.ValueType = IntType
		kv.Value = EncodeUint32(kv.Value, uint32(v))
	case int:
		kv.ValueType = LongType
		kv.Value = EncodeUint64(kv.Value, uint64(v))
	case int64:
		kv.ValueType = LongType
		kv.Value = EncodeUint64(kv.Value, uint64(v))
	case uint32:
		kv.ValueType = Uint64Type
		kv.Value = EncodeUint64(kv.Value, uint64(v))
	case uint:
		kv.ValueType = Uint64Type
		kv.Value = EncodeUint64(kv.Value, uint64(v))
	case uint64:
		kv.ValueType = Uint64Type
		kv.Value = EncodeUint64(kv.Value, v)
	case float32:
		kv.ValueType = DoubleType
		kv.Value = EncodeUint64(kv.Value, math.Float64bits(float64(v)))
	case float64:
		kv.ValueType = DoubleType
		kv.Value = EncodeUint64(kv.Value, math.Float64bits(v))
	case []byte:
		kv.ValueType = BytesType
		kv.Value = append(kv.Value, v...)
	default:
		kv.Recycle()
		return nil, fmt.Errorf("invalid value: %v", value)
	}

	if len(kv.Key) > SHORT_STRING_MAX_LEN {
		kv.Key = kv.Key[:SHORT_STRING_MAX_LEN]
	}

	// Check whether the short kv is too long. If it is too long, then trim the key and value.
	if !kv.isLong {
		if len(kv.Value) > SHORT_STRING_MAX_LEN {
			kv.Value = kv.Value[:SHORT_STRING_MAX_LEN]
			if kv.ValueType == TextType {
				kv.ValueType = StringType
			}
		}
	}

	return kv, nil
}

// NewStrKeyValue creates a key-value instance for a string kv-pair.
// It also does some necessary checks and encodes the value.
// This KeyValue is short by default: The lengths of the key and value cannot exceed 0xFF.
// You can pass an extra bool argument to mark it as a long KV.
func NewStrKeyValue(key, value string, isLong ...bool) *KeyValue {
	isLongKV := false
	if len(isLong) > 0 && isLong[0] {
		isLongKV = true
	}
	kv := keyValuePool.Get().(*KeyValue)
	atomic.StoreInt64(&kv.refCount, 1)
	kv.isLong = isLongKV
	kv.Key = key

	kv.Value = append(kv.Value, value...)
	kv.ValueType = StringType
	if len(kv.Value) > math.MaxUint8 {
		kv.ValueType = TextType
	}

	// Check whether the short kv is too long. If it is too long, then trim the key and value.
	if !kv.isLong {
		if len(kv.Value) > SHORT_STRING_MAX_LEN {
			kv.Value = kv.Value[:SHORT_STRING_MAX_LEN]
			if kv.ValueType == TextType {
				kv.ValueType = StringType
			}
		}
	}
	return kv
}

// NewOmniKeyValue create a long KeyValue instance for any type of key and value.
// Long KVs are put in the content area in the log batch.
func NewOmniKeyValue(key, value interface{}) *KeyValue {
	kv := keyValuePool.Get().(*KeyValue)
	atomic.StoreInt64(&kv.refCount, 1)
	kv.isLong = true

	switch v := key.(type) {
	case string:
		kv.Key = v
	case fmt.Stringer:
		vp := reflect.ValueOf(key)
		if !vp.IsValid() || vp.Kind() == reflect.Ptr && vp.IsNil() {
			kv.Key = fmt.Sprintf("%#v", key)
		} else {
			kv.Key = v.String()
		}
	default:
		byteBuf := make([]byte, 0, 32)
		tmpBuf := (*jsonBuf)(unsafe.Pointer(&byteBuf))
		e := json.NewEncoder(tmpBuf)
		e.SetEscapeHTML(false)
		err := e.Encode(v)
		if err != nil {
			kv.Key = fmt.Sprintf("%#v", v)
		}
		kv.Key = string(byteBuf[:len(byteBuf)-1])
	}

	if len(kv.Key) > SHORT_STRING_MAX_LEN {
		kv.Key = kv.Key[:SHORT_STRING_MAX_LEN]
	}

	switch v := value.(type) {
	case string:
		kv.Value = append(kv.Value, v...)
		kv.ValueType = StringType
		if len(kv.Value) > math.MaxUint8 {
			kv.ValueType = TextType
		}
	case fmt.Stringer:
		vp := reflect.ValueOf(value)
		if !vp.IsValid() || vp.Kind() == reflect.Ptr && vp.IsNil() {
			kv.Value = append(kv.Value, fmt.Sprintf("%#v", value)...)
		} else {
			kv.Value = append(kv.Value, v.String()...)
		}
		kv.ValueType = StringType
		if len(kv.Value) > math.MaxUint8 {
			kv.ValueType = TextType
		}
	case error:
		valueErr := reflect.ValueOf(value)
		kv.ValueType = StringType
		if !valueErr.IsValid() || valueErr.Kind() == reflect.Ptr && valueErr.IsNil() {
			kv.Value = append(kv.Value, fmt.Sprintf("%#v", value)...)
		} else {
			kv.Value = append(kv.Value, v.Error()...)
		}
		if len(kv.Value) > math.MaxUint8 {
			kv.ValueType = TextType
		}
	case bool:
		kv.ValueType = BoolType
		if v {
			kv.Value = EncodeUint8(kv.Value, 1)
		} else {
			kv.Value = EncodeUint8(kv.Value, 0)
		}
	case int8:
		kv.ValueType = IntType
		kv.Value = EncodeUint32(kv.Value, uint32(v))
	case int16:
		kv.ValueType = IntType
		kv.Value = EncodeUint32(kv.Value, uint32(v))
	case int32:
		kv.ValueType = IntType
		kv.Value = EncodeUint32(kv.Value, uint32(v))
	case uint8:
		kv.ValueType = IntType
		kv.Value = EncodeUint32(kv.Value, uint32(v))
	case uint16:
		kv.ValueType = IntType
		kv.Value = EncodeUint32(kv.Value, uint32(v))
	case int:
		kv.ValueType = LongType
		kv.Value = EncodeUint64(kv.Value, uint64(v))
	case int64:
		kv.ValueType = LongType
		kv.Value = EncodeUint64(kv.Value, uint64(v))
	case uint32:
		kv.ValueType = Uint64Type
		kv.Value = EncodeUint64(kv.Value, uint64(v))
	case uint:
		kv.ValueType = Uint64Type
		kv.Value = EncodeUint64(kv.Value, uint64(v))
	case uint64:
		kv.ValueType = Uint64Type
		kv.Value = EncodeUint64(kv.Value, v)
	case float32:
		kv.ValueType = DoubleType
		kv.Value = EncodeUint64(kv.Value, math.Float64bits(float64(v)))
	case float64:
		kv.ValueType = DoubleType
		kv.Value = EncodeUint64(kv.Value, math.Float64bits(v))
	case []byte:
		kv.ValueType = BytesType
		kv.Value = append(kv.Value, v...)
	default:
		kv.ValueType = StringType
		byteBuf := make([]byte, 0, 32)
		tmpBuf := (*jsonBuf)(unsafe.Pointer(&byteBuf))
		e := json.NewEncoder(tmpBuf)
		e.SetEscapeHTML(false)
		err := e.Encode(v)
		if err != nil {
			kv.Value = append(kv.Value, fmt.Sprintf("%#v", v)...)
		} else {
			kv.Value = append(kv.Value, byteBuf[:len(byteBuf)-1]...)
		}
		if len(kv.Value) > SHORT_STRING_MAX_LEN {
			kv.ValueType = TextType
		}
	}

	return kv
}

// Recycle puts the kv back the kv pool.
// Users should not call this function unless they understand what they are doing.
func (kv *KeyValue) Recycle() {
	if atomic.CompareAndSwapInt64(&kv.refCount, 1, 0) {
		kv.Key = ""
		kv.Value = kv.Value[:0]
		kv.ValueType = 0
		kv.isLong = false

		// avoid recycling kvs that exceed the threshold.
		if cap(kv.Value) < kvRecycleCapacityThreshold {
			keyValuePool.Put(kv)
		}
	} else {
		atomic.AddInt64(&kv.refCount, -1)
	}
}

var keyValuePool = &sync.Pool{
	New: func() interface{} {
		return &KeyValue{
			Key:       "",
			Value:     make([]byte, 0, 16),
			ValueType: 0,
			isLong:    false,
			refCount:  0,
		}
	},
}

func (kv *KeyValue) IsLong() bool {
	return kv.isLong
}

func (kv *KeyValue) Encode(buf []byte) []byte {
	return EncodeKeyValue(buf, kv.Key, kv.Value, kv.ValueType)
}

func (kv *KeyValue) Size() int {
	switch kv.ValueType {
	case TextType:
		return EncodedKVSizeStr(kv.Key, SliceByteToString(kv.Value))
	case StringType:
		return EncodedKVSizeStr(kv.Key, SliceByteToString(kv.Value))
	case BytesType:
		return EncodedStringSize(kv.Key) + len(kv.Value) + 5
	default:
		return EncodedStringSize(kv.Key) + len(kv.Value) + 1
	}
}

// String() returns a string "{key}={value}".
func (kv *KeyValue) String() string {
	var valueStr string
	switch kv.ValueType {
	case BoolType:
		if len(kv.Value) > 0 && kv.Value[0] == 1 {
			valueStr = "true"
		} else {
			valueStr = "false"
		}
	case IntType:
		valInt, _ := DecodeUint32(kv.Value)
		varInt64 := int64(int32(valInt))
		valueStr = strconv.FormatInt(varInt64, 10)
	case LongType:
		valLong, _ := DecodeUint64(kv.Value)
		valueStr = strconv.FormatInt(int64(valLong), 10)
	case Uint64Type:
		valUint64, _ := DecodeUint64(kv.Value)
		valueStr = strconv.FormatUint(valUint64, 10)
	case DoubleType:
		valInt64, _ := DecodeUint64(kv.Value)
		valDouble := math.Float64frombits(valInt64)
		valueStr = strconv.FormatFloat(valDouble, 'f', -1, 64)
	case BytesType:
		valueStr = fmt.Sprintf("%v", kv.Value)
	default:
		valueStr = string(kv.Value)
	}
	return kv.Key + "=" + valueStr
}

// ToKV returns a key-value pair
func (kv *KeyValue) ToKV() (string, string) {
	var valueStr string
	switch kv.ValueType {
	case BoolType:
		if len(kv.Value) > 0 && kv.Value[0] == 1 {
			valueStr = "true"
		} else {
			valueStr = "false"
		}
	case IntType:
		valInt, _ := DecodeUint32(kv.Value)
		valueStr = strconv.FormatInt(int64(int32(valInt)), 10)
	case LongType:
		valLong, _ := DecodeUint64(kv.Value)
		valueStr = strconv.FormatInt(int64(valLong), 10)
	case Uint64Type:
		valUint64, _ := DecodeUint64(kv.Value)
		valueStr = strconv.FormatUint(valUint64, 10)
	case DoubleType:
		valInt64, _ := DecodeUint64(kv.Value)
		valDouble := math.Float64frombits(valInt64)
		valueStr = strconv.FormatFloat(valDouble, 'f', -1, 64)
	case BytesType:
		valueStr = fmt.Sprintf("%v", kv.Value)
	default:
		valueStr = string(kv.Value)
	}
	return kv.Key, valueStr
}
func (kv *KeyValue) EncodeAsStr(buf []byte) []byte {
	buf = append(buf, kv.Key...)
	buf = append(buf, equalByte)
	var valueStr string
	switch kv.ValueType {
	case BoolType:
		if len(kv.Value) > 0 && kv.Value[0] == 1 {
			valueStr = "true"
		} else {
			valueStr = "false"
		}
	case IntType:
		valInt, _ := DecodeUint32(kv.Value)
		valueStr = strconv.FormatInt(int64(int32(valInt)), 10)
	case LongType:
		valLong, _ := DecodeUint64(kv.Value)
		valueStr = strconv.FormatInt(int64(valLong), 10)
	case Uint64Type:
		valUint64, _ := DecodeUint64(kv.Value)
		valueStr = strconv.FormatUint(valUint64, 10)
	case DoubleType:
		valInt64, _ := DecodeUint64(kv.Value)
		valDouble := math.Float64frombits(valInt64)
		valueStr = strconv.FormatFloat(valDouble, 'f', -1, 64)
	case BytesType:
		valueStr = fmt.Sprintf("%v", kv.Value)
	case StringType, TextType:
		valueStr = SliceByteToString(kv.Value)
	default:
		valueStr = string(kv.Value)
	}
	buf = append(buf, valueStr...)
	return buf
}

func (kv *KeyValue) Clone() *KeyValue {
	atomic.AddInt64(&kv.refCount, 1)
	return kv
}

func EncodeUint8(buf []byte, i uint8) []byte {
	return append(buf, i)
}

func EncodeUint16(buf []byte, i uint16) []byte {
	return append(buf, byte(i), byte(i>>8))
}

func EncodeUint32(buf []byte, i uint32) []byte {
	return append(buf, byte(i), byte(i>>8), byte(i>>16), byte(i>>24))
}

func EncodeUint64(buf []byte, i uint64) []byte {
	return append(buf, byte(i), byte(i>>8), byte(i>>16), byte(i>>24), byte(i>>32), byte(i>>40), byte(i>>48), byte(i>>56))
}

// EncodeKeyValue encodes a key-value pair.
// The value should be a string, a text, or byte[]
func EncodeKeyValue(buf []byte, k string, v []byte, valueType byte) []byte {
	buf = encodeStr(buf, k)
	buf = encodeVarWithType(buf, v, valueType)
	return buf
}

func encodeStr(buf []byte, s string) []byte {
	if len(s) <= math.MaxUint8 {
		return encodeShortStr(buf, s)
	} else {
		return encodeLongStr(buf, s)
	}
}

// encodeShortStr encodes a short string whose length is <= 255.
// Codec scheme of short string is as follows.
// Fields:     | type | length | value | delimiter |
// num of bytes:  1      1         n        1
func encodeShortStr(buf []byte, s string) []byte {
	if len(s) >= math.MaxUint8 {
		s = s[:math.MaxUint8]
	}
	buf = append(buf, StringType)
	buf = EncodeUint8(buf, uint8(len(s)))
	buf = append(buf, s...)
	buf = append(buf, StringSplitByte)
	return buf
}

// encodeLongStr encodes a long string which is also called text.
// Codec scheme of a long string is as follows.
// Fields:     | type | length | value | delimiter |
// num of bytes:  1      4         n        1
func encodeLongStr(buf []byte, s string) []byte {
	if len(s) >= math.MaxInt32 {
		s = s[:math.MaxInt32]
	}
	buf = append(buf, TextType)
	buf = EncodeUint32(buf, uint32(len(s)))
	buf = append(buf, s...)
	buf = append(buf, StringSplitByte)
	return buf
}

// encodeVarWithType is a helper function that directly write the type and bytes to buf.
// If we only know the type, it still writes some default bytes.
func encodeVarWithType(buf []byte, value []byte, valueType byte) []byte {
	if valueType == StringType {
		return encodeShortStr(buf, SliceByteToString(value))
	} else if valueType == TextType {
		return encodeLongStr(buf, SliceByteToString(value))
	} else if valueType == BytesType {
		return encodeBytes(buf, value)
	} else {
		buf = append(buf, valueType)
		switch valueType {
		case BoolType:
			if value == nil || len(value) < 1 {
				return EncodeUint8(buf, 0)
			}
			return append(buf, value[:1]...)
		case IntType:
			if value == nil || len(value) < 4 {
				return EncodeUint32(buf, 0)
			}
			return append(buf, value[:4]...)
		case LongType:
			if value == nil || len(value) < 8 {
				return EncodeUint64(buf, 0)
			}
			return append(buf, value[:8]...)
		case Uint64Type:
			if value == nil || len(value) < 8 {
				return EncodeUint64(buf, 0)
			}
			return append(buf, value[:8]...)
		case DoubleType:
			if value == nil || len(value) < 8 {
				return EncodeUint64(buf, 0)
			}
			return append(buf, value[:8]...)
		default:
			return append(buf, value...)
		}
	}
}

func encodeBytes(buf []byte, data []byte) []byte {
	if len(data) >= math.MaxInt32 {
		data = data[:math.MaxInt32]
	}
	buf = append(buf, BytesType)
	buf = EncodeUint32(buf, uint32(len(data)))
	buf = append(buf, data...)
	return buf
}

func encodeUint64(buf []byte, value uint64) []byte {
	buf = append(buf, Uint64Type)
	return EncodeUint64(buf, value)
}

func encodeInt(buf []byte, value uint32) []byte {
	buf = append(buf, IntType)
	return EncodeUint32(buf, value)
}

func encodeLong(buf []byte, value uint64) []byte {
	buf = append(buf, LongType)
	return EncodeUint64(buf, value)
}

func encodeDouble(buf []byte, value float64) []byte {
	buf = encodeVarWithType(buf, nil, DoubleType)
	WriteUint64(buf, len(buf)-8, math.Float64bits(value))
	return buf
}

func encodeBool(buf []byte, value bool) []byte {
	buf = append(buf, BoolType)
	if value {
		return EncodeUint8(buf, 1)
	}
	return EncodeUint8(buf, 0)
}

func StringToUint32(s string) (uint32, error) {
	return DecodeUint32(StringToSliceByte(s))
}

func StringToSliceByte(s string) []byte {
	l := len(s)
	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: (*(*reflect.StringHeader)(unsafe.Pointer(&s))).Data,
		Len:  l,
		Cap:  l,
	}))
}

func SliceByteToString(buf []byte) string {
	l := len(buf)
	return *(*string)(unsafe.Pointer(&reflect.StringHeader{
		Data: (*(*reflect.SliceHeader)(unsafe.Pointer(&buf))).Data,
		Len:  l,
	}))
}

func WriteUint8(buf []byte, pos int, value uint8) {
	buf[pos] = value
}

func WriteUint64(buf []byte, pos int, value uint64) {
	_ = buf[pos+7]
	buf[pos+0] = byte(value)
	buf[pos+1] = byte(value >> 8)
	buf[pos+2] = byte(value >> 16)
	buf[pos+3] = byte(value >> 24)
	buf[pos+4] = byte(value >> 32)
	buf[pos+5] = byte(value >> 40)
	buf[pos+6] = byte(value >> 48)
	buf[pos+7] = byte(value >> 56)
}

func WriteUint32(buf []byte, pos int, value uint32) {
	_ = buf[pos+3]
	buf[pos+0] = byte(value)
	buf[pos+1] = byte(value >> 8)
	buf[pos+2] = byte(value >> 16)
	buf[pos+3] = byte(value >> 24)
}

func WriteUint16(buf []byte, pos int, value uint16) {
	_ = buf[pos+1]
	buf[pos+0] = byte(value)
	buf[pos+1] = byte(value >> 8)
}

func WriteUint64Hex(buf []byte, pos int, value uint64) {
	_ = buf[pos+15]
	j := 0
	for j < 8 {
		v := byte(value >> (j * 8))
		buf[pos+(7-j)*2] = hextable[v>>4]
		buf[pos+(7-j)*2+1] = hextable[v&0x0f]
		j += 1
	}
}

func WriteBytes(buf []byte, pos int, value []byte, len int) {
	if len == 0 {
		return
	}
	_, _ = value[len-1], buf[pos+len-1]
	for i := 0; i < len; i++ {
		buf[pos+i] = value[i]
	}
}

func EncodeKeyValueStr(buf []byte, k, v string) []byte {
	buf = encodeStr(buf, k)
	buf = encodeStr(buf, v)
	return buf
}

func EncodeKeyValueText(buf []byte, k, v string) []byte {
	buf = encodeStr(buf, k)
	buf = encodeLongStr(buf, v)
	return buf
}

func EncodeKeyValueStrIfNecessary(buf []byte, k, v string) []byte {
	if len(k) == 0 || len(v) == 0 {
		return buf
	}
	buf = encodeStr(buf, k)
	buf = encodeStr(buf, v)
	return buf
}

func EncodeKeyValueUint64(buf []byte, k string, v uint64) []byte {
	buf = encodeStr(buf, k)
	buf = encodeUint64(buf, v)
	return buf
}

func EncodeKeyValueUint32(buf []byte, k string, v uint32) []byte {
	buf = encodeStr(buf, k)
	buf = encodeInt(buf, v)
	return buf
}

// EncodedStringSize computes the actual size of a string after encoding.
func EncodedStringSize(s string) int {
	strLen := len(s)
	if strLen > SHORT_STRING_MAX_LEN {
		return encodedLongStringSize(s)
	} else {
		return encodedShortStringSize(s)
	}
}

// encodedLongStringSize computes the actual size of a long string after encoding.
// Codec scheme of short string is as follows.
// Fields:     | type | length | value | delimiter |
// num of bytes:  1      4         n        1
func encodedLongStringSize(s string) int {
	strLen := len(s)
	if strLen > LONG_STRING_MAX_LEN {
		strLen = LONG_STRING_MAX_LEN
	}
	return strLen + 6
}

// encodedShortStringSize computes the actual size of a short string after encoding.
// Codec scheme of short string is as follows.
// Fields:     | type | length | value | delimiter |
// num of bytes:  1      1         n        1
func encodedShortStringSize(s string) int {
	strLen := len(s)
	if strLen > SHORT_STRING_MAX_LEN {
		strLen = SHORT_STRING_MAX_LEN
	}
	return strLen + 3
}

// EncodedKVSizeStr computes the actual size of a key-value pair after encoding.
// The value is a string, e.g., {"_psm", "toutiao.service.log"}
func EncodedKVSizeStr(k, v string) int {
	return EncodedStringSize(k) + EncodedStringSize(v)
}

// EncodedKVSizeText computes the actual size of a key-value pair after encoding.
// But the value must be a text, e.g., {"_msg", "my message"}
func EncodedKVSizeText(k, v string) int {
	return EncodedStringSize(k) + encodedLongStringSize(v)
}

// EncodedKVIPv4Size computes the actual size of a key-value pair when the value is ipv4.
// | {Key} | length | ipv4
func EncodedKVIPv4Size(key string) int {
	return EncodedStringSize(key) + 1 + BYTELOG_IPV4_BYTES
}

// EncodedKVIPv6Size computes the actual size of a key-value pair when the value is ipv6.
// | {Key} | length| ipv6
func EncodedKVIPv6Size(key string) int {
	return EncodedStringSize(key) + 1 + BYTELOG_IPV6_BYTES
}

// EncodedKVIBatchIDSize computes the actual size of a key-value pair when the value is batchid
func EncodedKVIBatchIDSize(key string, value string) int {
	return EncodedStringSize(key) + EncodedStringSize(value) + SEQ_ID_LENGTH
}

// EncodedKVFlagSize computes the actual size of a key-value pair when the value is ipv4.
// | {Key} | length | ipv4
func EncodedKVFlagSize(key string) int {
	return EncodedStringSize(key) + 1 + BYTELOG_FLAG_BYTES
}

/*
 * All decode functions will return the instance.
 * For decodedData with fixed size, it may also return an error.
 * For decodedData with uncertain length, it also returns the readLength value, which means how many bytes it processed.
 * Then we can process the remaining bytes from there.
 */

func decodeBytes(data []byte, offset int) (value []byte, readLength int, err error) {
	length, l, err := decodeUint32(data, offset)
	if err != nil {
		return nil, 0, err
	}
	end := l + int(length)
	if end > len(data)-offset {
		return nil, 0, ErrNoEnoughBytes
	}
	// bytes type not end of '\x00'
	return data[offset+l : offset+end], end, nil
}

func decodeUint32(data []byte, offset int) (value uint32, readLength int, err error) {
	if len(data) < 4+offset {
		return 0, 0, ErrNoEnoughBytes
	}
	return uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24, 4, nil
}

func decodeByteRaw(data []byte, offset int) (value byte, readLength int, err error) {
	if len(data) < 1+offset {
		return 0, 0, ErrNoEnoughBytes
	}
	return data[offset], 1, nil
}

func decodeUint8(data []byte, offset int) (value uint8, readLength int, err error) {
	if len(data) < 1+offset {
		return 0, 0, ErrNoEnoughBytes
	}
	return data[offset], 1, nil
}

func decodeUint16(data []byte, offset int) (value uint16, readLength int, err error) {
	if len(data) < 2+offset {
		return 0, 0, ErrNoEnoughBytes
	}
	return uint16(data[offset]) | uint16(data[offset+1])<<8, 2, nil
}

func decodeUint64(data []byte, offset int) (value uint64, readLength int, err error) {
	if len(data) < 8+offset {
		return 0, 0, ErrNoEnoughBytes
	}
	return uint64(data[offset]) | uint64(data[offset+1])<<8 | uint64(data[offset+2])<<16 | uint64(data[offset+3])<<24 |
		uint64(data[offset+4])<<32 | uint64(data[offset+5])<<40 | uint64(data[offset+6])<<48 | uint64(data[offset+7])<<56, 8, nil
}

func DecodeUint8(data []byte) (uint8, error) {
	if len(data) < 1 {
		return 0, ErrNoEnoughBytes
	}
	return data[0], nil
}

func DecodeUint16(data []byte) (uint16, error) {
	if len(data) < 2 {
		return 0, ErrNoEnoughBytes
	}
	return uint16(data[0]) | uint16(data[1])<<8, nil
}

func DecodeUint32(data []byte) (uint32, error) {
	if len(data) < 4 {
		return 0, ErrNoEnoughBytes
	}
	return uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24, nil
}

func DecodeUint64(b []byte) (uint64, error) {
	if len(b) < 8 {
		return 0, ErrNoEnoughBytes
	}
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56, nil
}

func ValueSize(o interface{}) int {
	switch v := o.(type) {
	case nil:
		return EncodedStringSize("")
	case fmt.Stringer:
		value := reflect.ValueOf(o)
		if !value.IsValid() || value.Kind() == reflect.Ptr && value.IsNil() {
			valueStr := fmt.Sprintf("%#v", o)
			return EncodedStringSize(valueStr)
		}
		return EncodedStringSize(v.String())
	case string:
		return EncodedStringSize(v)
	case error:
		value := reflect.ValueOf(o)
		if !value.IsValid() || value.Kind() == reflect.Ptr && value.IsNil() {
			valueStr := fmt.Sprintf("%#v", o)
			return EncodedStringSize(valueStr)
		}
		return EncodedStringSize(v.Error())
	case bool:
		return 1 + 1
	case int8:
		return 4 + 1
	case int16:
		return 4 + 1
	case uint8:
		return 4 + 1
	case uint16:
		return 4 + 1
	case int32:
		return 4 + 1
	case int:
		return 8 + 1
	case int64:
		return 8 + 1
	case uint32:
		return 8 + 1
	case uint64:
		return 8 + 1
	case float32:
		return 8 + 1
	case float64:
		return 8 + 1
	default:
		byteBuf := make([]byte, 0, 32)
		tmpBuf := (*jsonBuf)(unsafe.Pointer(&byteBuf))
		e := json.NewEncoder(tmpBuf)
		e.SetEscapeHTML(false)
		err := e.Encode(v)
		if err != nil {
			valueStr := fmt.Sprintf("%#v", o)
			return EncodedStringSize(valueStr)
		}
		return EncodedStringSize(SliceByteToString(tmpBuf.buf))
	}
}

// ValueToBytes encodes an interface to bytes.
// It also puts the type byte in front of the data bytes.
func ValueToBytes(buf []byte, o interface{}) []byte {
	switch v := o.(type) {
	case nil:
		return append(buf, emptyStrForByteLog...)
	case fmt.Stringer:
		value := reflect.ValueOf(o)
		var valueStr string
		if !value.IsValid() || value.Kind() == reflect.Ptr && value.IsNil() {
			valueStr = fmt.Sprintf("%#v", o)
			return encodeStr(buf, valueStr)
		}
		valueStr = v.String()
		return encodeStr(buf, valueStr)
	case string:
		return encodeStr(buf, v)
	case error:
		value := reflect.ValueOf(o)
		if !value.IsValid() || value.Kind() == reflect.Ptr && value.IsNil() {
			valueStr := fmt.Sprintf("%#v", o)
			return encodeStr(buf, valueStr)
		}
		return encodeStr(buf, v.Error())
	case bool:
		return encodeBool(buf, v)
	case int8:
		return encodeInt(buf, uint32(v))
	case int16:
		return encodeInt(buf, uint32(v))
	case int32:
		return encodeInt(buf, uint32(v))
	case uint8:
		return encodeInt(buf, uint32(v))
	case uint16:
		return encodeInt(buf, uint32(v))
	case int:
		return encodeLong(buf, uint64(v))
	case int64:
		return encodeLong(buf, uint64(v))
	case uint32:
		return encodeUint64(buf, uint64(v))
	case uint64:
		return encodeUint64(buf, v)
	case float32:
		return encodeDouble(buf, float64(v))
	case float64:
		return encodeDouble(buf, v)
	default:
		byteBuf := make([]byte, 0, 32)
		tmpBuf := (*jsonBuf)(unsafe.Pointer(&byteBuf))
		e := json.NewEncoder(tmpBuf)
		e.SetEscapeHTML(false)
		err := e.Encode(v)
		if err != nil {
			valueStr := fmt.Sprintf("%#v", o)
			return encodeStr(buf, valueStr)
		}
		if len(tmpBuf.buf) > SHORT_STRING_MAX_LEN {
			return encodeVarWithType(buf, tmpBuf.buf, TextType)
		}
		return encodeVarWithType(buf, tmpBuf.buf, StringType)
	}
}

// ValuesToBytes converts some objects to bytes and append them to the buf.
// It calls ValueToBytes indeed.
func ValuesToBytes(buf []byte, os ...interface{}) []byte {
	for _, o := range os {
		buf = ValueToBytes(buf, o)
	}
	return buf
}

// encodeVarStr encodes a VarString instance: {length: uint8, content string}.
// The length should be 1 + len(content). There is No '\0' at the end of the content.
func encodeVarStr(buf []byte, vs VarString) []byte {
	if len(vs.content) > math.MaxInt8 {
		vs.content = vs.content[:math.MaxInt8]
	}
	buf = EncodeUint8(buf, uint8(len(vs.content)))
	buf = append(buf, vs.content...)
	return buf
}

func decodeMultiValueLength(data []byte, offset, count int) (readLength int, err error) {
	pos := 0
	for i := 0; i < count; i++ {
		readLength, err = decodeValueLengthWithUnknownType(data, offset+pos)
		if err != nil {
			return 0, err
		}
		pos += readLength
	}
	return pos, nil
}

// decodeValueLengthWithUnknownType decode an unknown object length with its type
func decodeValueLengthWithUnknownType(data []byte, offset int) (readLength int, err error) {
	valueType, l, err := decodeByteRaw(data, offset)
	if err != nil {
		return 0, err
	}
	readLength, err = decodeValueLengthWithType(data, offset+l, valueType)
	if err != nil {
		return 0, err
	}
	return readLength + l, nil
}

func decodeValueLengthWithType(data []byte, offset int, valueType byte) (readLength int, err error) {
	switch valueType {
	case StringType:
		length, l, err := decodeUint8(data, offset)
		if err != nil {
			return 0, err
		}
		return int(length) + l + 1, nil
	case BoolType:
		return 1, nil
	case TextType:
		length, l, err := decodeUint32(data, offset)
		if err != nil {
			return 0, err
		}
		return int(length) + l + 1, nil
	case LongType:
		return 8, nil
	case IntType:
		return 4, nil
	case Uint64Type:
		return 8, nil
	case DoubleType:
		return 8, nil
	case Ipv4Type:
		return 4, nil
	case Ipv6Type:
		return 16, nil
	}
	return 0, fmt.Errorf("unsupported value, data: %v", data)
}

func bytesToShortStrings(data []byte, count int) ([]string, int) {
	res := make([]string, count)
	pos := 0
	for i := 0; i < count; i++ {
		shortStr, readLength, err := decodeShortStrWithType(data, pos)
		if err != nil || readLength == 0 {
			return nil, 0
		}
		pos += readLength
		res[i] = shortStr
	}

	return res, pos
}

func decodeShortStrWithType(data []byte, offset int) (res string, readLength int, err error) {
	valueType, l, err := decodeByteRaw(data, offset)
	if err != nil {
		return "", 0, err
	}
	if valueType != StringType {
		return "", 0, errors.New("not a short string")
	}
	res, readLength, err = decodeShortStr(data, offset+l)
	if err != nil {
		return "", 0, err
	}
	return res, readLength + l, nil
}

func decodeShortStr(data []byte, offset int) (res string, readLength int, err error) {
	length, l, err := decodeUint8(data, offset)
	if err != nil {
		return "", 0, err
	}
	end := l + int(length)
	if end > len(data)-offset {
		return "", 0, ErrNoEnoughBytes
	}
	return string(data[offset+l : offset+end]), end + 1, nil
}

func bytesToShortKVs(data []byte, length int) ([]*KeyValue, int, error) {
	pos := 0
	res := make([]*KeyValue, 0, 10)

	for pos < length {
		key, readLength, err := decodeValueWithUnknownType(data, pos)
		if err != nil || readLength == 0 {
			return nil, 0, err
		}
		pos += readLength
		value, readLength, err := decodeValueWithUnknownType(data, pos)
		if err != nil || readLength == 0 {
			return nil, 0, err
		}
		pos += readLength
		kv, _ := NewKeyValue(key, value)
		res = append(res, kv)
	}
	if pos != length {
		return res, pos, errors.New(" length not match in short string list")
	}
	return res, pos, nil
}

// decodeValueWithUnknownType decodes an unknown object with its type
func decodeValueWithUnknownType(data []byte, offset int) (value interface{}, readLength int, err error) {
	valueType, l, err := decodeByteRaw(data, offset)
	if err != nil {
		return "", 0, err
	}
	value, readLength, err = decodeValueWithType(data, offset+l, valueType)
	if err != nil {
		return "", 0, err
	}
	return value, readLength + l, nil
}

func decodeValueWithType(data []byte, offset int, valueType byte) (value interface{}, readLength int, err error) {
	switch valueType {
	case StringType:
		return decodeShortStr(data, offset)
	case BoolType:
		value = false
		if len(data) < 1+offset {
			return value, 0, err
		}
		if data[offset] == 1 {
			value = true
		}
		return value, 1, nil
	case BytesType:
		return decodeBytes(data, offset)
	case TextType:
		return decodeLongStr(data, offset)
	case LongType:
		val, readLength, err := decodeUint64(data, offset)
		return int(val), readLength, err
	case IntType:
		return decodeUint32(data, offset)
	case Uint64Type:
		return decodeUint64(data, offset)
	case DoubleType:
		return decodeDouble(data, offset)
	case Ipv4Type:
		return decodeIpv4(data, offset)
	case Ipv6Type:
		return decodeIpv6(data, offset)
	}
	return nil, 0, fmt.Errorf("unsupported value, data: %v", data)
}

func decodeLongStr(data []byte, offset int) (res string, readLength int, err error) {
	length, l, err := decodeUint32(data, offset)
	if err != nil {
		return "", 0, err
	}
	end := l + int(length)
	if end > len(data)-offset {
		return "", 0, ErrNoEnoughBytes
	}
	return string(data[offset+l : offset+end]), end + 1, nil // Note that there is '\x00' at the end of this string
}

func decodeLongStrWithType(data []byte, offset int) (res string, readLength int, err error) {
	valueType, l, err := decodeByteRaw(data, offset)
	if err != nil {
		return "", 0, err
	}
	if valueType != TextType {
		return "", 0, errors.New("not a long string")
	}
	res, readLength, err = decodeLongStr(data, offset+l)
	if err != nil {
		return "", 0, err
	}
	return res, readLength + l, nil // Note that there is '\x00' at the end of this string
}

func decodeIpv4(data []byte, offset int) (value Ipv4, readLength int, err error) {
	ipv4, l, err := decodeUint32(data, offset)
	if err != nil {
		return 0, 0, err
	}
	return Ipv4(ipv4), l, nil
}

func decodeIpv4WithType(data []byte, offset int) (value Ipv4, readLength int, err error) {
	valueType, l, err := decodeByteRaw(data, offset)
	if err != nil {
		return 0, 0, err
	}
	if valueType != Ipv4Type {
		return 0, 0, errors.New("not a Ipv4 variable")
	}
	value, readLength, err = decodeIpv4(data, offset+l)
	if err != nil {
		return 0, 0, err
	}
	return value, readLength + l, nil
}

func decodeIpv6(data []byte, offset int) (value Ipv6, readLength int, err error) {
	if len(data) < offset+16 {
		return value, 0, ErrNoEnoughBytes
	}
	copy(value[:], data[offset:offset+16])
	return value, 16, nil
}

func decodeIpv6WithType(data []byte, offset int) (value Ipv6, readLength int, err error) {
	valueType, l, err := decodeByteRaw(data, offset)
	if err != nil {
		return value, 0, err
	}
	if valueType != Ipv6Type {
		return value, 0, errors.New("not a Ipv6 variable")
	}
	value, readLength, err = decodeIpv6(data, offset+l)
	if err != nil {
		return value, 0, err
	}
	return value, readLength + l, nil
}

func decodeUint64WithType(data []byte, offset int) (value uint64, readLength int, err error) {
	valueType, l, err := decodeByteRaw(data, offset)
	if err != nil {
		return 0, 0, err
	}
	if valueType != LongType && valueType != Uint64Type {
		return 0, 0, errors.New("not a long variable")
	}
	value, readLength, err = decodeUint64(data, offset+l)
	if err != nil {
		return 0, 0, err
	}
	return value, readLength + l, nil
}

func decodeUint32WithType(data []byte, offset int) (value uint32, readLength int, err error) {
	valueType, l, err := decodeByteRaw(data, offset)
	if err != nil {
		return 0, 0, err
	}
	if valueType != IntType {
		return 0, 0, errors.New("not a long variable")
	}
	value, readLength, err = decodeUint32(data, offset+l)
	if err != nil {
		return 0, 0, err
	}
	return value, readLength + l, nil
}

func decodeDouble(data []byte, offset int) (value float64, readLength int, err error) {
	bits, l, err := decodeUint64(data, offset)
	if err != nil {
		return 0, 0, err
	}
	value = math.Float64frombits(bits)
	return value, l, nil
}

func decodeDoubleWithType(data []byte, offset int) (value float64, readLength int, err error) {
	valueType, l, err := decodeByteRaw(data, offset)
	if err != nil {
		return 0, 0, err
	}
	if valueType != DoubleType {
		return 0, 0, errors.New("not a long variable")
	}

	value, readLength, err = decodeDouble(data, offset+l)
	if err != nil {
		return 0, 0, err
	}
	return value, readLength + l, nil
}

func sliceByteArray(data []byte, low, high int) (value []byte, err error) {
	if low > high {
		return nil, errors.New("invalid index")
	}
	if low < 0 {
		return nil, errors.New("invalid index")
	}
	length := len(data)
	if low > length || high > length {
		return nil, errors.New("index ouf of range")
	}
	return data[low:high], nil
}

func usToMinute(timeStampInUs uint64) uint64 {
	return timeStampInUs / uint64(time.Minute/time.Microsecond)
}

type Packet *[]byte

// NewPacket gets a usable byte slice.
func NewPacket(length int) Packet {
	p := packetPool.Get().(Packet)
	*p = (*p)[:0]
	if length > 0 {
		*p = append(*p, make([]byte, length)...)
	}
	return p
}

var packetPool = &sync.Pool{
	New: func() interface{} {
		data := make([]byte, 0, 256)
		return Packet(&data)
	},
}

func PutPacket(p Packet) {
	*p = (*p)[:0]
	packetPool.Put(p)
}

func DecodeVarString(data []byte) (VarString, error) {
	if len(data) < 2 {
		return VarString{}, ErrNoEnoughBytes
	}
	lenStr, _ := DecodeUint8(data)
	return VarString{lenStr, string(data[1 : lenStr+1])}, nil
}
