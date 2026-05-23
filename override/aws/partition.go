// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aws // import "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"

import (
	"github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws/awsrulesfn"
)

// partitionPrimaryRegions maps each AWS partition to its primary region.
// When AWS introduces a new partition, add an entry here. Until that
// happens, GetPartitionPrimaryRegion returns "" for the new partition.
var partitionPrimaryRegions = map[string]string{
	"aws":        "us-east-1",
	"aws-cn":     "cn-north-1",
	"aws-us-gov": "us-gov-west-1",
	"aws-iso":    "us-iso-east-1",
	"aws-iso-b":  "us-isob-east-1",
	"aws-iso-e":  "eu-isoe-west-1",
	"aws-iso-f":  "us-isof-south-1",
	"aws-eusc":   "eusc-de-east-1",
}

// GetPartition returns the AWS partition ID for the given region (e.g.
// "aws", "aws-cn"). Returns "" if the region cannot be resolved.
func GetPartition(region string) string {
	p := awsrulesfn.GetPartition(region)
	if p == nil {
		return ""
	}
	return p.Name
}

// GetPartitionPrimaryRegion returns the primary region of the partition
// that contains region. Returns "" if the partition has no primary
// region entry.
func GetPartitionPrimaryRegion(region string) string {
	p := awsrulesfn.GetPartition(region)
	if p == nil {
		return ""
	}
	return partitionPrimaryRegions[p.Name]
}
