// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"io"
	"regexp"

	"go.uber.org/zap"
)

type LogWriterCloser struct {
	log *zap.Logger
}

func NewLogWriterCloser(log *zap.Logger) *LogWriterCloser {
	return &LogWriterCloser{log: log}
}

func (lwc *LogWriterCloser) Write(p []byte) (n int, err error) {
	lwc.log.Info(string(Scrub(p)))
	return len(p), nil
}

func (lwc *LogWriterCloser) Close() error {
	return nil
}

type LogProvider struct {
	log *zap.Logger
}

func NewLogProvider(log *zap.Logger) *LogProvider {
	return &LogProvider{log: log}
}

func (s *LogProvider) NewFile(p string) io.WriteCloser {
	return NewLogWriterCloser(s.log.Named(p))
}

func (s *LogProvider) Flush() {
}

var scrubPassword = regexp.MustCompile(`<password>(.*)</password>`)

func Scrub(in []byte) []byte {
	return scrubPassword.ReplaceAll(in, []byte(`<password>********</password>`))
}
