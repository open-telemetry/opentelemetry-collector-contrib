// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_openshift) && !omit_detector_openshift

package openshift

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	assert.Equal(t, Config{ResourceAttributes: metadata.DefaultResourceAttributesConfig()}, CreateDefaultConfig())
}
