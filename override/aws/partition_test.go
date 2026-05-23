// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPartition(t *testing.T) {
	tests := map[string]string{
		"us-east-1":       "aws",
		"eu-west-2":       "aws",
		"ap-east-1":       "aws", // opt-in region in commercial partition
		"cn-north-1":      "aws-cn",
		"cn-northwest-1":  "aws-cn",
		"us-gov-west-1":   "aws-us-gov",
		"us-gov-east-1":   "aws-us-gov",
		"us-iso-east-1":   "aws-iso",
		"us-isob-east-1":  "aws-iso-b",
		"eu-isoe-west-1":  "aws-iso-e",
		"us-isof-south-1": "aws-iso-f",
		"eusc-de-east-1":  "aws-eusc",
		"":                "aws", // empty resolves to default partition
		"not-a-region":    "aws", // unknown patterns fall through to default
	}
	for region, want := range tests {
		t.Run(region, func(t *testing.T) {
			assert.Equal(t, want, GetPartition(region))
		})
	}
}

func TestGetPartitionPrimaryRegion(t *testing.T) {
	tests := map[string]string{
		"us-east-1":       "us-east-1",
		"eu-west-2":       "us-east-1",
		"ap-east-1":       "us-east-1",
		"cn-north-1":      "cn-north-1",
		"cn-northwest-1":  "cn-north-1",
		"us-gov-east-1":   "us-gov-west-1",
		"us-gov-west-1":   "us-gov-west-1",
		"us-iso-east-1":   "us-iso-east-1",
		"us-isob-east-1":  "us-isob-east-1",
		"eu-isoe-west-1":  "eu-isoe-west-1",
		"us-isof-south-1": "us-isof-south-1",
		"eusc-de-east-1":  "eusc-de-east-1",
		// Unknown patterns resolve to "aws" partition, mapping to its primary.
		"":             "us-east-1",
		"not-a-region": "us-east-1",
	}
	for region, want := range tests {
		t.Run(region, func(t *testing.T) {
			assert.Equal(t, want, GetPartitionPrimaryRegion(region))
		})
	}
}
