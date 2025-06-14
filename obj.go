package logs

type jsonBuf struct {
	buf []byte
}

func (b *jsonBuf) Write(p []byte) (n int, err error) {
	b.buf = append(b.buf, p...)
	return len(p), err
}
