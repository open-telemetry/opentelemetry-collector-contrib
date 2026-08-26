// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cvm

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// roundTripperFunc implements http.RoundTripper from a function.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGetMetadata(t *testing.T) {
	tests := []struct {
		name      string
		doer      roundTripperFunc
		want      *Metadata
		expectErr bool
	}{
		{
			name: "successfully retrieves all metadata",
			doer: func(req *http.Request) (*http.Response, error) {
				responses := map[string]string{
					metadataBaseURL + "instance-name":          "cvm-test-instance",
					metadataBaseURL + "instance/image-id":      "img-abcdef123456",
					metadataBaseURL + "instance-id":            "ins-abcdef123456",
					metadataBaseURL + "instance/instance-type": "S5.MEDIUM2",
					metadataBaseURL + "app-id":                 "1234567890",
					metadataBaseURL + "placement/region":       "ap-guangzhou",
					metadataBaseURL + "placement/zone":         "ap-guangzhou-3",
				}
				if resp, ok := responses[req.URL.String()]; ok {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewBufferString(resp)),
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(bytes.NewBufferString("not found")),
				}, nil
			},
			want: &Metadata{
				InstanceName: "cvm-test-instance",
				ImageID:      "img-abcdef123456",
				InstanceID:   "ins-abcdef123456",
				InstanceType: "S5.MEDIUM2",
				AppID:        "1234567890",
				RegionID:     "ap-guangzhou",
				ZoneID:       "ap-guangzhou-3",
			},
		},
		{
			name: "connection failure",
			doer: func(_ *http.Request) (*http.Response, error) {
				return nil, errors.New("connection refused")
			},
			expectErr: true,
		},
		{
			name: "metadata request returns non-200",
			doer: func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(bytes.NewBufferString("metadata not found")),
				}, nil
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &metadataClient{
				client: &http.Client{
					Timeout:   1 * time.Second,
					Transport: tt.doer,
				},
			}

			got, err := p.Metadata(t.Context())
			if tt.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want.InstanceName, got.InstanceName)
			require.Equal(t, tt.want.ImageID, got.ImageID)
			require.Equal(t, tt.want.InstanceID, got.InstanceID)
			require.Equal(t, tt.want.InstanceType, got.InstanceType)
			require.Equal(t, tt.want.AppID, got.AppID)
			require.Equal(t, tt.want.RegionID, got.RegionID)
			require.Equal(t, tt.want.ZoneID, got.ZoneID)
		})
	}
}

func TestResponseSizeLimit(t *testing.T) {
	// Create a response larger than maxResponseSize
	oversizedResponse := strings.Repeat("x", maxResponseSize+1000)

	doer := func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(oversizedResponse)),
		}, nil
	}

	p := &metadataClient{
		client: &http.Client{
			Timeout:   1 * time.Second,
			Transport: roundTripperFunc(doer),
		},
	}

	// The request should succeed, but responses will be truncated
	meta, err := p.Metadata(t.Context())
	require.NoError(t, err)
	// Verify the value was truncated to maxResponseSize
	require.Len(t, meta.InstanceName, maxResponseSize)
}
