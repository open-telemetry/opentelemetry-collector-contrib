// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dbstorage

import (
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func Test_replaceCompatDSNOptions(t *testing.T) {
	logger := zap.L()
	tests := []struct {
		name    string
		dsn     string
		want    url.Values
		wantErr bool
	}{
		{
			name:    "Should keep DSN as is if no options provided",
			dsn:     "file://foo.db?",
			want:    url.Values{},
			wantErr: false,
		},
		{
			name: "Should keep new driver options",
			dsn:  "file://foo.db?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)",
			want: url.Values{
				"_pragma": []string{
					"busy_timeout(10000)",
					"journal_mode(WAL)",
					"synchronous(NORMAL)",
				},
			},
			wantErr: false,
		},
		{
			name: "Should convert legacy driver options",
			dsn:  "file://foo.db?_busy_timeout=10000&_journal=WAL&_sync=NORMAL",
			want: url.Values{
				"_pragma": []string{
					"busy_timeout(10000)",
					"journal_mode(WAL)",
					"synchronous(NORMAL)",
				},
			},
			wantErr: false,
		},
		{
			name:    "Should return error on incorrect query sting",
			dsn:     "file://foo.db?;malformed_query_string",
			want:    url.Values{},
			wantErr: true,
		},
		{
			name: "Should return error on unknown legacy driver option",
			dsn:  "file://foo.db?_unknown_option=10000&_journal=WAL&_sync=NORMAL",
			want: url.Values{
				"_pragma": []string{
					"journal_mode(WAL)",
					"synchronous(NORMAL)",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := replaceCompatDSNOptions(logger, tt.dsn)
			pos := strings.IndexRune(got, '?')
			values, _ := url.ParseQuery(got[pos+1:])
			// Sort value slices to avoid flaky test results
			for _, v := range values {
				if len(v) > 1 {
					slices.Sort(v)
				}
			}
			assert.Equal(t, tt.want, values)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
