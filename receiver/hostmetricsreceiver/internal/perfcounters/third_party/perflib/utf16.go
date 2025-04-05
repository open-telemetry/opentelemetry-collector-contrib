//go:build windows

package perflib // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/perfcounters/third_party/perflib"

import (
	"encoding/binary"
	"io"

	"golang.org/x/sys/windows"
)

// readUTF16StringAtPos Read an unterminated UTF16 string at a given position, specifying its length.
func readUTF16StringAtPos(r io.ReadSeeker, absPos int64, length uint32) (string, error) {
	value := make([]uint16, length/2)

	_, err := r.Seek(absPos, io.SeekStart)
	if err != nil {
		return "", err
	}

	err = binary.Read(r, bo, value)
	if err != nil {
		return "", err
	}

	return windows.UTF16ToString(value), nil
}

// readUTF16String Reads a null-terminated UTF16 string at the current offset.
func readUTF16String(r io.Reader) (string, error) {
	var err error

	b := make([]byte, 2)
	out := make([]uint16, 0, 100)

	for i := 0; err == nil; i += 2 {
		_, err = r.Read(b)

		if b[0] == 0 && b[1] == 0 {
			break
		}

		out = append(out, bo.Uint16(b))
	}

	if err != nil {
		return "", err
	}

	return windows.UTF16ToString(out), nil
}
