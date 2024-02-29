//go:build linux || solaris

package attrs

import (
	"fmt"
	"os"
	"os/user"
	"syscall"
)

func (r *Resolver) addOwnerInfo(file *os.File, attributes map[string]any) (err error) {
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
