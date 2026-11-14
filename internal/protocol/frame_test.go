package protocol

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestReadLine(t *testing.T) {
	r := bufio.NewReaderSize(strings.NewReader("hello\nworld\n"), BufferSize)
	line, err := ReadLine(r, BufferSize)
	if err != nil {
		t.Fatal(err)
	}
	if string(line) != "hello\n" {
		t.Fatalf("got %q", line)
	}
	line, err = ReadLine(r, BufferSize)
	if err != nil {
		t.Fatal(err)
	}
	if string(line) != "world\n" {
		t.Fatalf("got %q", line)
	}
}

func TestReadLineTooLarge(t *testing.T) {
	long := strings.Repeat("a", BufferSize) + "\n"
	r := bufio.NewReaderSize(strings.NewReader(long), BufferSize)
	if _, err := ReadLine(r, BufferSize); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("got %v, want ErrFrameTooLarge", err)
	}
}

func TestReadLineOverConfiguredMax(t *testing.T) {
	r := bufio.NewReaderSize(strings.NewReader("0123456789\n"), BufferSize)
	if _, err := ReadLine(r, 4); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("got %v, want ErrFrameTooLarge", err)
	}
}

func TestParse(t *testing.T) {
	cmd, err := Parse([]byte("msg hello   world\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name != "MSG" {
		t.Fatalf("name = %q", cmd.Name)
	}
	if len(cmd.Args) != 2 || cmd.Args[0] != "hello" || cmd.Args[1] != "world" {
		t.Fatalf("args = %#v", cmd.Args)
	}
}

func TestParseEmpty(t *testing.T) {
	if _, err := Parse([]byte("\r\n")); !errors.Is(err, ErrEmptyFrame) {
		t.Fatalf("got %v, want ErrEmptyFrame", err)
	}
}

func BenchmarkReadLine(b *testing.B) {
	line := []byte("MSG hello world, this is a chat message\n")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r := bufio.NewReaderSize(bytes.NewReader(line), BufferSize)
		if _, err := ReadLine(r, BufferSize); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParse(b *testing.B) {
	line := []byte("MSG hello world, this is a chat message\n")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Parse(line); err != nil {
			b.Fatal(err)
		}
	}
}
