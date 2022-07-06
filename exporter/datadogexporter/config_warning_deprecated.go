// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"fmt"

	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/multierr"
)

var _ error = (*renameError)(nil)

// renameError is an error related to a renamed setting.
type renameError struct {
	// oldName of the configuration option.
	oldName string
	// newName of the configuration option.
	newName string
	// oldRemovedIn is the version where the old config option will be removed.
	oldRemovedIn string
	// issueNumber on opentelemetry-collector-contrib for tracking
	issueNumber uint
}

// List of settings that have been removed, but for which we keep a custom error.
var removedSettings = []renameError{
	{
		oldName:      "metrics::send_monotonic_counter",
		newName:      "metrics::sums::cumulative_monotonic_mode",
		oldRemovedIn: "v0.50.0",
		issueNumber:  8489,
	},
	{
		oldName:      "tags",
		newName:      "host_metadata::tags",
		oldRemovedIn: "v0.52.0",
		issueNumber:  9099,
	},
	{
		oldName:      "send_metadata",
		newName:      "host_metadata::enabled",
		oldRemovedIn: "v0.52.0",
		issueNumber:  9099,
	},
	{
		oldName:      "use_resource_metadata",
		newName:      "host_metadata::hostname_source",
		oldRemovedIn: "v0.52.0",
		issueNumber:  9099,
	},
	{
		oldName:      "metrics::report_quantiles",
		newName:      "metrics::summaries::mode",
		oldRemovedIn: "v0.53.0",
		issueNumber:  8845,
	},
}

// Error implements the error interface.
func (e renameError) Error() string {
	return fmt.Sprintf(
		"%q has been deprecated in favor of %q and will be removed in %s or later. See github.com/open-telemetry/opentelemetry-collector-contrib/issues/%d",
		e.oldName,
		e.newName,
		e.oldRemovedIn,
		e.issueNumber,
	)
}

// RemovedErr returns an error describing that the old name was removed in favor of the new name.
func (e renameError) RemovedErr(configMap *confmap.Conf) error {
	if configMap.IsSet(e.oldName) {
		return fmt.Errorf(
			"%q was removed in favor of %q. See github.com/open-telemetry/opentelemetry-collector-contrib/issues/%d",
			e.oldName,
			e.newName,
			e.issueNumber,
		)
	}
	return nil
}

func handleRemovedSettings(configMap *confmap.Conf) (err error) {
	for _, removed := range removedSettings {
		err = multierr.Append(err, removed.RemovedErr(configMap))
	}
	return
}
