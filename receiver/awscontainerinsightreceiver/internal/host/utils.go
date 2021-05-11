package host

import "time"

// execute the refresh() function periodically with the given refresh interval
// until shouldRefresh() return false or the shutdownC is closed
func refreshUntil(refresh func(), refreshInterval time.Duration,
	shouldRefresh func() bool, shutdownC <-chan bool) {
	refreshTicker := time.NewTicker(refreshInterval)
	defer refreshTicker.Stop()
	for {
		select {
		case <-refreshTicker.C:
			if !shouldRefresh() {
				return
			}
			refresh()
		case <-shutdownC:
			return
		}
	}
}
