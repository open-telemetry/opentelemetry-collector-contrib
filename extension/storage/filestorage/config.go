// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"
)

// Config defines configuration for file storage extension.
type Config struct {
	Directory string        `mapstructure:"directory,omitempty"`
	Timeout   time.Duration `mapstructure:"timeout,omitempty"`

	Compaction *CompactionConfig `mapstructure:"compaction,omitempty"`

	// FSync specifies that fsync should be called after each database write
	FSync bool `mapstructure:"fsync,omitempty"`
}

// CompactionConfig defines configuration for optional file storage compaction.
type CompactionConfig struct {
	// OnStart specifies that compaction is attempted each time on start
	OnStart bool `mapstructure:"on_start,omitempty"`
	// OnRebound specifies that compaction is attempted online, when rebound conditions are met.
	// This typically happens when storage usage has increased, which caused increase in space allocation
	// and afterwards it had most items removed. We want to run the compaction online only when there are
	// not too many elements still being stored (which is an indication that "heavy usage" period is over)
	// so compaction should be relatively fast and at the same time there is relatively large volume of space
	// that might be reclaimed.
	OnRebound bool `mapstructure:"on_rebound,omitempty"`
	// Directory specifies where the temporary files for compaction will be stored
	Directory string `mapstructure:"directory,omitempty"`
	// ReboundNeededThresholdMiB specifies the minimum total allocated size (both used and empty)
	// to mark the need for online compaction
	ReboundNeededThresholdMiB int64 `mapstructure:"rebound_needed_threshold_mib"`
	// ReboundTriggerThresholdMiB is used when compaction is marked as needed. When allocated data size drops
	// below the specified value, the compactions starts and the flag marking need for compaction is cleared
	ReboundTriggerThresholdMiB int64 `mapstructure:"rebound_trigger_threshold_mib"`
	// MaxTransactionSize specifies the maximum number of items that might be present in single compaction iteration
	MaxTransactionSize int64 `mapstructure:"max_transaction_size,omitempty"`
	// CheckInterval specifies frequency of compaction check
	CheckInterval time.Duration `mapstructure:"check_interval,omitempty"`
}

func (cfg *Config) Validate() error {
	var dirs []string
	if cfg.Compaction.OnStart {
		dirs = []string{cfg.Directory, cfg.Compaction.Directory}
	} else {
		dirs = []string{cfg.Directory}
	}
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("directory must exist: %w", err)
			}

			fsErr := &fs.PathError{}
			if errors.As(err, &fsErr) {
				return fmt.Errorf("problem accessing configured directory: %s, err: %w", dir, fsErr)
			}
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", dir)
		}
	}

	if cfg.Compaction.MaxTransactionSize < 0 {
		return errors.New("max transaction size for compaction cannot be less than 0")
	}

	if cfg.Compaction.OnRebound && cfg.Compaction.CheckInterval <= 0 {
		return errors.New("compaction check interval must be positive when rebound compaction is set")
	}

	return nil
}
