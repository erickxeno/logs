package writer

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

var (
	letters = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789,./;'[]\\-=<>?:{}|+_)(*&^%$#@!")
)

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func TestEncodeNumeric(t *testing.T) {
	buf := make([]byte, 0)
	buf = EncodeUint8(buf, 42)
	assert.Equal(t, buf[0], byte(42))
	buf = make([]byte, 0)
	var i int32 = 420
	buf = EncodeUint32(buf, 420)
	assert.Equal(t, buf[0], byte(i))
	assert.Equal(t, buf[0], byte(i))
	buf = make([]byte, 0)
	buf = EncodeUint64(buf, 42)
	buf = make([]byte, 0)
	buf = encodeLongStr(buf, "42")
	assert.Equal(t, buf[0], TextType)
	assert.Equal(t, buf[5:7], []byte("42"))
	buf = make([]byte, 0)
}

func TestCodecStrAndText(t *testing.T) {
	buf := make([]byte, 0)
	rawStr := "hello, world!"
	buf = encodeLongStr(buf, rawStr)
	assert.Equal(t, len(buf), 1+4+13+1)
	decStr, readLength, err := decodeLongStrWithType(buf, 0)
	assert.Nil(t, err)
	assert.Equal(t, decStr, rawStr)
	assert.Equal(t, readLength, 19)
	buf = buf[:0]
	buf = encodeShortStr(buf, rawStr)
	assert.Equal(t, len(buf), 1+1+13+1)
	decStr, readLength, err = decodeShortStrWithType(buf, 0)
	assert.Nil(t, err)
	assert.Equal(t, decStr, rawStr)
	assert.Equal(t, readLength, 16)
}

func TestStringToUint32(t *testing.T) {
	magicNumber := "SLOG"
	value, err := StringToUint32(magicNumber)
	assert.Nil(t, err)
	buf := make([]byte, 0)
	buf = EncodeUint32(buf, value)
	decodedStr := string(buf)
	assert.Equal(t, magicNumber, decodedStr)
}

func TestCodecUintX(t *testing.T) {
	buf := make([]byte, 0, 100)
	var i1 uint8 = 100
	var i2 uint16 = 200
	var i3 uint32 = 300
	var i4 uint64 = 600
	buf = EncodeUint8(buf, i1)
	buf = EncodeUint16(buf, i2)
	buf = EncodeUint32(buf, i3)
	buf = EncodeUint64(buf, i4)

	d1, _ := DecodeUint8(buf[0:])
	d2, _ := DecodeUint16(buf[1:])
	d3, _ := DecodeUint32(buf[3:])
	d4, _ := DecodeUint64(buf[7:])
	assert.Equal(t, i1, d1)
	assert.Equal(t, i2, d2)
	assert.Equal(t, i3, d3)
	assert.Equal(t, i4, d4)

	buf = buf[:0]
	_, err := DecodeUint8(buf[0:])
	assert.ErrorIs(t, err, ErrNoEnoughBytes)
	_, err = DecodeUint16(buf[0:])
	assert.ErrorIs(t, err, ErrNoEnoughBytes)
	_, err = DecodeUint32(buf[0:])
	assert.ErrorIs(t, err, ErrNoEnoughBytes)
	_, err = DecodeUint64(buf[0:])
	assert.ErrorIs(t, err, ErrNoEnoughBytes)
	_, _, err = decodeByteRaw(buf, 0)
	assert.ErrorIs(t, err, ErrNoEnoughBytes)
}

func BenchmarkEncodeUintXLittleEndian(b *testing.B) {
	buf := make([]byte, 0, 100)
	var i1 uint8 = 100
	var i2 uint16 = 200
	var i3 uint32 = 300
	var i4 uint64 = 600
	buf = EncodeUint8(buf, i1)
	buf = EncodeUint16(buf, i2)
	buf = EncodeUint32(buf, i3)
	buf = EncodeUint64(buf, i4)

	b.Run("DecodeUint", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _ = DecodeUint8(buf[0:])
				_, _ = DecodeUint16(buf[1:])
				_, _ = DecodeUint32(buf[3:])
				_, _ = DecodeUint64(buf[7:])
			}
		})
	})

}

func TestStringToSliceByte(t *testing.T) {
	buf := make([]byte, 0)
	buf = append(buf, "ABCDEFG"...)
	s1 := string(buf)
	s2 := SliceByteToString(buf)
	buf2 := StringToSliceByte(s2)
	assert.Equal(t, len(buf), len(buf2))
	assert.Equal(t, len(s1), len(s2))
	assert.Equal(t, len(s1), len(buf))
	assert.NotEqual(t, *(*uintptr)(unsafe.Pointer(&s1)), *(*uintptr)(unsafe.Pointer(&s2)))
	assert.Equal(t, *(*uintptr)(unsafe.Pointer(&buf)), *(*uintptr)(unsafe.Pointer(&s2)))
	assert.Equal(t, *(*uintptr)(unsafe.Pointer(&buf)), *(*uintptr)(unsafe.Pointer(&buf2)))
}

func TestWriteUint64Hex(t *testing.T) {
	var val uint64 = 0x9182736455463728
	buf := make([]byte, 16)
	WriteUint64Hex(buf, 0, val)
	assert.Equal(t, "9182736455463728", string(buf))

	var val2 uint64 = 0x01
	buf2 := make([]byte, 16)
	WriteUint64Hex(buf2, 0, val2)
	assert.Equal(t, "0000000000000001", string(buf2))
}

var rawS = "12345678910111213"
var rawBuf = []byte("12345678910111213")

func BenchmarkStringToSliceByte_SliceByteToString(b *testing.B) {
	b.Run("string to byte slice", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = []byte(rawS)
			}
		})
	})

	b.Run("unsafe string to byte slice", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = StringToSliceByte(rawS)
			}
		})
	})

	b.Run("byte slice to string", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = string(rawBuf)
			}
		})
	})

	b.Run("unsafe byte slice to slice", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = SliceByteToString(rawBuf)
			}
		})
	})
}

func TestValuesToBytes(t *testing.T) {
	var err error
	longVal := int64(3)
	strVal := &person{"bob", 20}
	values := make([]interface{}, 0)

	values = append(values, err, true, longVal, strVal)
	buf := make([]byte, 0, 32)
	buf = ValuesToBytes(buf, values...)
	assert.Equal(t, buf[0], StringType)
	assert.Equal(t, buf[3], BoolType)
	assert.Equal(t, buf[5], LongType)
}

type person struct {
	name string
	age  int
}

func (p *person) String() string {
	return fmt.Sprintf("{name:%s, age:%d}", p.name, p.age)
}

func TestKeyValue_Encode(t *testing.T) {
	strVal := &person{"bob", 20}
	kv, err := NewKeyValue("person", strVal)
	assert.Nil(t, err)
	assert.Equal(t, kv.ValueType, StringType)
	key, value := kv.ToKV()
	assert.Equal(t, key, "person")
	assert.Equal(t, value, "{name:bob, age:20}")
	assert.Equal(t, kv.String(), "person={name:bob, age:20}")
}

type a struct {
	s string
}

func (this *a) String() string {
	return this.s
}

func TestNilKV(t *testing.T) {
	var strVal *a

	shortKV, err := NewKeyValue("a", strVal)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(shortKV.String(), "a="))
	shortKV.Recycle()

	shortKV2, err := NewKeyValue(strVal, strVal)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(shortKV2.String(), "nil"))
	shortKV2.Recycle()
}

type testObj struct {
	Field1 string
	Field2 int
	Filed3 float32
}

func TestSliceByteArray(t *testing.T) {
	b := []byte{1, 2, 3, 4, 5}
	value1, err := sliceByteArray(b, 1, 5)
	assert.Nil(t, err)
	assert.Equal(t, value1, b[1:5])

	_, err = sliceByteArray(b, -1, 4)
	assert.NotNil(t, err)
	_, err = sliceByteArray(b, 4, 1)
	assert.NotNil(t, err)
	_, err = sliceByteArray(b, 1, 6)
	assert.NotNil(t, err)
}

func TestDecodeValueWithType(t *testing.T) {
	data := []byte{Ipv4Type, 1, 2, 3, 4}
	_, _, err := decodeIpv4WithType(data, 0)
	assert.Nil(t, err)
	data = data[:4]
	_, _, err = decodeIpv4WithType(data, 0)
	assert.ErrorIs(t, err, ErrNoEnoughBytes)

	data = []byte{Ipv6Type, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3}
	_, _, err = decodeIpv6WithType(data, 0)
	assert.Nil(t, err)
	data = data[:16]
	_, _, err = decodeIpv6WithType(data, 0)
	assert.ErrorIs(t, err, ErrNoEnoughBytes)

	data = []byte{IntType, 0, 0, 0, 1}
	_, _, err = decodeUint32WithType(data, 0)
	assert.Nil(t, err)
	data = data[:4]
	_, _, err = decodeUint32WithType(data, 0)
	assert.ErrorIs(t, err, ErrNoEnoughBytes)

	data = []byte{Uint64Type, 0, 0, 0, 0, 0, 0, 0, 1}
	_, _, err = decodeUint64WithType(data, 0)
	assert.Nil(t, err)
	data = data[:8]
	_, _, err = decodeUint64WithType(data, 0)
	assert.ErrorIs(t, err, ErrNoEnoughBytes)

	data = []byte{DoubleType, 0, 0, 0, 0, 0, 0, 0, 1}
	_, _, err = decodeDoubleWithType(data, 0)
	assert.Nil(t, err)
	data = data[:8]
	_, _, err = decodeDoubleWithType(data, 0)
	assert.ErrorIs(t, err, ErrNoEnoughBytes)
}

func TestKVWithBytes(t *testing.T) {
	data := []byte{1, 2, 3, 5}
	kv, _ := NewKeyValue("a", data)
	assert.Equal(t, kv.ValueType, BytesType)

	buf := kv.Encode(nil)
	kv2, _ := NewKeyValue("a", string(data))
	buf2 := kv2.Encode(nil)
	assert.NotEqual(t, buf, buf2)
	assert.Equal(t, len(buf), len(buf2)+2)
	fmt.Println(kv.String())
}

func TestEncodedKV_NegativeInt(t *testing.T) {
	kv, _ := NewKeyValue("a", int32(-1))
	assert.Equal(t, kv.ValueType, IntType)
	assert.True(t, strings.Contains(kv.String(), "a=-1"))
	_, v := kv.ToKV()
	assert.Equal(t, "-1", v)

	kv2, _ := NewKeyValue("b", int64(-1))
	assert.Equal(t, kv2.ValueType, LongType)
	assert.True(t, strings.Contains(kv2.String(), "b=-1"))
	_, v = kv2.ToKV()
	assert.Equal(t, "-1", v)

	kv3, _ := NewKeyValue("c", -1)
	assert.Equal(t, kv3.ValueType, LongType)
	assert.True(t, strings.Contains(kv3.String(), "c=-1"))
	_, v = kv3.ToKV()
	assert.Equal(t, "-1", v)
}

func TestEncodeStrKV(t *testing.T) {
	kv := NewStrKeyValue("key", "value")
	assert.Equal(t, kv.ValueType, StringType)
	_, v := kv.ToKV()
	assert.Equal(t, "value", v)

	kv2 := NewStrKeyValue("key", "value", true)
	assert.Equal(t, kv2.ValueType, StringType)
	_, v2 := kv2.ToKV()
	assert.Equal(t, "value", v2)

	kv3 := NewStrKeyValue("key", randString(300), true)
	assert.Equal(t, kv3.ValueType, TextType)
	_, v3 := kv3.ToKV()
	assert.Equal(t, 300, len(v3))
}

func TestEncodeStrKVAppendToKV(t *testing.T) {
	kv := NewStrKeyValue("key", "value")
	assert.Equal(t, kv.ValueType, StringType)
	buffer := make([]byte, 0, 128)
	buffer = kv.EncodeAsStr(buffer)
	assert.Equal(t, kv.String(), string(buffer))

	kv2 := NewStrKeyValue("key", "value", true)
	assert.Equal(t, kv2.ValueType, StringType)
	buffer = buffer[:0]
	buffer = kv2.EncodeAsStr(buffer)
	assert.Equal(t, kv2.String(), string(buffer))

	kv3 := NewStrKeyValue("key", randString(300), true)
	assert.Equal(t, kv3.ValueType, TextType)
	buffer = buffer[:0]
	buffer = kv3.EncodeAsStr(buffer)
	assert.Equal(t, kv3.String(), string(buffer))
}

func BenchmarkEncodeKeyValueAsString(b *testing.B) {
	value := randString(1000)
	b.Run("String", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				kv := NewStrKeyValue("key", value)
				_ = kv.String()
				kv.Recycle()
			}
		})
	})

	b.Run("EncodeKV", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				p := NewPacket(0)
				kv := NewStrKeyValue("key", value)
				*p = kv.EncodeAsStr(*p)
				kv.Recycle()
				PutPacket(p)
			}
		})
	})

}

func Test_decodeValueWithUnknownType(t *testing.T) {
	type args struct {
		data   []byte
		offset int
	}
	tests := []struct {
		name           string
		args           args
		wantValue      interface{}
		wantReadLength int
		wantErr        assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
		{
			name: "ipv4",
			args: args{
				data:   []byte{Ipv4Type, 10, 0, 0, 0},
				offset: 0,
			},
			wantValue:      Ipv4(10),
			wantReadLength: 1 + 4,
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return true
			},
		},
		{
			name: "ipv6",
			args: args{
				data:   []byte{Ipv6Type, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3},
				offset: 0,
			},
			wantValue:      Ipv6{0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3},
			wantReadLength: 1 + 16,
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return true
			},
		},
		{
			name: "int",
			args: args{
				data:   []byte{IntType, 1, 0, 0, 0},
				offset: 0,
			},
			wantValue:      uint32(1),
			wantReadLength: 1 + 4,
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return true
			},
		},
		{
			name: "uint64",
			args: args{
				data:   []byte{Uint64Type, 1, 0, 0, 0, 0, 0, 0, 0},
				offset: 0,
			},
			wantValue:      uint64(1),
			wantReadLength: 1 + 8,
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return true
			},
		},
		{
			name: "double",
			args: args{
				data:   []byte{DoubleType, 10, 0, 0, 0, 0, 0, 0, 0},
				offset: 0,
			},
			wantValue:      math.Float64frombits(10),
			wantReadLength: 1 + 8,
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return true
			},
		},
		{
			name: "string",
			args: args{
				data:   []byte{StringType, 2, 'a', 'b', 0, 0, 0, 0, 0},
				offset: 0,
			},
			wantValue:      "ab",
			wantReadLength: 1 + 1 + 2 + 1,
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return true
			},
		},
		{
			name: "bool",
			args: args{
				data:   []byte{BoolType, 1},
				offset: 0,
			},
			wantValue:      true,
			wantReadLength: 1 + 1,
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return true
			},
		},
		{
			name: "text",
			args: args{
				data:   []byte{TextType, 4, 0, 0, 0, 't', 'e', 'x', 't', 0, 0, 0},
				offset: 0,
			},
			wantValue:      "text",
			wantReadLength: 1 + 4 + 4 + 1,
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return true
			},
		},
		{
			name: "bytes",
			args: args{
				data:   []byte{BytesType, 5, 0, 0, 0, 'b', 'y', 't', 'e', 's', 0, 0, 0},
				offset: 0,
			},
			wantValue:      []byte{'b', 'y', 't', 'e', 's'},
			wantReadLength: 1 + 4 + 5 + 0,
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return true
			},
		},
		{
			name: "unknown",
			args: args{
				data:   []byte{100, 0, 0, 0, 0},
				offset: 0,
			},
			wantValue:      "",
			wantReadLength: 0,
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return true
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotReadLength, err := decodeValueWithUnknownType(tt.args.data, tt.args.offset)
			if !tt.wantErr(t, err, fmt.Sprintf("decodeValueWithUnknownType(%v, %v)", tt.args.data, tt.args.offset)) {
				return
			}
			assert.Equalf(t, tt.wantValue, gotValue, "decodeValueWithUnknownType(%v, %v)", tt.args.data, tt.args.offset)
			assert.Equalf(t, tt.wantReadLength, gotReadLength, "decodeValueWithUnknownType(%v, %v)", tt.args.data, tt.args.offset)
		})
	}
}

type TestError struct {
	Message string
}

func NewTestError(message string) *TestError {
	return &TestError{Message: message}
}

func (e *TestError) Error() string {
	return e.Message
}
