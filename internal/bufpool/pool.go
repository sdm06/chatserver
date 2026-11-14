// Package bufpool recycles bufio readers and writers between connections.
//
// Allocating a fresh 4 KiB buffer for every connection is fine at low volume
// but wastes the garbage collector's time under load. sync.Pool lets us hand
// the same buffers back and forth instead.
package bufpool

import (
	"bufio"
	"io"
	"sync"
)

// discardReader is an io.Reader that is always empty. io.Discard is a Writer,
// so it cannot be used where a Reader is required.
type discardReader struct{}

func (discardReader) Read([]byte) (int, error) { return 0, io.EOF }

var emptyReader io.Reader = discardReader{}

var readers = sync.Pool{
	New: func() any { return bufio.NewReaderSize(emptyReader, 4096) },
}

var writers = sync.Pool{
	New: func() any { return bufio.NewWriterSize(io.Discard, 4096) },
}

// GetReader returns a pooled reader pointed at r.
func GetReader(r io.Reader) *bufio.Reader {
	br := readers.Get().(*bufio.Reader)
	br.Reset(r)
	return br
}

// PutReader returns a reader to the pool and drops its reference to the
// connection so the connection can be garbage collected.
func PutReader(br *bufio.Reader) {
	br.Reset(emptyReader)
	readers.Put(br)
}

// GetWriter returns a pooled writer pointed at w.
func GetWriter(w io.Writer) *bufio.Writer {
	bw := writers.Get().(*bufio.Writer)
	bw.Reset(w)
	return bw
}

// PutWriter returns a writer to the pool and drops its connection reference.
func PutWriter(bw *bufio.Writer) {
	bw.Reset(io.Discard)
	writers.Put(bw)
}
