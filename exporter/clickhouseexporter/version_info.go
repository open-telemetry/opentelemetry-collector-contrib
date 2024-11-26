package clickhouseexporter

import (
	"runtime"
	"runtime/debug"
	"sync"
)

var (
	versionOnce      sync.Once
	collectorVersion string
)

func getCollectorVersion() string {
	versionOnce.Do(func() {
		osInformation := runtime.GOOS[:3] + "-" + runtime.GOARCH
		collectorVersion = "unknown-" + osInformation

		info, ok := debug.ReadBuildInfo()
		if !ok {
			return
		}

		collectorVersion = info.Main.Version + "-" + osInformation
	})

	return collectorVersion
}
