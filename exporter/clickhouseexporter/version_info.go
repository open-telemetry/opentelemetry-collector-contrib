package clickhouseexporter

import (
	"runtime"
	"runtime/debug"
	"sync"
)

var (
	once    sync.Once
	version string
)

func getCollectorVersion() string {
	once.Do(func() {
		osInformation := runtime.GOOS[:3] + "-" + runtime.GOARCH
		version = "unknown-" + osInformation

		info, ok := debug.ReadBuildInfo()
		if !ok {
			return
		}

		version = info.Main.Version + "-" + osInformation
	})

	return version
}
