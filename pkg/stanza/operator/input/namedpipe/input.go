// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package namedpipe // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/namedpipe"

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

const ReadTimeout = 2 * time.Second

type Input struct {
	helper.InputOperator

	buffer      []byte
	path        string
	permissions uint32
	splitFunc   bufio.SplitFunc
	trimFunc    trim.Func
	cancel      context.CancelFunc
	pipe        *os.File
	wg          sync.WaitGroup
}

func (i *Input) Start(_ operator.Persister) error {
	stat, err := os.Stat(i.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat named pipe: %w", err)
	}

	if !os.IsNotExist(err) && stat.Mode()&os.ModeNamedPipe == 0 {
		return fmt.Errorf("path %s is not a named pipe", i.path)
	}

	if os.IsNotExist(err) {
		if fifoErr := unix.Mkfifo(i.path, i.permissions); fifoErr != nil {
			return fmt.Errorf("failed to create named pipe: %w", fifoErr)
		}
	}

	// chmod the named pipe because mkfifo respects the umask which may result
	// in a named pipe with incorrect permissions.
	if chmodErr := os.Chmod(i.path, os.FileMode(i.permissions)); chmodErr != nil {
		return fmt.Errorf("failed to chmod named pipe: %w", chmodErr)
	}

	// Open the pipe with O_RDWR so it won't block on opening the pipe when there is no writer.
	// The current process is both a writer and reader, which prevents the read from receiving
	// EOF because there is always a writer (the process itself) connects to the pipe.
	pipe, err := os.OpenFile(i.path, os.O_RDWR, os.ModeNamedPipe)
	if err != nil {
		return fmt.Errorf("failed to open named pipe: %w", err)
	}
	i.pipe = pipe

	ctx, cancel := context.WithCancel(context.Background())
	i.cancel = cancel

	i.wg.Add(1)
	go i.readLoop(ctx)

	return nil
}

func (i *Input) Stop() error {
	if i.cancel != nil {
		i.cancel()
	}

	if i.pipe != nil {
		i.pipe.Close()
	}

	i.wg.Wait()
	return nil
}

func (i *Input) readLoop(ctx context.Context) {
	defer i.wg.Done()
	pipe := i.pipe

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := i.process(ctx, pipe); err != nil {
				i.Logger().Error("error processing named pipe", zap.Error(err))
			}

			// The process exits due to whatever reason, wait for ReadTimeout and try again.
			select {
			case <-ctx.Done():
				return
			case <-time.After(ReadTimeout):
				i.Logger().Warn("processing named pipe is interrupted, retrying the process now", zap.String("path", i.path))
			}
		}
	}
}

func (i *Input) process(ctx context.Context, pipe *os.File) error {
	scan := bufio.NewScanner(pipe)
	scan.Split(i.splitFunc)
	scan.Buffer(i.buffer, len(i.buffer))

	for scan.Scan() {
		line := scan.Bytes()
		if len(line) == 0 {
			continue
		}

		if err := i.sendEntry(ctx, line); err != nil {
			return fmt.Errorf("failed to send entry: %w", err)
		}
	}

	return scan.Err()
}

// sendEntry sends an entry to the next operator in the pipeline.
func (i *Input) sendEntry(ctx context.Context, bytes []byte) error {
	bytes = i.trimFunc(bytes)
	if len(bytes) == 0 {
		return nil
	}

	entry, err := i.NewEntry(string(bytes))
	if err != nil {
		return fmt.Errorf("failed to create entry: %w", err)
	}

	return i.Write(ctx, entry)
}
