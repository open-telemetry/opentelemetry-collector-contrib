// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func parseHexDump(name string) []byte {
	_, file, _, _ := runtime.Caller(0)
	dir, err := filepath.Abs(filepath.Dir(file))
	if err != nil {
		panic("Failed to find absolute path of hex dump: " + err.Error())
	}

	path := filepath.Join(dir, name+".hexdump")
	dump, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		panic("failed to read hex dump file " + path + ": " + err.Error())
	}

	var hexStr string
	for _, line := range strings.Split(string(dump), "\n") {
		if len(line) == 0 {
			continue
		}
		line = strings.Split(line, "|")[0]
		hexStr += strings.Join(strings.Fields(line)[1:], "")
	}

	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		panic("failed to parse hex bytes: " + err.Error())
	}

	return bytes
}
