// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ocsfconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/ocsfconnector"

type Config struct {
	Mappings []*Mapping `mapstructure:"mappings"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type Mapping struct {
	Detection       string `mapstructure:"detection"`
	ClassUID        int    `mapstructure:"class_uid"`
	ClassName       string `mapstructure:"class_name"`
	CategoryUID     int    `mapstructure:"category_uid"`
	CategoryName    string `mapstructure:"category_name"`
	ActivityID      int    `mapstructure:"activity_id"`
	ActivityName    string `mapstructure:"activity_name"`
	SeverityID      int    `mapstructure:"severity_id"`
	Severity        string `mapstructure:"severity"`
	MessageTemplate string `mapstructure:"message"`
}
