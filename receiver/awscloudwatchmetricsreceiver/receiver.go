// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awscloudwatchmetricsreceiver

import (
	"errors"
	"time"
)

var (
	defaultPollInterval = time.Minute
)

// Config is the overall config structure for the awscloudwatchmetricsreceiver
type Config struct {
	Region       string        `mapstructure:"region"`
	Profile      string        `mapstructure:"profile"`
	IMDSEndpoint string        `mapstructure:"imds_endpoint"`
	PollInterval time.Duration `mapstructure:"poll_interval"`
}

var (
	errNoLogsConfigured    = errors.New("no metrics configured")
	errNoRegion            = errors.New("no region was specified")
	errInvalidPollInterval = errors.New("poll interval is incorrect, it must be a duration greater than one second")
)
