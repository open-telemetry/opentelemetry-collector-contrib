// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotreloadprocessor

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

type FileWatcher struct {
	watchPath  string
	logger     *zap.Logger
	dirWatcher *fsnotify.Watcher
	done       chan struct{}
	onChange   func(filePath string) error
}

func NewFileWatcher(
	logger *zap.Logger,
	watchPath string,
	onChange func(filePath string) error,
) (*FileWatcher, error) {
	return &FileWatcher{
		watchPath: watchPath,
		logger:    logger,
		done:      make(chan struct{}),
		onChange:  onChange,
	}, nil
}

func (p *FileWatcher) watchDir() error {
	info, err := os.Stat(p.watchPath)

	if os.IsNotExist(err) {
		return fmt.Errorf("dir doesn't exist: %s", p.watchPath)
	}

	if !info.IsDir() {
		return fmt.Errorf("file is not a dir: %s", p.watchPath)
	}

	p.logger.Info(
		"watching dir",
		zap.String("dir", p.watchPath),
	)
	err = p.dirWatcher.Add(p.watchPath)
	if err != nil {
		return err
	}

	return nil
}

func (p *FileWatcher) Start(ctx context.Context) error {
	var err error
	var debounceTimer *time.Timer
	debounceDuration := 1 * time.Second

	if p.dirWatcher, err = fsnotify.NewWatcher(); err != nil {
		return err
	}
	err = p.watchDir()
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.done:
				return
			case event, ok := <-p.dirWatcher.Events:
				if !ok {
					return
				}
				p.logger.Debug(
					"file changed",
					zap.String("file", event.Name),
					zap.String("event", event.String()),
				)
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(debounceDuration, func() {
					p.logger.Debug(
						"debounce timer expired",
						zap.String("file", event.Name),
						zap.String("event", event.String()),
					)
					err = p.onChange(event.Name)
					if err != nil {
						p.logger.Error(
							"error calling onChange",
							zap.String("event", event.String()),
							zap.Error(err),
						)
					}
				})
			case err, ok := <-p.dirWatcher.Errors:
				if !ok {
					return
				}
				p.logger.Error("error watching dir", zap.Error(err))
			}
		}
	}()

	return nil
}

func (p *FileWatcher) Stop(ctx context.Context) error {
	p.dirWatcher.Close()
	close(p.done)
	return nil
}
