// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"
)

func (s *Supervisor) agentLogFilePath() string {
	if s.commander != nil {
		if path := s.commander.LogFilePath(); path != "" {
			return path
		}
	}
	if s.config.Agent.PassthroughLogs {
		return ""
	}
	return filepath.Join(s.config.Storage.Directory, agentLogFileName)
}

func (s *Supervisor) collectorCrashLogSnippet() string {
	if s.config.Agent.CollectorCrashLogSnippetKiB <= 0 {
		return ""
	}
	return strings.ToValidUTF8(strings.TrimSpace(s.collectorLogTail()), "")
}

func (s *Supervisor) appendCollectorCrashDetails(msg string) string {
	snippet := s.collectorCrashLogSnippet()
	if snippet == "" {
		return msg
	}
	return fmt.Sprintf("%s\nCollector log tail:\n%s", msg, snippet)
}

func (s *Supervisor) collectorLogTail() string {
	if path := s.agentLogFilePath(); path != "" {
		data, err := readFileTail(path, int64(s.config.Agent.CollectorCrashLogSnippetKiB*1024))
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				s.telemetrySettings.Logger.Debug("Could not read collector log tail", zap.Error(err))
			}
			return ""
		}
		return string(data)
	}
	return s.passthroughLogTail()
}

func (s *Supervisor) passthroughLogTail() string {
	s.passthroughLogMu.Lock()
	buf := s.passthroughLogBuffer
	s.passthroughLogMu.Unlock()
	if buf == nil {
		return ""
	}
	return buf.Tail(s.config.Agent.CollectorCrashLogSnippetKiB * 1024)
}

func (s *Supervisor) appendPassthroughLogLine(line string) {
	if line == "" || s.config.Agent.CollectorCrashLogSnippetKiB <= 0 {
		return
	}
	buf := s.ensurePassthroughLogBuffer()
	buf.Append(line)
}

func (s *Supervisor) ensurePassthroughLogBuffer() *logRingBuffer {
	s.passthroughLogMu.Lock()
	defer s.passthroughLogMu.Unlock()
	if s.passthroughLogBuffer == nil {
		s.passthroughLogBuffer = newLogRingBuffer(s.config.Agent.CollectorCrashLogSnippetKiB * 1024)
	}
	return s.passthroughLogBuffer
}

func (s *Supervisor) resetPassthroughLogBuffer() {
	if !s.config.Agent.PassthroughLogs {
		return
	}

	s.passthroughLogMu.Lock()
	defer s.passthroughLogMu.Unlock()
	s.passthroughLogBuffer = nil
}

func readFileTail(path string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return []byte{}, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := info.Size()
	start := max(size-maxBytes, 0)

	if _, seekErr := f.Seek(start, io.SeekStart); seekErr != nil {
		return nil, seekErr
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	if start > 0 {
		if idx := bytes.IndexByte(data, '\n'); idx >= 0 && idx+1 < len(data) {
			data = data[idx+1:]
		}
	}

	return data, nil
}

type logRingBuffer struct {
	mu       sync.Mutex
	maxBytes int
	data     []byte
}

func newLogRingBuffer(maxBytes int) *logRingBuffer {
	if maxBytes <= 0 {
		maxBytes = 1
	}
	return &logRingBuffer{
		maxBytes: maxBytes,
	}
}

func (b *logRingBuffer) Append(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if line == "" {
		return
	}

	if len(b.data) > 0 {
		b.data = append(b.data, '\n')
	}
	b.data = append(b.data, line...)

	if len(b.data) > b.maxBytes {
		b.data = b.data[len(b.data)-b.maxBytes:]
	}
}

func (b *logRingBuffer) Tail(limit int) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.data) == 0 {
		return ""
	}
	if limit <= 0 || limit >= len(b.data) {
		return string(b.data)
	}
	return string(b.data[len(b.data)-limit:])
}
