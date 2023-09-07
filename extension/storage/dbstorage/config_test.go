// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Skip tests on Windows temporarily, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/11451
//go:build !windows
// +build !windows

package dbstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/dbstorage"

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		errWanted error
	}{
		{
			"Missing driver name",
			Config{DataSource: "foo"},
			errors.New("missing driver name"),
		},
		{
			"Missing datasource",
			Config{DriverName: "foo"},
			errors.New("missing datasource"),
		},
		{
			"valid",
			Config{DriverName: "foo", DataSource: "bar"},
			nil,
		},
	}

	for _, test := range tests {
		err := test.config.Validate()
		if test.errWanted == nil {
			assert.NoError(t, err)
		} else {
			assert.Equal(t, test.errWanted, err)
		}
	}
}
