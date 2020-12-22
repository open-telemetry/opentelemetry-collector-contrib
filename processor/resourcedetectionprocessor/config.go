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

package resourcedetectionprocessor

import (
	"time"

	"go.opentelemetry.io/collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ec2"
)

// Config defines configuration for Resource processor.
type Config struct {
	configmodels.ProcessorSettings `mapstructure:",squash"`
	// Detectors is an ordered list of named detectors that should be
	// run to attempt to detect resource information.
	Detectors []string `mapstructure:"detectors"`
	// Timeout specifies the maximum amount of time that we will wait
	// before assuming a detector has failed. Defaults to 5s.
	Timeout time.Duration `mapstructure:"timeout"`
	// Override indicates whether any existing resource attributes
	// should be overridden or preserved. Defaults to true.
	Override bool `mapstructure:"override"`
	// InternalConfig is a list of settings specific to all detectors
	DetectorConfigs DetectorConfigs `mapstructure:",squash"`
}

type DetectorConfigs struct {
	EC2Config ec2.Config `mapstructure:"ec2"`
}

func (d *DetectorConfigs) GetConfigFromType(detectorType internal.DetectorType) internal.DetectorConfig{
	switch detectorType {
	case ec2.TypeStr:
		return d.EC2Config
	default:
		return nil
	}
}