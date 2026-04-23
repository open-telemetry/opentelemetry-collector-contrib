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
	"sync"
	"time"

	gojson "github.com/goccy/go-json"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Input is an operator that process logs using journald
type Input struct {
	helper.InputOperator

	newCmd func(ctx context.Context, cursor []byte) cmd

	persister           operator.Persister
	convertMessageBytes bool
	cancel              context.CancelFunc
	wg                  sync.WaitGroup
	errChan             chan error
}

type cmd interface {
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
	Start() error
	Wait() error
}

type journalctl struct {
	cmd    cmd
	stdout io.ReadCloser
	stderr io.ReadCloser
}

var lastReadCursorKey = "lastReadCursor"

// Start will start generating log entries.
func (operator *Input) Start(persister operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	operator.cancel = cancel

	operator.persister = persister
	operator.errChan = make(chan error)

	go operator.run(ctx)

	select {
	case err := <-operator.errChan:
		return fmt.Errorf("journalctl command failed: %w", err)
	case <-time.After(waitDuration):
		return nil
	}
}

// run starts the journalctl process and monitor it.
// If there is an error in operator.newJournalctl, the error will be sent to operator.errChan.
// If the journalctl process started successfully, but there is an error in operator.runJournalctl,
// The error will be logged and the journalctl process will be restarted.
func (operator *Input) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			jctl, err := operator.newJournalctl(ctx)
			// If we can't start journalctl, there is nothing we can do but logging the error and return.
			if err != nil {
				select {
				case operator.errChan <- err:
				case <-time.After(waitDuration):
					operator.Logger().Error("Failed to init and start journalctl", zap.Error(err))
				}
				return
			}

			operator.Logger().Debug("Starting the journalctl command")
			if err := operator.runJournalctl(ctx, jctl); err != nil {
				ee := &exec.ExitError{}
				if ok := errors.As(err, &ee); ok && ee.ExitCode() != 0 {
					operator.Logger().Error("journalctl command exited", zap.Error(ee))
				} else {
					operator.Logger().Info("journalctl command exited")
				}
			}
			// Backoff before restart.
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}
}

// newJournalctl creates a new journalctl command.
func (operator *Input) newJournalctl(ctx context.Context) (*journalctl, error) {
	// Start from a cursor if there is a saved offset
	cursor, err := operator.persister.Get(ctx, lastReadCursorKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get journalctl state: %w", err)
	}

	journal := operator.newCmd(ctx, cursor)
	jctl := &journalctl{
		cmd: journal,
	}

	jctl.stdout, err = journal.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get journalctl stdout: %w", err)
	}
	jctl.stderr, err = journal.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get journalctl stderr: %w", err)
	}

	if err = journal.Start(); err != nil {
		return nil, fmt.Errorf("start journalctl: %w", err)
	}

	return jctl, nil
}

// runJournalctl runs the journalctl command. This is a blocking call that returns
// when the command exits.
func (operator *Input) runJournalctl(ctx context.Context, jctl *journalctl) error {
	// Start the stderr reader goroutine.
	// This goroutine reads the stderr from the journalctl process. If the
	// process exits for any reason, then the stderr will be closed, this
	// goroutine will get an EOF error and exit.
	operator.wg.Go(func() {
		stderrBuf := bufio.NewReader(jctl.stderr)

		for {
			line, err := stderrBuf.ReadBytes('\n')
			if err != nil {
				if !errors.Is(err, io.EOF) {
					operator.Logger().Error("Received error reading from journalctl stderr", zap.Error(err))
				}
				return
			}
			operator.Logger().Error("Received from journalctl stderr", zap.ByteString("stderr", line))
		}
	})

	// Start the reader goroutine.
	// This goroutine reads the stdout from the journalctl process, parses
	// the data, and writes to output. If the journalctl process exits for
	// any reason, then the stdout will be closed, this goroutine will get
	// an EOF error and exits.
	operator.wg.Go(func() {
		stdoutBuf := bufio.NewReader(jctl.stdout)

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
			if err = operator.persister.Set(ctx, lastReadCursorKey, []byte(cursor)); err != nil {
				operator.Logger().Warn("Failed to set offset", zap.Error(err))
			}
			if err = operator.Write(ctx, entry); err != nil {
				operator.Logger().Error("failed to write entry", zap.Error(err))
			}
		}
	})

	// we wait for the reader goroutines to exit before calling Cmd.Wait().
	// As per documentation states, "It is thus incorrect to call Wait before all reads from the pipe have completed".
	operator.wg.Wait()

	return jctl.cmd.Wait()
}

func (operator *Input) parseJournalEntry(line []byte) (*entry.Entry, string, error) {
	var body map[string]any
	err := gojson.Unmarshal(line, &body)
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

	if operator.convertMessageBytes {
		// Convert the message bytes to string if given as a byte array
		msgArr, msgArrayOk := body["MESSAGE"].([]any)
		if msgArrayOk {
			var bytes []byte
			for _, val := range msgArr {
				floatVal, floatCheckOk := val.(float64)
				if floatCheckOk {
					bytes = append(bytes, byte(int(floatVal)))
				}
			}
			body["MESSAGE"] = string(bytes)
		}
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
	return nil
}
