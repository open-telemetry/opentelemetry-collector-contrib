// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver/internal/testutil"

import (
	"math/rand/v2"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type LogFileGenerator struct {
	tb       testing.TB
	charset  []byte
	logLines [][]byte
}

func NewLogFileGenerator(tb testing.TB) *LogFileGenerator {
	logFileGenerator := &LogFileGenerator{
		tb:      tb,
		charset: []byte("abcdefghijklmnopqrstuvwxyz"),
	}
	logFileGenerator.logLines = logFileGenerator.generateLogLines(8, 999)
	return logFileGenerator
}

func (g *LogFileGenerator) generateLogLines(numLines, lineLength int) (logLines [][]byte) {
	logLines = make([][]byte, numLines)
	for i := range numLines {
		logLines[i] = make([]byte, lineLength)
		for j := range lineLength {
			logLines[i][j] = g.charset[rand.IntN(len(g.charset))]
		}
	}
	return logLines
}

func (g *LogFileGenerator) GenerateLogFile(numLines int) (logFilePath string) {
	f, err := os.CreateTemp(g.tb.TempDir(), "")
	require.NoError(g.tb, err)
	g.tb.Cleanup(func() { _ = f.Close() })
	for range numLines {
		_, err := f.Write(g.logLines[rand.IntN(len(g.logLines))])
		require.NoError(g.tb, err)
		_, err = f.WriteString("\n")
		require.NoError(g.tb, err)
	}
	return f.Name()
}
