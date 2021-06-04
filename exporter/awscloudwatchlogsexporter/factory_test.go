// Copyright 2020, OpenTelemetry Authors
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

package awscloudwatchlogsexporter

import (
	"reflect"
	"testing"

	"go.opentelemetry.io/collector/config/configmodels"
)

func TestDefaultConfig_exporterSettings(t *testing.T) {
	config := createDefaultConfig().(*Config)
	want := &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type("awscloudwatchlogs"),
			NameVal: "awscloudwatchlogs",
		},
	}
	if got := config; !reflect.DeepEqual(got.ExporterSettings, want.ExporterSettings) {
		t.Errorf("createDefaultConfig().ExporterSettings = %v, want %v", got, want)
	}
}
