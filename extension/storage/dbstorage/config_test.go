// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dbstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/dbstorage"

import (
	"errors"
	"fmt"
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
			"Unknown driver",
			Config{DriverName: "foo", DataSource: "bar"},
			fmt.Errorf("unsupported driver %s", "foo"),
		},
		{
			"Valid",
			Config{DriverName: driverSQLite, DataSource: "bar"},
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
