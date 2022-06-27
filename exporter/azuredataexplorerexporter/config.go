// Copyright OpenTelemetry Authors
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

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/config"
)

// Config defines configuration for Azure Monitor
type Config struct {
	config.ExporterSettings `mapstructure:",squash"`
	ClusterName             string `mapstructure:"cluster_name"`
	ClientId                string `mapstructure:"client_id"`
	ClientSecret            string `mapstructure:"client_secret"`
	TenantId                string `mapstructure:"tenant_id"`
	Database                string `mapstructure:"db_name"`
	RawMetricTable          string `mapstructure:"metrics_table_name"`
	IngestionType           string `mapstructure:"ingestion_type"`
}

// Validate checks if the exporter configuration is valid. TODO tests for this one
func (adxCfg *Config) Validate() error {
	if adxCfg == nil {
		return errors.New("ADX config is nil / not provided")
	}

	if isEmpty(adxCfg.ClusterName) || isEmpty(adxCfg.ClientId) || isEmpty(adxCfg.ClientSecret) || isEmpty(adxCfg.TenantId) {
		return errors.New(`mandatory configurations "cluster_name" ,"client_id" , "client_secret" and "tenant_id" are missing or empty `)
	}

	if !(adxCfg.IngestionType == managedingesttype || adxCfg.IngestionType == queuedingesttest || isEmpty(adxCfg.IngestionType)) {
		return fmt.Errorf("unsupported configuration for ingestion_type. Accepted types [%s, %s] Provided [%s]", managedingesttype, queuedingesttest, adxCfg.IngestionType)
	}

	return nil
}

func isEmpty(str string) bool {
	return strings.TrimSpace(str) == ""
}
