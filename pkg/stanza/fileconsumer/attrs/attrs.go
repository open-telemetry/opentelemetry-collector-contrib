// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attrs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"syscall"
)

const (
	LogFileName         = "log.file.name"
	LogFilePath         = "log.file.path"
	LogFileNameResolved = "log.file.name_resolved"
	LogFilePathResolved = "log.file.path_resolved"
	LogFileOwnerName    = "log.file.owner.name"
	LogFileGroupName    = "log.file.group.name"
)

type Resolver struct {
	IncludeFileName         bool `mapstructure:"include_file_name,omitempty"`
	IncludeFilePath         bool `mapstructure:"include_file_path,omitempty"`
	IncludeFileNameResolved bool `mapstructure:"include_file_name_resolved,omitempty"`
	IncludeFilePathResolved bool `mapstructure:"include_file_path_resolved,omitempty"`
	IncludeFileOwnerName    bool `mapstructure:"include_file_owner_name,omitempty"`
	IncludeFileGroupName    bool `mapstructure:"include_file_group_name,omitempty"`
}

func (r *Resolver) AddFileInfos(file *os.File, attributes map[string]any) (err error) {
	var fileInfo, errStat = file.Stat()
	if errStat != nil {
		return fmt.Errorf("resolve file stat: %w", err)
	}
	var fileStat = fileInfo.Sys().(*syscall.Stat_t)

	if r.IncludeFileOwnerName {
		var fileOwner, errFileUser = user.LookupId(fmt.Sprint(fileStat.Uid))
		if errFileUser != nil {
			return fmt.Errorf("resolve file owner name: %w", errFileUser)
		}
		attributes[LogFileOwnerName] = fileOwner.Username
	}
	if r.IncludeFileGroupName {
		var fileGroup, errFileGroup = user.LookupGroupId(fmt.Sprint(fileStat.Gid))
		if errFileGroup != nil {
			return fmt.Errorf("resolve file group name: %w", errFileGroup)
		}
		attributes[LogFileGroupName] = fileGroup.Name
	}
	return nil
}

func (r *Resolver) Resolve(file *os.File) (attributes map[string]any, err error) {
	var path = file.Name()
	// size 2 is sufficient if not resolving symlinks. This optimizes for the most performant cases.
	attributes = make(map[string]any, 2)
	if r.IncludeFileName {
		attributes[LogFileName] = filepath.Base(path)
	}
	if r.IncludeFilePath {
		attributes[LogFilePath] = path
	}
	if r.IncludeFileOwnerName || r.IncludeFileGroupName {
		err = r.AddFileInfos(file, attributes)
		if err != nil {
			return nil, err
		}
	}
	if !r.IncludeFileNameResolved && !r.IncludeFilePathResolved {
		return attributes, nil
	}

	resolved := path
	// Dirty solution, waiting for this permanent fix https://github.com/golang/go/issues/39786
	// EvalSymlinks on windows is partially working depending on the way you use Symlinks and Junctions
	if runtime.GOOS != "windows" {
		resolved, err = filepath.EvalSymlinks(path)
		if err != nil {
			return nil, fmt.Errorf("resolve symlinks: %w", err)
		}
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return nil, fmt.Errorf("resolve abs: %w", err)
	}

	if r.IncludeFileNameResolved {
		attributes[LogFileNameResolved] = filepath.Base(abs)
	}
	if r.IncludeFilePathResolved {
		attributes[LogFilePathResolved] = abs
	}
	return attributes, nil
}
