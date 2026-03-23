// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscwotlpbatchsplitprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigValidate(t *testing.T) {
	testCases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"valid", Config{MaxRequestByteSize: 1}, false},
		{"zero", Config{MaxRequestByteSize: 0}, true},
		{"negative", Config{MaxRequestByteSize: -1}, true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.cfg.Validate()
			if testCase.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
