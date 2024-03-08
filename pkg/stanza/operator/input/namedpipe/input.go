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

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

// Deprecated [v0.97.0] Use Factory.CreateOperator instead.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	set := component.TelemetrySettings{}
	if logger != nil {
		set.Logger = logger.Desugar()
	}
	return NewFactory().CreateOperator(&c, set)
}

// CreateOperator creates a new operator.
func (f *factory) CreateOperator(cfg component.Config, set component.TelemetrySettings) (operator.Operator, error) {
	c := cfg.(*Config)
	inputOperator, err := helper.NewInput(c.InputConfig, set)
	if err != nil {
		return nil, err
	}

	enc, err := decode.LookupEncoding(c.Encoding)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup encoding %q: %w", c.Encoding, err)
	}

	splitFunc, err := c.SplitConfig.Func(enc, true, DefaultMaxLogSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create split function: %w", err)
	}

	maxLogSize := c.MaxLogSize
	if maxLogSize == 0 {
		maxLogSize = DefaultMaxLogSize
	}

	return &Input{
		InputOperator: inputOperator,

		buffer:      make([]byte, maxLogSize),
		path:        c.Path,
		permissions: c.Permissions,
		splitFunc:   splitFunc,
		trimFunc:    c.TrimConfig.Func(),
	}, nil
}

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

func (n *Input) Start(_ operator.Persister) error {
	stat, err := os.Stat(n.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat named pipe: %w", err)
	}

	if !os.IsNotExist(err) && stat.Mode()&os.ModeNamedPipe == 0 {
		return fmt.Errorf("path %s is not a named pipe", n.path)
	}

	if os.IsNotExist(err) {
		if fifoErr := unix.Mkfifo(n.path, n.permissions); fifoErr != nil {
			return fmt.Errorf("failed to create named pipe: %w", fifoErr)
		}
	}

	// chmod the named pipe because mkfifo respects the umask which may result
	// in a named pipe with incorrect permissions.
	if chmodErr := os.Chmod(n.path, os.FileMode(n.permissions)); chmodErr != nil {
		return fmt.Errorf("failed to chmod named pipe: %w", chmodErr)
	}

	watcher, err := NewWatcher(n.path)
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	pipe, err := os.OpenFile(n.path, os.O_RDWR, os.ModeNamedPipe)
	if err != nil {
		return fmt.Errorf("failed to open named pipe: %w", err)
	}

	n.pipe = pipe

	ctx, cancel := context.WithCancel(context.Background())
	n.cancel = cancel

	n.wg.Add(2)
	go func() {
		defer n.wg.Done()
		if err := watcher.Watch(ctx); err != nil {
			n.Logger().Errorw("failed to watch named pipe", zap.Error(err))
		}
	}()

	go func() {
		defer n.wg.Done()
		for {
			select {
			case <-watcher.C:
				if err := n.process(ctx, pipe); err != nil {
					n.Logger().Errorw("failed to process named pipe", zap.Error(err))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (n *Input) Stop() error {
	if n.pipe != nil {
		n.pipe.Close()
	}

	if n.cancel != nil {
		n.cancel()
	}

	n.wg.Wait()
	return nil
}

func (n *Input) process(ctx context.Context, pipe *os.File) error {
	scan := bufio.NewScanner(pipe)
	scan.Split(n.splitFunc)
	scan.Buffer(n.buffer, len(n.buffer))

	for scan.Scan() {
		line := scan.Bytes()
		if len(line) == 0 {
			continue
		}

		if err := n.sendEntry(ctx, line); err != nil {
			return fmt.Errorf("failed to send entry: %w", err)
		}
	}

	return scan.Err()
}

// sendEntry sends an entry to the next operator in the pipeline.
func (n *Input) sendEntry(ctx context.Context, bytes []byte) error {
	bytes = n.trimFunc(bytes)
	if len(bytes) == 0 {
		return nil
	}

	entry, err := n.NewEntry(string(bytes))
	if err != nil {
		return fmt.Errorf("failed to create entry: %w", err)
	}

	n.Write(ctx, entry)
	return nil
}
