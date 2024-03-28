// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Azure Data Explorer Exporter
type Config struct {
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	configretry.BackOffConfig      `mapstructure:"retry_on_failure"`
	ClusterURI                     string              `mapstructure:"cluster_uri"`
	ApplicationID                  string              `mapstructure:"application_id"`
	ApplicationKey                 configopaque.String `mapstructure:"application_key"`
	TenantID                       string              `mapstructure:"tenant_id"`
	ManagedIdentityID              string              `mapstructure:"managed_identity_id"`
	Database                       string              `mapstructure:"db_name"`
	MetricTable                    string              `mapstructure:"metrics_table_name"`
	LogTable                       string              `mapstructure:"logs_table_name"`
	TraceTable                     string              `mapstructure:"traces_table_name"`
	MetricTableMapping             string              `mapstructure:"metrics_table_json_mapping"`
	LogTableMapping                string              `mapstructure:"logs_table_json_mapping"`
	TraceTableMapping              string              `mapstructure:"traces_table_json_mapping"`
	IngestionType                  string              `mapstructure:"ingestion_type"`
}

// Validate checks if the exporter configuration is valid
func (adxCfg *Config) Validate() error {
	if adxCfg == nil {
		return errors.New("ADX config is nil / not provided")
	}
	isAppAuthEmpty := isEmpty(adxCfg.ApplicationID) || isEmpty(string(adxCfg.ApplicationKey)) || isEmpty(adxCfg.TenantID)
	isManagedAuthEmpty := isEmpty(adxCfg.ManagedIdentityID)
	isClusterURIEmpty := isEmpty(adxCfg.ClusterURI)
	// Cluster URI is the target ADX cluster
	if isClusterURIEmpty {
		return errors.New(`clusterURI config is mandatory`)
	}
	// Parameters for AD App Auth or Managed Identity Auth are mandatory
	if isAppAuthEmpty && isManagedAuthEmpty {
		return errors.New(`either ["application_id" , "application_key" , "tenant_id"] or ["managed_identity_id"] are needed for auth`)
	}

	if !(adxCfg.IngestionType == managedIngestType || adxCfg.IngestionType == queuedIngestTest || isEmpty(adxCfg.IngestionType)) {
		return fmt.Errorf("unsupported configuration for ingestion_type. Accepted types [%s, %s] Provided [%s]", managedIngestType, queuedIngestTest, adxCfg.IngestionType)
	}
	// Validate managed identity ID. Use system for system assigned managed identity or UserManagedIdentityID (objectID) for user assigned managed identity
	if !isEmpty(adxCfg.ManagedIdentityID) && !strings.EqualFold(strings.TrimSpace(adxCfg.ManagedIdentityID), "SYSTEM") {
		// if the managed identity is not a system identity, validate if it is a valid UUID
		_, err := uuid.Parse(strings.TrimSpace(adxCfg.ManagedIdentityID))
		if err != nil {
			return errors.New("managed_identity_id should be a UUID string (for User Managed Identity) or system (for System Managed Identity)")
		}
	}
	return nil
}

func isEmpty(str string) bool {
	return strings.TrimSpace(str) == ""
}
