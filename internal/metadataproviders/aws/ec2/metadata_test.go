// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ec2

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

type ImdsGetMetadataAPI interface {
	GetMetadata(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error)
}

type ImdsInstanceIdentityDocumentAPI interface {
	GetInstanceIdentityDocument(ctx context.Context, params *imds.GetInstanceIdentityDocumentInput, optFns ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error)
}

func GetMetadataFromImds(ctx context.Context, api ImdsGetMetadataAPI, path string) ([]byte, error) {
	output, err := api.GetMetadata(ctx, &imds.GetMetadataInput{
		Path: path,
	})
	if err != nil {
		return nil, err
	}
	defer output.Content.Close()

	return io.ReadAll(output.Content)
}

func GetInstanceIdentityDocumentFromImds(ctx context.Context, api ImdsInstanceIdentityDocumentAPI) (imds.InstanceIdentityDocument, error) {
	output, err := api.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		return imds.InstanceIdentityDocument{}, err
	}

	return output.InstanceIdentityDocument, nil
}

type mockGetMetadataAPI func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error)

func (m mockGetMetadataAPI) GetMetadata(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
	return m(ctx, params, optFns...)
}

type mockInstanceIdentityDocumentAPI func(ctx context.Context, params *imds.GetInstanceIdentityDocumentInput, optFns ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error)

func (m mockInstanceIdentityDocumentAPI) GetInstanceIdentityDocument(ctx context.Context, params *imds.GetInstanceIdentityDocumentInput, optFns ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error) {
	return m(ctx, params, optFns...)
}

func TestGetMetadataFromImds(t *testing.T) {
	cases := []struct {
		name    string
		client  func(t *testing.T) ImdsGetMetadataAPI
		path    string
		expect  []byte
		wantErr bool
	}{
		{
			name: "Successfully retrieves InstanceID metadata",
			client: func(t *testing.T) ImdsGetMetadataAPI {
				return mockGetMetadataAPI(func(_ context.Context, params *imds.GetMetadataInput, _ ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
					t.Helper()
					if e, a := "instance-id", params.Path; e != a {
						t.Errorf("expected Path: %v, got: %v", e, a)
					}
					return &imds.GetMetadataOutput{
						Content: io.NopCloser(bytes.NewReader([]byte("this is the body foo bar baz"))),
					}, nil
				})
			},
			path:    "instance-id",
			expect:  []byte("this is the body foo bar baz"),
			wantErr: false,
		},
		{
			name: "Successfully retrieves Hostname metadata",
			client: func(t *testing.T) ImdsGetMetadataAPI {
				return mockGetMetadataAPI(func(_ context.Context, params *imds.GetMetadataInput, _ ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
					t.Helper()
					if e, a := "hostname", params.Path; e != a {
						t.Errorf("expected Path: %v, got: %v", e, a)
					}
					return &imds.GetMetadataOutput{
						Content: io.NopCloser(bytes.NewReader([]byte("this is the body foo bar baz"))),
					}, nil
				})
			},
			path:    "hostname",
			expect:  []byte("this is the body foo bar baz"),
			wantErr: false,
		},
		{
			name: "Path is empty",
			client: func(t *testing.T) ImdsGetMetadataAPI {
				return mockGetMetadataAPI(func(_ context.Context, params *imds.GetMetadataInput, _ ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
					t.Helper()
					if params.Path == "" {
						return nil, errors.New("Path cannot be empty")
					}
					return nil, nil
				})
			},
			path:    "",
			expect:  nil,
			wantErr: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			content, err := GetMetadataFromImds(ctx, tt.client(t), tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("expected error: %v, got: %v", tt.wantErr, err)
			}
			if !tt.wantErr && !bytes.Equal(tt.expect, content) {
				t.Errorf("expected content: %v, got: %v", string(tt.expect), string(content))
			}
		})
	}
}

func TestInstanceIdentityDocumentFromImds(t *testing.T) {
	cases := []struct {
		name    string
		client  func(t *testing.T) ImdsInstanceIdentityDocumentAPI
		expect  imds.InstanceIdentityDocument
		wantErr bool
	}{
		{
			name: "Successfully retrieves Instance Identity Document",
			client: func(t *testing.T) ImdsInstanceIdentityDocumentAPI {
				return mockInstanceIdentityDocumentAPI(func(_ context.Context, _ *imds.GetInstanceIdentityDocumentInput, _ ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error) {
					t.Helper()
					return &imds.GetInstanceIdentityDocumentOutput{
						InstanceIdentityDocument: imds.InstanceIdentityDocument{
							DevpayProductCodes:      []string{"code1", "code2"},
							MarketplaceProductCodes: []string{"market1"},
							AvailabilityZone:        "us-west-2a",
							PrivateIP:               "192.168.1.1",
							Version:                 "2017-09-30",
							Region:                  "us-west-2",
							InstanceID:              "i-1234567890abcdef0",
							BillingProducts:         []string{"prod1"},
							InstanceType:            "t2.micro",
							AccountID:               "123456789012",
							PendingTime:             time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC),
							ImageID:                 "ami-abcdef1234567890",
							KernelID:                "",
							RamdiskID:               "",
							Architecture:            "x86_64",
						},
					}, nil
				})
			},
			expect: imds.InstanceIdentityDocument{
				DevpayProductCodes:      []string{"code1", "code2"},
				MarketplaceProductCodes: []string{"market1"},
				AvailabilityZone:        "us-west-2a",
				PrivateIP:               "192.168.1.1",
				Version:                 "2017-09-30",
				Region:                  "us-west-2",
				InstanceID:              "i-1234567890abcdef0",
				BillingProducts:         []string{"prod1"},
				InstanceType:            "t2.micro",
				AccountID:               "123456789012",
				PendingTime:             time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC),
				ImageID:                 "ami-abcdef1234567890",
				KernelID:                "",
				RamdiskID:               "",
				Architecture:            "x86_64",
			},
			wantErr: false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			document, err := GetInstanceIdentityDocumentFromImds(ctx, tt.client(t))
			if (err != nil) != tt.wantErr {
				t.Fatalf("expected error: %v, got: %v", tt.wantErr, err)
			}

			if !tt.wantErr {
				if !reflect.DeepEqual(document, tt.expect) {
					t.Errorf("expected document: %+v, got: %+v", tt.expect, document)
				}
			}
		})
	}
}
