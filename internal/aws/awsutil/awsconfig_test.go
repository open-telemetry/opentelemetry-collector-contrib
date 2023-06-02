// Copyright The OpenTelemetry Authors
// Portions of this file Copyright 2018-2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package awsutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultSessionConfig(t *testing.T) {
	expectedCfg := AWSSessionSettings{
		NumberOfWorkers:       8,
		Endpoint:              "",
		RequestTimeoutSeconds: 30,
		MaxRetries:            2,
		NoVerifySSL:           false,
		ProxyAddress:          "",
		Region:                "",
		LocalMode:             false,
		ResourceARN:           "",
		RoleARN:               "",
	}
	assert.Equal(t, expectedCfg, CreateDefaultSessionConfig())
}
