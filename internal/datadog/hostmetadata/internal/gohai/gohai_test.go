// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package gohai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestGetPayload(t *testing.T) {
	logger := zap.NewNop()
	gohai := NewPayload(logger)
	assert.NotNil(t, gohai.Gohai.Gohai.CPU)
	assert.NotPanics(t, func() { gohai.CPU() })
	assert.NotNil(t, gohai.Gohai.Gohai.FileSystem)
	assert.NotNil(t, gohai.Gohai.Gohai.Memory)
	assert.NotNil(t, gohai.Gohai.Gohai.Network)
	assert.NotPanics(t, func() { gohai.Network() })
	assert.NotNil(t, gohai.Gohai.Gohai.Platform)
	assert.NotPanics(t, func() { gohai.Platform() })
}
