// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/filter"
import (
	"os"
	"time"

	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/stanzatime"
)

type excludeOlderThanOption struct {
	age time.Duration
}

func (eot excludeOlderThanOption) apply(items []*item) ([]*item, error) {
	filteredItems := make([]*item, 0, len(items))
	var errs error
	for _, item := range items {
		fi, err := os.Stat(item.value)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}

		// Keep (include) the file if its age (since last modification)
		// is the same or less than the configured age.
		fileAge := stanzatime.Since(fi.ModTime())
		if fileAge <= eot.age {
			filteredItems = append(filteredItems, item)
		}
	}

	return filteredItems, errs
}

// ExcludeOlderThan excludes files whose modification time is older than the specified age.
func ExcludeOlderThan(age time.Duration) Option {
	return excludeOlderThanOption{age: age}
}
