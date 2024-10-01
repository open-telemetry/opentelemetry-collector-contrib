// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package attrs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"

import (
	"fmt"
	"os"
	"os/user"
	"syscall"
)

func (r *Resolver) addOwnerInfo(file *os.File, attributes map[string]any) error {
	fileInfo, errStat := file.Stat()
	if errStat != nil {
		return fmt.Errorf("resolve file stat: %w", errStat)
	}
	fileStat := fileInfo.Sys().(*syscall.Stat_t)

	if r.IncludeFileOwnerName {
		fileOwner, errFileUser := user.LookupId(fmt.Sprint(fileStat.Uid))
		if errFileUser != nil {
			return fmt.Errorf("resolve file owner name: %w", errFileUser)
		}
		attributes[LogFileOwnerName] = fileOwner.Username
	}
	if r.IncludeFileOwnerGroupName {
		fileGroup, errFileGroup := user.LookupGroupId(fmt.Sprint(fileStat.Gid))
		if errFileGroup != nil {
			return fmt.Errorf("resolve file group name: %w", errFileGroup)
		}
		attributes[LogFileOwnerGroupName] = fileGroup.Name
	}
	return nil
}
