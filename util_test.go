package logs

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecMark(t *testing.T) {
	key := "value"
	value := 100
	res := SecMark(key, value)

	if enableSecMark {
		assert.Equal(t, "{{value=100}}", res)
	} else {
		assert.Equal(t, "value=100", res)
	}
}

func TestSecMarkStructure(t *testing.T) {
	key := person{"Bob", "0001", 20}
	value := 100
	res := SecMark(key, value)

	if enableSecMark {
		assert.Equal(t, "{{{\"Name\":\"Bob\",\"Id\":\"0001\",\"Age\":20}=100}}", res)
	} else {
		assert.Equal(t, "{\"Name\":\"Bob\",\"Id\":\"0001\",\"Age\":20}=100", res)
	}
}

func TestSecMarkNonnumericValue(t *testing.T) {
	key := person{"Bob", "0001", 20}
	value := "StringValue"
	res := SecMark(key, value)

	if enableSecMark {
		assert.Equal(t, "{{{\"Name\":\"Bob\",\"Id\":\"0001\",\"Age\":20}=StringValue}}", res)
	} else {
		assert.Equal(t, "{\"Name\":\"Bob\",\"Id\":\"0001\",\"Age\":20}=StringValue", res)
	}
}

var bytesSlice = []byte("123456                          ")

func BenchmarkTrimPadding(b *testing.B) {
	for i := 0; i < b.N; i++ {
		trimPadding(&bytesSlice, []byte(""), true)
	}
}

type person struct {
	Name string
	Id   string
	Age  int
}

type human struct {
	name string
	age  int
}

type foobar struct {
	Bar string
	Baz int
}

func (f foobar) String() string {
	return fmt.Sprintf("{\"Bar\":\"%s\",\"Baz\":%d}", f.Bar, f.Baz)
}

type spStringifier struct {
	Val interface{}
}

func (js spStringifier) String() string {
	return "spStringifier_String"
}

func NewSPStringifier(val interface{}) spStringifier {
	return spStringifier{Val: val}
}

func TestValueToStr(t *testing.T) {
	a := foobar{"Bar", 666}
	expected_a_str := "{\"Bar\":\"Bar\",\"Baz\":666}"
	expected_b_str := "spStringifier_String"
	a_str := ValueToStr(a)
	b := NewSPStringifier(a)
	b_str := ValueToStr(b)
	assert.Equal(t, expected_a_str, a_str)
	assert.Equal(t, expected_b_str, b_str)

	var c []byte
	for i := 0; i < 100; i++ {
		c = append(c, strconv.Itoa(i)...)
	}
	c_str := ValueToStr(c)
	fmt.Println(c_str)

	var err error
	nil_err_str := ValueToStr(err)
	fmt.Println(nil_err_str)
}
