// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotreloadprocessor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/otelcol"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/hotreloadprocessor/internal/encryption"
)

type mockListObjectsV2Paginator struct {
	HasMorePagesFunc func() bool
	NextPageFunc     func(ctx context.Context, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	Objects          []s3types.Object
	CommonPrefixes   []s3types.CommonPrefix
	MaxPageSize      int
}

func (m *mockListObjectsV2Paginator) HasMorePages() bool {
	return len(m.Objects) > 0 || len(m.CommonPrefixes) > 0
}

func (m *mockListObjectsV2Paginator) NextPage(
	_ context.Context,
	_ ...func(*s3.Options),
) (*s3.ListObjectsV2Output, error) {
	if len(m.Objects) > 0 {
		maxPageSize := m.MaxPageSize
		if maxPageSize == 0 {
			maxPageSize = len(m.Objects)
		}
		output := s3.ListObjectsV2Output{
			Contents: m.Objects[0:maxPageSize],
		}
		m.Objects = m.Objects[maxPageSize:]
		return &output, nil
	} else if len(m.CommonPrefixes) > 0 {
		maxPageSize := m.MaxPageSize
		if maxPageSize == 0 {
			maxPageSize = len(m.CommonPrefixes)
		}
		output := s3.ListObjectsV2Output{
			CommonPrefixes: m.CommonPrefixes[0:maxPageSize],
		}
		m.CommonPrefixes = m.CommonPrefixes[maxPageSize:]
		return &output, nil
	} else {
		return nil, nil
	}
}

// mockS3Client is a mock implementation of the S3 client.
type mockS3Client struct {
	s3.Client
	GetObjectFunc     func(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	ListObjectsV2Func func(ctx context.Context, input *s3.ListObjectsV2Input, opts ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

func (m *mockS3Client) GetObject(
	ctx context.Context,
	input *s3.GetObjectInput,
	opts ...func(*s3.Options),
) (*s3.GetObjectOutput, error) {
	if m.GetObjectFunc != nil {
		return m.GetObjectFunc(ctx, input, opts...)
	}
	return nil, nil
}

func (m *mockS3Client) ListObjectsV2(
	ctx context.Context,
	input *s3.ListObjectsV2Input,
	opts ...func(*s3.Options),
) (*s3.ListObjectsV2Output, error) {
	if m.ListObjectsV2Func != nil {
		return m.ListObjectsV2Func(ctx, input, opts...)
	}
	return nil, nil
}

var testKey = "test-1234567890-1234567890-1234567890" // gitleaks:allow

func TestIterateConfigs(t *testing.T) {
	now := time.Now()
	dayAgo := now.Add(time.Hour * -24)
	twoDaysAgo := now.Add(time.Hour * -48)
	threeDaysAgo := now.Add(time.Hour * -72)
	tests := []struct {
		name                string
		config              Config
		getObjectFunc       func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
		listAllDaysFunc     func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
		listDayObjectsFunc  func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
		createPaginatorFunc func(client S3Client, bucket string, key string) ListObjectsV2Paginator
		applyConfigFunc     func(ctx context.Context, config otelcol.Config, key string) error
		expectedError       bool
		wantedKeys          []string
		rerun               bool
	}{
		{
			name: "Choose latest config",
			config: Config{
				ConfigurationPrefix: "s3://test-bucket/test-org-id/prefix-1",
				Region:              "us-east-1",
				EncryptionKey:       testKey,
			},
			getObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				data, err := os.ReadFile(filepath.Join("testdata", "config.yaml"))
				require.NoError(t, err)
				encrypted := encrypt(t, data)
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader(encrypted)),
				}, nil
			},
			createPaginatorFunc: func(client S3Client, bucket string, key string) ListObjectsV2Paginator {
				if key == "test-org-id/prefix-1/" {
					return &mockListObjectsV2Paginator{
						CommonPrefixes: []s3types.CommonPrefix{
							{Prefix: aws.String("test-org-id/prefix-1/1000000000000")},
							{Prefix: aws.String("test-org-id/prefix-1/3000000000000")},
							{Prefix: aws.String("test-org-id/prefix-1/2000000000000")},
						},
					}
				} else if key == "test-org-id/prefix-1/3000000000000" {
					return &mockListObjectsV2Paginator{
						Objects: []s3types.Object{
							{Key: aws.String(fmt.Sprintf("test-org-id/prefix-1/3000000000000/%d.yaml", now.Unix())), LastModified: &now, ETag: aws.String("etag1")},
							{Key: aws.String(fmt.Sprintf("test-org-id/prefix-1/3000000000000/%d.yaml", dayAgo.Unix())), LastModified: &dayAgo, ETag: aws.String("etag2")},
						},
					}
				}
				return nil
			},
			applyConfigFunc: func(ctx context.Context, config otelcol.Config, key string) error {
				if key == fmt.Sprintf("test-org-id/prefix-1/3000000000000/%d.yaml", now.Unix()) {
					return nil
				}
				return fmt.Errorf("wrong config")
			},
			expectedError: false,
			wantedKeys: []string{
				fmt.Sprintf("test-org-id/prefix-1/3000000000000/%d.yaml", now.Unix()),
			},
		},
		{
			name: "Choose before latest config",
			config: Config{
				ConfigurationPrefix: "s3://test-bucket/test-org-id/prefix-1",
				Region:              "us-east-1",
				EncryptionKey:       testKey,
			},
			getObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				data, err := os.ReadFile(filepath.Join("testdata", "config.yaml"))
				require.NoError(t, err)
				encrypted := encrypt(t, data)
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader(encrypted)),
				}, nil
			},
			createPaginatorFunc: func(client S3Client, bucket string, key string) ListObjectsV2Paginator {
				if key == "test-org-id/prefix-1/" {
					return &mockListObjectsV2Paginator{
						CommonPrefixes: []s3types.CommonPrefix{
							{Prefix: aws.String("test-org-id/prefix-1/1000000000000")},
							{Prefix: aws.String("test-org-id/prefix-1/3000000000000")},
							{Prefix: aws.String("test-org-id/prefix-1/2000000000000")},
						},
					}
				} else if key == "test-org-id/prefix-1/3000000000000" {
					return &mockListObjectsV2Paginator{
						Objects: []s3types.Object{
							{Key: aws.String(fmt.Sprintf("test-org-id/prefix-1/3000000000000/%d.yaml", dayAgo.Unix())), LastModified: &dayAgo, ETag: aws.String("etag2")},
							{Key: aws.String(fmt.Sprintf("test-org-id/prefix-1/3000000000000/%d.yaml", now.Unix())), LastModified: &now, ETag: aws.String("etag1")},
						},
					}
				}
				return nil
			},
			applyConfigFunc: func(ctx context.Context, config otelcol.Config, key string) error {
				if key == fmt.Sprintf("test-org-id/prefix-1/3000000000000/%d.yaml", dayAgo.Unix()) {
					return nil
				}
				return fmt.Errorf("wrong config")
			},
			expectedError: false,
			wantedKeys: []string{
				fmt.Sprintf("test-org-id/prefix-1/3000000000000/%d.yaml", now.Unix()),
				fmt.Sprintf("test-org-id/prefix-1/3000000000000/%d.yaml", dayAgo.Unix()),
			},
			rerun: true,
		},
		{
			name: "Choose later day config",
			config: Config{
				ConfigurationPrefix: "s3://test-bucket/test-org-id/prefix-1",
				Region:              "us-east-1",
				EncryptionKey:       testKey,
			},
			getObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				data, err := os.ReadFile(filepath.Join("testdata", "config.yaml"))
				require.NoError(t, err)
				encrypted := encrypt(t, data)
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader(encrypted)),
				}, nil
			},
			createPaginatorFunc: func(client S3Client, bucket string, key string) ListObjectsV2Paginator {
				if key == "test-org-id/prefix-1/" {
					return &mockListObjectsV2Paginator{
						CommonPrefixes: []s3types.CommonPrefix{
							{Prefix: aws.String("test-org-id/prefix-1/1000000000000")},
							{Prefix: aws.String("test-org-id/prefix-1/3000000000000")},
							{Prefix: aws.String("test-org-id/prefix-1/2000000000000")},
						},
						MaxPageSize: 1,
					}
				} else if key == "test-org-id/prefix-1/3000000000000" {
					return &mockListObjectsV2Paginator{
						Objects: []s3types.Object{
							{Key: aws.String(fmt.Sprintf("test-org-id/prefix-1/3000000000000/%d.yaml", now.Unix())), LastModified: &now, ETag: aws.String("etag1")},
							{Key: aws.String(fmt.Sprintf("test-org-id/prefix-1/3000000000000/%d.yaml", dayAgo.Unix())), LastModified: &dayAgo, ETag: aws.String("etag2")},
						},
						MaxPageSize: 1,
					}
				} else if key == "test-org-id/prefix-1/2000000000000" {
					return &mockListObjectsV2Paginator{
						Objects: []s3types.Object{
							{Key: aws.String(fmt.Sprintf("test-org-id/prefix-1/2000000000000/%d.yaml", twoDaysAgo.Unix())), LastModified: &twoDaysAgo, ETag: aws.String("etag3")},
							{Key: aws.String(fmt.Sprintf("test-org-id/prefix-1/2000000000000/%d.yaml", threeDaysAgo.Unix())), LastModified: &threeDaysAgo, ETag: aws.String("etag4")},
						},
					}
				}
				return nil
			},
			applyConfigFunc: func(ctx context.Context, config otelcol.Config, key string) error {
				if key == fmt.Sprintf(
					"test-org-id/prefix-1/2000000000000/%d.yaml",
					twoDaysAgo.Unix(),
				) {
					return nil
				}
				return fmt.Errorf("wrong config")
			},
			expectedError: false,
			wantedKeys: []string{
				fmt.Sprintf("test-org-id/prefix-1/3000000000000/%d.yaml", now.Unix()),
				fmt.Sprintf("test-org-id/prefix-1/3000000000000/%d.yaml", dayAgo.Unix()),
				fmt.Sprintf("test-org-id/prefix-1/2000000000000/%d.yaml", twoDaysAgo.Unix()),
			},
		},
		{
			name: "valid config not found",
			config: Config{
				ConfigurationPrefix: "s3://test-bucket/test-org-id/prefix-1",
				Region:              "us-east-1",
				EncryptionKey:       testKey,
			},
			getObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				data, err := os.ReadFile(filepath.Join("testdata", "config.yaml"))
				require.NoError(t, err)
				encrypted := encrypt(t, data)
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader(encrypted)),
				}, nil
			},
			createPaginatorFunc: func(client S3Client, bucket string, key string) ListObjectsV2Paginator {
				if key == "test-org-id/prefix-1/" {
					return &mockListObjectsV2Paginator{
						CommonPrefixes: []s3types.CommonPrefix{
							{Prefix: aws.String("test-org-id/prefix-1/1000000000000")},
							{Prefix: aws.String("test-org-id/prefix-1/3000000000000")},
							{Prefix: aws.String("test-org-id/prefix-1/2000000000000")},
						},
						MaxPageSize: 1,
					}
				} else if key == "test-org-id/prefix-1/3000000000000" {
					return &mockListObjectsV2Paginator{
						Objects: []s3types.Object{
							{Key: aws.String(fmt.Sprintf("test-org-id/prefix-1/3000000000000/%d.yaml", now.Unix())), LastModified: &now, ETag: aws.String("etag1")},
							{Key: aws.String(fmt.Sprintf("test-org-id/prefix-1/3000000000000/%d.yaml", dayAgo.Unix())), LastModified: &dayAgo, ETag: aws.String("etag2")},
						},
						MaxPageSize: 1,
					}
				} else if key == "test-org-id/prefix-1/2000000000000" {
					return &mockListObjectsV2Paginator{
						Objects: []s3types.Object{
							{Key: aws.String(fmt.Sprintf("test-org-id/prefix-1/2000000000000/%d.yaml", twoDaysAgo.Unix())), LastModified: &twoDaysAgo, ETag: aws.String("etag3")},
							{Key: aws.String(fmt.Sprintf("test-org-id/prefix-1/2000000000000/%d.yaml", threeDaysAgo.Unix())), LastModified: &threeDaysAgo, ETag: aws.String("etag4")},
						},
					}
				} else if key == "test-org-id/prefix-1/1000000000000" {
					return &mockListObjectsV2Paginator{}
				}
				return nil
			},
			applyConfigFunc: func(ctx context.Context, config otelcol.Config, key string) error {
				return fmt.Errorf("wrong config")
			},
			expectedError: true,
			wantedKeys: []string{
				fmt.Sprintf("test-org-id/prefix-1/3000000000000/%d.yaml", now.Unix()),
				fmt.Sprintf("test-org-id/prefix-1/3000000000000/%d.yaml", dayAgo.Unix()),
				fmt.Sprintf("test-org-id/prefix-1/2000000000000/%d.yaml", twoDaysAgo.Unix()),
				fmt.Sprintf("test-org-id/prefix-1/2000000000000/%d.yaml", threeDaysAgo.Unix()),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockS3Client{
				GetObjectFunc:     tt.getObjectFunc,
				ListObjectsV2Func: tt.listAllDaysFunc,
			}
			hp := newS3Helper(tt.config, zap.NewNop(), mockClient, tt.createPaginatorFunc)
			keys := []string{}
			reruns := 1
			if tt.rerun {
				reruns = 2
			}
			var err error
			for range reruns {
				err = hp.iterateConfigs(
					context.Background(),
					func(ctx context.Context, config otelcol.Config, key string) error {
						keys = append(keys, key)
						err := tt.applyConfigFunc(ctx, config, key)
						if err != nil {
							return err
						}
						return nil
					},
				)
			}
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, len(tt.wantedKeys), len(keys))
				for i, wantedKey := range tt.wantedKeys {
					require.EqualValues(t, wantedKey, keys[i])
				}
			}
		})
	}
}

func encrypt(t *testing.T, data []byte) []byte {
	encryptor, err := encryption.NewFileEncryptor(testKey)
	require.NoError(t, err)
	defer encryptor.Close()
	encrypted, err := encryptor.Encrypt(data)
	require.NoError(t, err)
	return encrypted
}
