package ldif

import (
	"bufio"
	"fmt"
	"io"
)

type RecordWriter interface {
	WriteRecord(record Record) error
}

type Writer struct {
	w *bufio.Writer
}

func NewWriter(w io.Writer, size int) *Writer {
	if size <= 0 {
		size = 1024 * 1024
	}
	return &Writer{w: bufio.NewWriterSize(w, size)}
}

func (w *Writer) WriteRecord(record Record) error {
	if record.DN == "" {
		return fmt.Errorf("record DN is empty")
	}
	if _, err := fmt.Fprintln(w.w, EncodeLine("dn", Value{Text: record.DN})); err != nil {
		return err
	}
	for _, attr := range record.Attributes {
		for _, value := range attr.Values {
			if _, err := fmt.Fprintln(w.w, EncodeLine(attr.Name, value)); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintln(w.w)
	return err
}

func (w *Writer) Flush() error {
	return w.w.Flush()
}
