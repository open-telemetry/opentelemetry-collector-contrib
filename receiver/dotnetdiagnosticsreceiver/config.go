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
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

type Config struct {
	// ScraperController's collection_interval is used to set the interval between
	// metric collection. The interval is converted to seconds and sent to the
	// dotnet backend at receiver startup time. The dotnet process then sends the
	// receiver data at the specified interval. Defaults to 1 second.
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	// The process ID of the dotnet process from which to collect diagnostics. This
	// process ID is used to generate the file glob "dotnet-diagnostic-%d-*-socket"
	// to locate a file in TMPDIR (or "/tmp" if unset). If the file is found, it is
	// used as a Unix domain socket (on Linux/Mac) to communicate with the dotnet
	// process. For ease of use, this receiver is intended to be used with an
	// observer and receiver creator for process discovery and receiver creation.
	PID int `mapstructure:"pid"`
	// A list of counters for the dotnet process to send to the collector. Defaults
	// to ["System.Runtime", "Microsoft.AspNetCore.Hosting"]. Available counters can
	// be displayed by the `dotnet-counters` tool:
	// https://docs.microsoft.com/en-us/dotnet/core/diagnostics/dotnet-counters
	Counters []string `mapstructure:"counters"`
}
