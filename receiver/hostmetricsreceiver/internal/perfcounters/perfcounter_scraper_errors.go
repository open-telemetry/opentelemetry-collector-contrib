package perfcounters

import "errors"

// ErrAllObjectsUnavailable is an error returned by PerfCounterScraper.Initialize when all objects fail to be initialized.
// This indicates that not even a partial scrape is possible.
var ErrAllObjectsUnavailable = errors.New("all request counters unavailable")
