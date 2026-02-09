// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecs

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"
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
				// Handle token request
				if req.URL.String() == tokenURL {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewBufferString("test-token-12345")),
					}, nil
				}
				// Handle metadata requests
				responses := map[string]string{
					metadataBaseURL + "hostname":               "ecs-instance-01",
					metadataBaseURL + "image-id":               "m-abcdef123456",
					metadataBaseURL + "instance-id":            "i-abcdef123456",
					metadataBaseURL + "instance/instance-type": "ecs.g6.large",
					metadataBaseURL + "owner-account-id":       "1234567890123456",
					metadataBaseURL + "region-id":              "cn-hangzhou",
					metadataBaseURL + "zone-id":                "cn-hangzhou-a",
				}
				if resp, ok := responses[req.URL.String()]; ok {
					// Verify token header is present
					if req.Header.Get(tokenHeader) != "test-token-12345" {
						return &http.Response{
							StatusCode: http.StatusUnauthorized,
							Body:       io.NopCloser(bytes.NewBufferString("missing or invalid token")),
						}, nil
					}
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
				Hostname:       "ecs-instance-01",
				ImageID:        "m-abcdef123456",
				InstanceID:     "i-abcdef123456",
				InstanceType:   "ecs.g6.large",
				OwnerAccountID: "1234567890123456",
				RegionID:       "cn-hangzhou",
				ZoneID:         "cn-hangzhou-a",
			},
		},
		{
			name: "token request fails",
			doer: func(req *http.Request) (*http.Response, error) {
				if req.URL.String() == tokenURL {
					return nil, errors.New("connection refused")
				}
				return nil, errors.New("unexpected request")
			},
			expectErr: true,
		},
		{
			name: "token request returns non-200",
			doer: func(req *http.Request) (*http.Response, error) {
				if req.URL.String() == tokenURL {
					return &http.Response{
						StatusCode: http.StatusForbidden,
						Body:       io.NopCloser(bytes.NewBufferString("forbidden")),
					}, nil
				}
				return nil, errors.New("unexpected request")
			},
			expectErr: true,
		},
		{
			name: "metadata request fails",
			doer: func(req *http.Request) (*http.Response, error) {
				if req.URL.String() == tokenURL {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewBufferString("test-token")),
					}, nil
				}
				return nil, errors.New("connection failed")
			},
			expectErr: true,
		},
		{
			name: "metadata request returns non-200",
			doer: func(req *http.Request) (*http.Response, error) {
				if req.URL.String() == tokenURL {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewBufferString("test-token")),
					}, nil
				}
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
			require.Equal(t, tt.want.Hostname, got.Hostname)
			require.Equal(t, tt.want.ImageID, got.ImageID)
			require.Equal(t, tt.want.InstanceID, got.InstanceID)
			require.Equal(t, tt.want.InstanceType, got.InstanceType)
			require.Equal(t, tt.want.OwnerAccountID, got.OwnerAccountID)
			require.Equal(t, tt.want.RegionID, got.RegionID)
			require.Equal(t, tt.want.ZoneID, got.ZoneID)
		})
	}
}

func TestTokenCaching(t *testing.T) {
	tokenRequestCount := 0

	doer := func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == tokenURL {
			tokenRequestCount++
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("cached-token")),
			}, nil
		}

		if strings.HasPrefix(req.URL.String(), metadataBaseURL) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("test-value")),
			}, nil
		}

		return nil, errors.New("unexpected request")
	}

	p := &metadataClient{
		client: &http.Client{
			Timeout:   1 * time.Second,
			Transport: roundTripperFunc(doer),
		},
	}

	ctx := t.Context()

	// Make multiple metadata requests - each Metadata() call makes multiple HTTP requests
	// but should only fetch the token once
	_, _ = p.Metadata(ctx)
	_, _ = p.Metadata(ctx)

	// Token should only be fetched once due to caching
	require.Equal(t, 1, tokenRequestCount, "expected 1 token request due to caching")
}

func TestResponseSizeLimit(t *testing.T) {
	// Create a response larger than maxResponseSize
	oversizedResponse := strings.Repeat("x", maxResponseSize+1000)

	doer := func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == tokenURL {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(oversizedResponse)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("test-value")),
		}, nil
	}

	p := &metadataClient{
		client: &http.Client{
			Timeout:   1 * time.Second,
			Transport: roundTripperFunc(doer),
		},
	}

	// The request should succeed, but the token will be truncated
	// This validates that io.LimitReader is working and prevents memory exhaustion
	_, err := p.Metadata(t.Context())
	require.NoError(t, err)

	// Verify the token was truncated to maxResponseSize
	require.Len(t, p.token, maxResponseSize)
}

func TestTokenHeaderVerification(t *testing.T) {
	doer := func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == tokenURL {
			// Verify TTL header is set correctly
			ttl := req.Header.Get(tokenTTLHeader)
			require.Equal(t, strconv.Itoa(defaultTokenTTL), ttl, "TTL header mismatch")
			// Verify it's a PUT request
			require.Equal(t, http.MethodPut, req.Method, "expected PUT method for token request")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("valid-token")),
			}, nil
		}

		// Verify token header on metadata requests
		token := req.Header.Get(tokenHeader)
		if token != "valid-token" {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(bytes.NewBufferString("invalid token")),
			}, nil
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("test-value")),
		}, nil
	}

	p := &metadataClient{
		client: &http.Client{
			Timeout:   1 * time.Second,
			Transport: roundTripperFunc(doer),
		},
	}

	meta, err := p.Metadata(t.Context())
	require.NoError(t, err)
	require.Equal(t, "test-value", meta.Hostname)
}
