package protocol

import (
	"bufio"
	"errors"
	"strings"
)

// BufferSize is the size of every pooled bufio.Reader. A frame can never be
// larger than this: anything longer makes ReadSlice return ErrBufferFull,
// which we turn into ErrFrameTooLarge. This bounds memory per connection.
const BufferSize = 4096

var (
	ErrFrameTooLarge = errors.New("protocol: frame exceeds maximum size")
	ErrEmptyFrame    = errors.New("protocol: empty frame")
)

// Command is one parsed client frame, e.g. "MSG hello there" becomes
// Name="MSG", Args=["hello","there"].
type Command struct {
	Name string
	Args []string
}

// ReadLine reads a single newline-terminated frame from r.
//
// The returned slice points into r's internal buffer and is only valid until
// the next read. Callers that need to keep the data must copy it (Parse does).
func ReadLine(r *bufio.Reader, max int) ([]byte, error) {
	line, err := r.ReadSlice('\n')
	if err != nil {
		if errors.Is(err, bufio.ErrBufferFull) {
			return nil, ErrFrameTooLarge
		}
		return nil, err
	}
	if len(line) > max {
		return nil, ErrFrameTooLarge
	}
	return line, nil
}

// Parse splits a raw frame into a command name and its arguments.
func Parse(line []byte) (Command, error) {
	s := strings.TrimRight(string(line), "\r\n")
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return Command{}, ErrEmptyFrame
	}
	return Command{Name: strings.ToUpper(fields[0]), Args: fields[1:]}, nil
}
