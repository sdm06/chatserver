package bufpool

import (
	"io"
	"strings"
	"testing"
)

func TestReaderRoundTrip(t *testing.T) {
	r := GetReader(strings.NewReader("hello\n"))
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if line != "hello\n" {
		t.Fatalf("got %q", line)
	}
	PutReader(r)
}

func TestWriterRoundTrip(t *testing.T) {
	var sb strings.Builder
	w := GetWriter(&sb)
	if _, err := w.WriteString("hi"); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	PutWriter(w)
	if sb.String() != "hi" {
		t.Fatalf("got %q", sb.String())
	}
}

func BenchmarkGetPutReader(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		PutReader(GetReader(emptyReader))
	}
}

func BenchmarkGetPutWriter(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		PutWriter(GetWriter(io.Discard))
	}
}
