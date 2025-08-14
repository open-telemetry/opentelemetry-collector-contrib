// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nova

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"
)

// mockHTTPDoer allows us to fake HTTP responses from the Nova metadata service.
type mockHTTPDoer func(*http.Request) (*http.Response, error)

func (m mockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	return m(req)
}

func TestGetNovaMetadataDocument(t *testing.T) {
	dummyBody := `{
		"uuid": "12345678-abcd-ef00-1234-56789abcdef0",
		"meta": {
			"key1": "value1",
			"key2": "value2"
		},
		"hostname": "test-vm-01",
		"name": "test-vm-01",
		"availability_zone": "zone-a",
		"project_id": "proj-123456",
		"launch_index": 0
	}`

	tests := []struct {
		name      string
		doer      mockHTTPDoer
		want      Document
		expectErr bool
	}{
		{
			name: "successfully retrieves Nova metadata document",
			doer: func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(dummyBody)),
				}, nil
			},
			want: Document{
				UUID:             "12345678-abcd-ef00-1234-56789abcdef0",
				Meta:             map[string]string{"key1": "value1", "key2": "value2"},
				Hostname:         "test-vm-01",
				Name:             "test-vm-01",
				ProjectID:        "proj-123456",
				AvailabilityZone: "zone-a",
			},
		},
		{
			name: "http error",
			doer: func(_ *http.Request) (*http.Response, error) {
				return nil, errors.New("connection failed")
			},
			expectErr: true,
		},
		{
			name: "non-200 status",
			doer: func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(bytes.NewBufferString("fail")),
				}, nil
			},
			expectErr: true,
		},
		{
			name: "invalid json",
			doer: func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString("{not-json")),
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
					Transport: roundTripperFunc(tt.doer),
				},
			}

			doc, err := p.Get(t.Context())
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(doc, tt.want) {
				t.Fatalf("document mismatch\n got: %+v\nwant: %+v", doc, tt.want)
			}
		})
	}
}

func TestNovaProviderAccessors(t *testing.T) {
	dummyBody := `{
		"uuid": "abcd-1234",
		"hostname": "vm-accessor-test",
		"name": "vm-accessor-test",
		"project_id": "proj-xyz"
	}`

	p := &metadataClient{
		client: &http.Client{
			Timeout: 1 * time.Second,
			Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(dummyBody)),
				}, nil
			}),
		},
	}

	ctx := t.Context()

	// InstanceID
	id, err := p.InstanceID(ctx)
	if err != nil {
		t.Fatalf("unexpected error in InstanceID: %v", err)
	}
	if id != "abcd-1234" {
		t.Fatalf("InstanceID mismatch: got %q, want %q", id, "abcd-1234")
	}

	// Hostname
	hn, err := p.Hostname(ctx)
	if err != nil {
		t.Fatalf("unexpected error in Hostname: %v", err)
	}
	if hn != "vm-accessor-test" {
		t.Fatalf("Hostname mismatch: got %q, want %q", hn, "vm-accessor-test")
	}
}

func TestNovaInstanceType(t *testing.T) {
	tests := []struct {
		name      string
		doer      roundTripperFunc
		want      string
		expectErr bool
	}{
		{
			name: "successfully retrieves instance type",
			doer: func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != ec2MetaBaseURL+"instance-type" {
					t.Fatalf("expected URL %s, got %s", ec2MetaBaseURL+"instance-type", req.URL.String())
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString("t3.small")),
				}, nil
			},
			want:      "t3.small",
			expectErr: false,
		},
		{
			name: "returns error on HTTP failure",
			doer: func(_ *http.Request) (*http.Response, error) {
				return nil, errors.New("connection failed")
			},
			expectErr: true,
		},
		{
			name: "returns error on non-200 status",
			doer: func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(bytes.NewBufferString("not found")),
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

			got, err := p.InstanceType(t.Context())
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Fatalf("InstanceType mismatch: got %q, want %q", got, tt.want)
			}
		})
	}
}

// helper to implement http.RoundTripper from a function
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
