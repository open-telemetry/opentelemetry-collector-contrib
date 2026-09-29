// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsiamdbauthextension

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	require.NoError(t, (&Config{Region: "us-east-1"}).Validate(), "a region is the only required field")
	require.ErrorIs(t, (&Config{}).Validate(), errNoRegion, "an empty region fails at config load")
}

func TestValidateRejectsAssumeRoleFieldsWithoutARN(t *testing.T) {
	cfg := &Config{Region: "eu-central-1", AssumeRole: AssumeRole{SessionName: "x"}}
	require.ErrorIs(t, cfg.Validate(), errAssumeRoleWithoutARN)
}

func TestValidateAcceptsAssumeRole(t *testing.T) {
	cfg := &Config{Region: "eu-central-1", AssumeRole: AssumeRole{ARN: "arn:aws:iam::123456789012:role/reader"}}
	require.NoError(t, cfg.Validate())
}
