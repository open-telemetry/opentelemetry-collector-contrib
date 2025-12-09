// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"bytes"
	"encoding/json"
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
		if s.config.Agent.PassthroughLogs {
			return ""
		}
	}
	if s.config.Agent.PassthroughLogs {
		return ""
	}
	return filepath.Join(s.config.Storage.Directory, agentLogFileName)
}

func (s *Supervisor) collectorCrashLogSnippet() string {
	raw := strings.TrimSpace(s.collectorLogTail())
	if raw == "" {
		return ""
	}

	if formatted := formatCollectorLogSnippet(raw); formatted != "" {
		return formatted
	}

	return raw
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
		data, err := readFileTail(path, int64(collectorCrashLogSnippetBytes))
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
	return buf.Tail(int(collectorCrashLogSnippetBytes))
}

func (s *Supervisor) appendPassthroughLogLine(line string) {
	if line == "" {
		return
	}
	buf := s.ensurePassthroughLogBuffer()
	buf.Append(line)
}

func (s *Supervisor) ensurePassthroughLogBuffer() *logRingBuffer {
	s.passthroughLogMu.Lock()
	defer s.passthroughLogMu.Unlock()
	if s.passthroughLogBuffer == nil {
		s.passthroughLogBuffer = newLogRingBuffer(int(collectorCrashLogSnippetBytes))
	}
	return s.passthroughLogBuffer
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
	start := size - maxBytes
	if start < 0 {
		start = 0
	}

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

func formatCollectorLogSnippet(snippet string) string {
	lines := splitLines(snippet)

	stackStartIdx, stack := captureStackTrace(lines)
	structuredLines := extractStructuredErrors(lines[:stackStartIdx])

	var sections []string
	if len(structuredLines) > 0 {
		sections = append(sections, strings.Join(structuredLines, "\n"))
	}
	if len(stack) > 0 {
		sections = append(sections, strings.Join(stack, "\n"))
	}

	return strings.Join(sections, "\n\n")
}

func splitLines(snippet string) []string {
	rawLines := strings.Split(snippet, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		lines = append(lines, strings.TrimRight(line, "\r"))
	}
	return lines
}

func captureStackTrace(lines []string) (int, []string) {
	for idx, line := range lines {
		if isStackTraceStart(line) {
			stack := strings.TrimSpace(strings.Join(lines[idx:], "\n"))
			if stack != "" {
				return idx, strings.Split(stack, "\n")
			}
			return idx, nil
		}
	}
	return len(lines), nil
}

func isStackTraceStart(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(lower, "panic:") ||
		strings.HasPrefix(lower, "fatal error:") ||
		strings.Contains(lower, "sigsegv") ||
		strings.HasPrefix(lower, "goroutine ") ||
		strings.Contains(lower, "runtime stack")
}

func extractStructuredErrors(lines []string) []string {
	var jsonLines []string
	var kvLines []string
	var plainLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "{") && strings.Contains(trimmed, "}"):
			if summary := parseJSONErrorLine(trimmed); summary != "" {
				jsonLines = append(jsonLines, summary)
				continue
			}
		case strings.Contains(trimmed, "="):
			if summary := parseKeyValueErrorLine(trimmed); summary != "" {
				kvLines = append(kvLines, summary)
				continue
			}
		}

		if isPlainErrorLine(trimmed) {
			plainLines = append(plainLines, trimmed)
		}
	}

	switch {
	case len(jsonLines) > 0:
		return jsonLines
	case len(kvLines) > 0:
		return kvLines
	default:
		return plainLines
	}
}

func parseJSONErrorLine(line string) string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		return ""
	}

	level := strings.ToLower(getString(payload, "level"))
	if level == "" {
		level = strings.ToLower(getString(payload, "severity"))
	}
	if level == "" || !isErrorLevel(level) {
		return ""
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("level=%s", level))

	if component := getString(payload, "component", "name", "logger"); component != "" {
		parts = append(parts, fmt.Sprintf("component=%s", component))
	}

	if msg := getString(payload, "msg", "message"); msg != "" {
		parts = append(parts, fmt.Sprintf("msg=%s", msg))
	}

	if errVal := getString(payload, "error", "err"); errVal != "" {
		parts = append(parts, fmt.Sprintf("error=%s", errVal))
	}

	extra := strings.TrimSpace(getString(payload, "stacktrace"))
	if extra != "" {
		parts = append(parts, fmt.Sprintf("stacktrace=%s", extra))
	}

	return strings.Join(parts, " ")
}

func getString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if val, ok := payload[key]; ok {
			switch typed := val.(type) {
			case string:
				return typed
			case fmt.Stringer:
				return typed.String()
			case nil:
				continue
			default:
				return fmt.Sprint(typed)
			}
		}
	}
	return ""
}

func parseKeyValueErrorLine(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}

	kv := map[string]string{}
	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(parts[0])
		value := strings.Trim(parts[1], `"`)
		kv[key] = value
	}

	level := kv["level"]
	if level == "" {
		level = kv["severity"]
	}
	if level == "" || !isErrorLevel(level) {
		return ""
	}

	return line
}

func isPlainErrorLine(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "error") ||
		strings.Contains(lower, "level=err") ||
		strings.Contains(lower, "severity=err") ||
		strings.Contains(lower, "fatal")
}

func isErrorLevel(level string) bool {
	switch strings.ToLower(level) {
	case "error", "err", "fatal", "panic":
		return true
	default:
		return false
	}
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
