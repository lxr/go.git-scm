// These interfaces and functions provide a convenient interface for
// scanning from and printing to line-based readers and writers.

package protocol

import (
	"fmt"
)

type lineReader interface {
	ReadLine() (string, error)
}

type lineWriter interface {
	WriteLine(string) error
}

func fmtLscanf(lr lineReader, format string, a ...interface{}) (int, error) {
	s, err := lr.ReadLine()
	if err != nil {
		return 0, err
	}
	return fmt.Sscanf(s, format, a...)
}

func fmtLprintf(lw lineWriter, format string, a ...interface{}) error {
	s := fmt.Sprintf(format, a...)
	return lw.WriteLine(s)
}
