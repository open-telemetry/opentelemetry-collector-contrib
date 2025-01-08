// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshalertest

import (
	"errors"
	"go.opentelemetry.io/collector/pdata/plog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewNopLogs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		logs plog.Logs
		err  error
	}{
		{
			name: "no error",
			logs: plog.NewLogs(),
			err:  nil,
		},
		{
			name: "with error",
			logs: plog.NewLogs(),
			err:  errors.New("test error"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			unmarshaler := NewNopLogs(test.logs, test.err)
			got, err := unmarshaler.UnmarshalLogs(nil)
			require.Equal(t, test.err, err)
			require.Equal(t, test.logs, got)
			require.Equal(t, typeStr, unmarshaler.Type())
		})
	}
}
