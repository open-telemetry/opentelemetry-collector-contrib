// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
)

// Non-SemConv attributes that are used for common Azure Log Record fields
const (
	// OpenTelemetry attributes for the Recommendation category,
	// possible values are "High Availability", "Performance", "Security", and "Cost"
	attributeAzureRecommendationCategory = "azure.recommendation.category"

	// OpenTelemetry attributes for the impact of the recommendation,
	// possible values are "High", "Medium", and "Low"
	attributeAzureRecommendationImpact = "azure.recommendation.impact"

	// OpenTelemetry attributes for other recommendation name
	attributeAzureRecommendationName = "azure.recommendation.name"

	// OpenTelemetry attributes for other recommendation type
	attributeAzureRecommendationType = "azure.recommendation.type"

	// OpenTelemetry attributes for other recommendation schema version published in the Activity Log entry
	attributeAzureRecommendationSchemaVersion = "azure.recommendation.schema_version"

	// OpenTelemetry attributes for link to the recommendation link
	attributeAzureRecommendationLink = "azure.recommendation.link"

	// OpenTelemetry attributes for the risk of the recommendation risk,
	// possible values are "High", "Medium", and "Low"
	attributeAzureRecommendationRisk = "azure.recommendation.risk"
)

// See https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#recommendation-category
type azureRecommendationLog struct {
	azureLogRecordBase

	Properties struct {
		RecommendationSchemaVersion string `json:"recommendationSchemaVersion"`
		RecommendationCategory      string `json:"recommendationCategory"`
		RecommendationImpact        string `json:"recommendationImpact"`
		RecommendationName          string `json:"recommendationName"`
		RecommendationResourceLink  string `json:"recommendationResourceLink"`
		RecommendationType          string `json:"recommendationType"`
		RecommendationRisk          string `json:"recommendationRisk"`
	} `json:"properties"`
}

func (r *azureRecommendationLog) PutProperties(attrs pcommon.Map, _ pcommon.Value) error {
	unmarshaler.AttrPutStrIf(attrs, attributeAzureRecommendationCategory, r.Properties.RecommendationCategory)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureRecommendationImpact, r.Properties.RecommendationImpact)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureRecommendationName, r.Properties.RecommendationName)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureRecommendationType, r.Properties.RecommendationType)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureRecommendationSchemaVersion, r.Properties.RecommendationSchemaVersion)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureRecommendationLink, r.Properties.RecommendationResourceLink)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureRecommendationRisk, r.Properties.RecommendationRisk)

	return nil
}
