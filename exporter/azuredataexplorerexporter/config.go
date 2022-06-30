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
	ClusterUri              string `mapstructure:"cluster_uri"`
	ApplicationId           string `mapstructure:"application_id"`
	ApplicationKey          string `mapstructure:"application_key"`
	TenantId                string `mapstructure:"tenant_id"`
	Database                string `mapstructure:"db_name"`
	OTELMetricTable         string `mapstructure:"metrics_table_name"`
	OTELLogTable            string `mapstructure:"logs_table_name"`
	OTELTraceTable          string `mapstructure:"traces_table_name"`
	OTELMetricTableMapping  string `mapstructure:"metrics_table_name_mapping"`
	OTELLogTableMapping     string `mapstructure:"logs_table_name_mapping"`
	OTELTraceTableMapping   string `mapstructure:"traces_table_name_mapping"`
	IngestionType           string `mapstructure:"ingestion_type"`
}

// Validate checks if the exporter configuration is valid. TODO tests for this one
func (adxCfg *Config) Validate() error {
	if adxCfg == nil {
		return errors.New("ADX config is nil / not provided")
	}

	if isEmpty(adxCfg.ClusterUri) || isEmpty(adxCfg.ApplicationId) || isEmpty(adxCfg.ApplicationKey) || isEmpty(adxCfg.TenantId) {
		return errors.New(`mandatory configurations "cluster_uri" ,"application_id" , "application_key" and "tenant_id" are missing or empty `)
	}

	if !(adxCfg.IngestionType == managedingesttype || adxCfg.IngestionType == queuedingesttest || isEmpty(adxCfg.IngestionType)) {
		return fmt.Errorf("unsupported configuration for ingestion_type. Accepted types [%s, %s] Provided [%s]", managedingesttype, queuedingesttest, adxCfg.IngestionType)
	}

	return nil
}

func isEmpty(str string) bool {
	return strings.TrimSpace(str) == ""
}
