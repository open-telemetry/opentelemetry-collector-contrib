// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwlog"

type extractedFields struct {
	AccountID string `json:"@aws.account,omitempty"`
	Region    string `json:"@aws.region,omitempty"`
}

type cWLog struct {
	MessageType         string   `json:"messageType"`
	Owner               string   `json:"owner"`
	LogGroup            string   `json:"logGroup"`
	LogStream           string   `json:"logStream"`
	SubscriptionFilters []string `json:"subscriptionFilters"`
	LogEvents           []struct {
		ID              string           `json:"id"`
		Timestamp       int64            `json:"timestamp"`
		Message         string           `json:"message"`
		ExtractedFields *extractedFields `json:"extractedFields,omitempty"`
	} `json:"logEvents"`
}
