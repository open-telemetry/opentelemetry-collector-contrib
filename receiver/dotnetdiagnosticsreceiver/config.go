// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dotnetdiagnosticsreceiver

import (
	"time"

	"go.opentelemetry.io/collector/config/configmodels"
)

type Config struct {
	configmodels.ReceiverSettings `mapstructure:",squash"`
	// Optional directory name to save binary blobs read from the socket. This can
	// be useful for troubleshooting/debugging. If left blank or unset, no blobs are
	// saved.
	BlobDir string `mapstructure:"blob_dir"`
	// The maximum number of blob files to keep in the blob_dir, if specified. Defaults
	// to 10 (most recent).
	MaxBlobFiles int `mapstructure:"max_blob_files"`
	// The process ID of the dotnet process from which to collect diagnostics. This
	// receiver is intended to be used with an observer and receiver creator for
	// process discovery.
	PID int `mapstructure:"pid"`
	// The duration between collection of metrics. The duration value is converted
	// to seconds and sent to the dotnet backend at receiver startup time.
	// Defaults to 1 second.
	CollectionInterval time.Duration `mapstructure:"collection_interval"`
	// A list of counters for the dotnet process to send to the collector.
	// Defaults to "System.Runtime". Available counters can be displayed
	// by the `dotnet-counters` tool:
	// https://docs.microsoft.com/en-us/dotnet/core/diagnostics/dotnet-counters
	Counters []string `mapstructure:"counters"`
}
