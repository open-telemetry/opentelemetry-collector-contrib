// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elbaccesslogs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_convertTextToALBAccessLogRecord_unbracketedIPv6(t *testing.T) {
	line := `https 2018-07-02T22:23:00.186641Z app/my-loadbalancer/50dc6c495c0c9188 2a02:26f7:fb08:4000:1000::b:20381 10.0.0.1:80 0.086 0.048 0.037 200 200 0 57 "GET https://www.example.com:443/ HTTP/1.1" "curl/7.46.0" ECDHE-RSA-AES128-GCM-SHA256 TLSv1.2 arn:aws:elasticloadbalancing:us-east-2:123456789012:targetgroup/my-targets/73e2d6bc24d8a067 "Root=1-58337281-1d84f3d73c47ec4e58577259" "www.example.com" "arn:aws:acm:us-east-2:123456789012:certificate/12345678-1234-1234-1234-123456789012" 1 2018-07-02T22:22:48.364000Z "authenticate,forward" "-" "-" "10.0.0.1:80" "200" "-" "-" TID_1234abcd5678ef90`
	fields, err := extractFields(line)
	require.NoError(t, err)
	record, err := convertTextToALBAccessLogRecord(fields)
	require.NoError(t, err)
	require.Equal(t, "2a02:26f7:fb08:4000:1000::b", record.ClientIP)
	require.Equal(t, int64(20381), record.ClientPort)
}

func Test_scanField(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		wantError string
	}{
		{
			name:     "Simple input",
			input:    "value 123",
			expected: "value",
		},
		{
			name:     "Quoted string with space delimiter",
			input:    `"GET http://example.com/index.html HTTP/1.1" otherValue`,
			expected: "GET http://example.com/index.html HTTP/1.1",
		},
		{
			name:     "Multi byte character handling",
			input:    `"GET http://example.com/こんにちは HTTP/1.1" otherValue`,
			expected: "GET http://example.com/こんにちは HTTP/1.1",
		},
		{
			name:     "Quoted array - expects unquoted array",
			input:    `"a","b","c"`,
			expected: "a,b,c",
		},
		{
			name:     "Quoted string only",
			input:    `"This is quoted and with spaces"`,
			expected: "This is quoted and with spaces",
		},
		{
			name:      "Invalid input",
			input:     `"no end quotes`,
			wantError: "log line has no end quote",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := scanField(tt.input)
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
			}

			require.Equal(t, tt.expected, got)
		})
	}
}

func Test_parseRequestField(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		wantMethod       string
		wantURI          string
		wantProtoName    string
		wantProtoVersion string
		wantErr          bool
	}{
		{
			name:             "Valid input with expected sections",
			input:            "GET http://example.com/ HTTP/1.1",
			wantMethod:       "GET",
			wantURI:          "http://example.com/",
			wantProtoName:    "http",
			wantProtoVersion: "1.1",
		},
		{
			name:             "Missing protocol/version",
			input:            "GET http://example.com/ -",
			wantMethod:       "GET",
			wantURI:          "http://example.com/",
			wantProtoName:    "-",
			wantProtoVersion: "-",
		},
		{
			name:             "URI section with spaces",
			input:            "GET http://example.com/path to somewhere HTTP/1.1",
			wantMethod:       "GET",
			wantURI:          "http://example.com/path to somewhere",
			wantProtoName:    "http",
			wantProtoVersion: "1.1",
		},
		{
			name:             "Input with spaces and missing protocol/version",
			input:            "- http://example.com/path to somewhere- -",
			wantMethod:       "-",
			wantURI:          "http://example.com/path to somewhere-",
			wantProtoName:    "-",
			wantProtoVersion: "-",
		},
		{
			name:             "Missing method and protocol/version",
			input:            "- http://example.com:80- ",
			wantErr:          false,
			wantMethod:       "-",
			wantURI:          "http://example.com:80-",
			wantProtoName:    "-",
			wantProtoVersion: "-",
		},
		{
			name:       "Invalid input with missing expected sections",
			input:      "GET /",
			wantMethod: "GET",
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMethod, gotURI, gotProtoName, gotProtoVersion, err := parseRequestField(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRequestField() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotMethod != tt.wantMethod {
				t.Errorf("parseRequestField() gotMethod = %v, want %v", gotMethod, tt.wantMethod)
			}
			if gotURI != tt.wantURI {
				t.Errorf("parseRequestField() gotURI = %v, want %v", gotURI, tt.wantURI)
			}
			if gotProtoName != tt.wantProtoName {
				t.Errorf("parseRequestField() gotProtoName = %v, want %v", gotProtoName, tt.wantProtoName)
			}
			if gotProtoVersion != tt.wantProtoVersion {
				t.Errorf("parseRequestField() gotProtoVersion = %v, want %v", gotProtoVersion, tt.wantProtoVersion)
			}
		})
	}
}
