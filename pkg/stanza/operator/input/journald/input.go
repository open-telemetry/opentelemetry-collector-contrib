// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package journald // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Input is an operator that process logs using journald
type Input struct {
	helper.InputOperator

	newCmd func(ctx context.Context, cursor []byte) cmd

	persister operator.Persister
	json      jsoniter.API
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

type cmd interface {
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
	Start() error
	Wait() error
}

type failedCommand struct {
	err    string
	output string
}

var lastReadCursorKey = "lastReadCursor"

// Start will start generating log entries.
func (operator *Input) Start(persister operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	operator.cancel = cancel

	// Start from a cursor if there is a saved offset
	cursor, err := persister.Get(ctx, lastReadCursorKey)
	if err != nil {
		return fmt.Errorf("failed to get journalctl state: %w", err)
	}

	operator.persister = persister

	// Start journalctl
	journal := operator.newCmd(ctx, cursor)
	stdout, err := journal.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get journalctl stdout: %w", err)
	}
	stderr, err := journal.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get journalctl stderr: %w", err)
	}
	err = journal.Start()
	if err != nil {
		return fmt.Errorf("start journalctl: %w", err)
	}

	stderrChan := make(chan string)
	failedChan := make(chan failedCommand)

	// Start the wait goroutine
	operator.wg.Add(1)
	go func() {
		defer operator.wg.Done()
		err := journal.Wait()
		message := <-stderrChan

		f := failedCommand{
			output: message,
		}

		if err != nil {
			ee := (&exec.ExitError{})
			if ok := errors.As(err, &ee); ok && ee.ExitCode() != 0 {
				f.err = ee.Error()
			}
		}

		select {
		case failedChan <- f:
		// log an error in case channel is closed
		case <-time.After(waitDuration):
			operator.Logger().Error("journalctl command exited", zap.String("error", f.err), zap.String("output", f.output))
		}
	}()

	// Start the stderr reader goroutine
	operator.wg.Add(1)
	go func() {
		defer operator.wg.Done()

		stderrBuf := bufio.NewReader(stderr)
		messages := []string{}

		for {
			line, err := stderrBuf.ReadBytes('\n')
			if err != nil {
				if !errors.Is(err, io.EOF) {
					operator.Logger().Error("Received error reading from journalctl stderr", zap.Error(err))
				}
				stderrChan <- strings.Join(messages, "\n")
				return
			}
			messages = append(messages, string(line))
		}
	}()

	// Start the reader goroutine
	operator.wg.Add(1)
	go func() {
		defer operator.wg.Done()

		stdoutBuf := bufio.NewReader(stdout)

		for {
			line, err := stdoutBuf.ReadBytes('\n')
			if err != nil {
				if !errors.Is(err, io.EOF) {
					operator.Logger().Error("Received error reading from journalctl stdout", zap.Error(err))
				}
				return
			}

			entry, cursor, err := operator.parseJournalEntry(line)
			if err != nil {
				operator.Logger().Warn("Failed to parse journal entry", zap.Error(err))
				continue
			}
			if err := operator.persister.Set(ctx, lastReadCursorKey, []byte(cursor)); err != nil {
				operator.Logger().Warn("Failed to set offset", zap.Error(err))
			}
			operator.Write(ctx, entry)
		}
	}()

	// Wait waitDuration for eventual error
	select {
	case err := <-failedChan:
		if err.err == "" {
			return fmt.Errorf("journalctl command exited")
		}
		return fmt.Errorf("journalctl command failed (%v): %v", err.err, err.output)
	case <-time.After(waitDuration):
		return nil
	}
}

func (operator *Input) parseJournalEntry(line []byte) (*entry.Entry, string, error) {
	var body map[string]any
	err := operator.json.Unmarshal(line, &body)
	if err != nil {
		return nil, "", err
	}

	timestamp, ok := body["__REALTIME_TIMESTAMP"]
	if !ok {
		return nil, "", errors.New("journald body missing __REALTIME_TIMESTAMP field")
	}

	timestampString, ok := timestamp.(string)
	if !ok {
		return nil, "", errors.New("journald field for timestamp is not a string")
	}

	timestampInt, err := strconv.ParseInt(timestampString, 10, 64)
	if err != nil {
		return nil, "", fmt.Errorf("parse timestamp: %w", err)
	}

	delete(body, "__REALTIME_TIMESTAMP")

	cursor, ok := body["__CURSOR"]
	if !ok {
		return nil, "", errors.New("journald body missing __CURSOR field")
	}

	cursorString, ok := cursor.(string)
	if !ok {
		return nil, "", errors.New("journald field for cursor is not a string")
	}

	entry, err := operator.NewEntry(body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create entry: %w", err)
	}

	entry.Timestamp = time.Unix(0, timestampInt*1000) // in microseconds
	return entry, cursorString, nil
}

// Stop will stop generating logs.
func (operator *Input) Stop() error {
	if operator.cancel != nil {
		operator.cancel()
	}
	operator.wg.Wait()
	return nil
}
