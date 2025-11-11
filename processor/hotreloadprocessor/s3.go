// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotreloadprocessor

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"go.opentelemetry.io/collector/otelcol"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/hotreloadprocessor/internal/encryption"
)

type applyConfig func(ctx context.Context, config otelcol.Config, key string) error

// S3Client is an interface that abstracts the s3.Client
type S3Client interface {
	GetObject(
		ctx context.Context,
		params *s3.GetObjectInput,
		optFns ...func(*s3.Options),
	) (*s3.GetObjectOutput, error)
	ListObjectsV2(
		ctx context.Context,
		params *s3.ListObjectsV2Input,
		optFns ...func(*s3.Options),
	) (*s3.ListObjectsV2Output, error)
}

type ListObjectsV2Paginator interface {
	HasMorePages() bool
	NextPage(ctx context.Context, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

type S3Config struct {
	Bucket                 string
	FolderName             string
	Region                 string
	Client                 S3Client
	InsightRefreshDuration time.Duration
	DownloadConcurrency    int
}

type S3Helper struct {
	Config          Config
	Logger          *zap.Logger
	Client          S3Client
	CreatePaginator func(client S3Client, bucket string, key string) ListObjectsV2Paginator
	activeConfig    string
	seenConfigs     map[string]bool
	iterateMu       sync.Mutex
}

func newS3Helper(
	config Config,
	logger *zap.Logger,
	client S3Client,
	createPaginator func(client S3Client, bucket string, key string) ListObjectsV2Paginator,
) *S3Helper {
	return &S3Helper{
		Config:          config,
		Logger:          logger,
		Client:          client,
		CreatePaginator: createPaginator,
		seenConfigs:     make(map[string]bool),
		iterateMu:       sync.Mutex{},
	}
}

func createS3Client(ctx context.Context, region string) (S3Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		return nil, err
	}

	svc := s3.NewFromConfig(cfg)
	return svc, nil
}

func (hp *S3Helper) fetchObject(
	ctx context.Context,
	client S3Client,
	bucket string,
	key string,
) ([]byte, error) {
	objInput := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	result, err := client.GetObject(ctx, objInput)
	if err != nil {
		return nil, fmt.Errorf("failed to get the object from S3 %s: %w", key, err)
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read the body of the S3 object: %s: %w", key, err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			hp.Logger.Warn("Error closing S3 object body", zap.Error(err))
		}
	}(result.Body)

	return body, nil
}

func (hp *S3Helper) decryptObject(
	data []byte,
	key string,
) (otelcol.Config, error) {
	encryptor, err := encryption.NewFileEncryptor(key)
	if err != nil {
		return otelcol.Config{}, fmt.Errorf("failed to create encryptor: %w", err)
	}
	defer encryptor.Close()

	decrypted, err := encryptor.Decrypt(data)
	if err != nil {
		return otelcol.Config{}, fmt.Errorf("failed to decrypt the object: %w", err)
	}

	var config otelcol.Config
	if err := yaml.Unmarshal([]byte(decrypted), &config); err != nil {
		return otelcol.Config{}, fmt.Errorf("failed to unmarshal %T config: %w", config, err)
	}

	return config, nil
}

func (hp *S3Helper) iterateDayLevel(
	ctx context.Context,
	client S3Client,
	bucket string,
	key string,
	applyConfig applyConfig,
) error {
	paginator := hp.CreatePaginator(client, bucket, key)

	contents := []s3types.Object{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to get next page: %w", err)
		}

		contents = append(contents, page.Contents...)
	}

	sort.Slice(contents, func(i, j int) bool {
		return *contents[i].Key > *contents[j].Key
	})

	for _, obj := range contents {
		if hp.activeConfig == *obj.ETag {
			return nil
		}

		if hp.seenConfigs[*obj.Key] {
			continue
		}

		hp.seenConfigs[*obj.Key] = true

		data, err := hp.fetchObject(ctx, client, bucket, *obj.Key)
		if err != nil {
			return fmt.Errorf("failed to fetch object: %w", err)
		}

		config, err := hp.decryptObject(data, hp.Config.EncryptionKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt object: %w", err)
		}

		err = applyConfig(ctx, config, *obj.Key)
		if err != nil {
			hp.Logger.Info(
				"Failed to apply config, trying next one",
				zap.String("key", *obj.Key),
				zap.Error(err),
			)
		} else {
			hp.activeConfig = *obj.ETag
			return nil
		}
	}

	return fmt.Errorf("no valid config found")
}

func (hp *S3Helper) iterateAllDays(
	ctx context.Context,
	client S3Client,
	bucket string,
	key string,
	applyConfig applyConfig,
) error {
	paginator := hp.CreatePaginator(client, bucket, key)

	contents := []s3types.Object{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to get next page: %w", err)
		}

		for _, obj := range page.CommonPrefixes {
			contents = append(contents, s3types.Object{
				Key: obj.Prefix,
			})
		}
	}

	sort.Slice(contents, func(i, j int) bool {
		// sort in descending order of keys which include unix time as last part
		return *contents[i].Key > *contents[j].Key
	})

	for _, obj := range contents {
		err := hp.iterateDayLevel(ctx, client, bucket, *obj.Key, applyConfig)
		if err != nil {
			hp.Logger.Info(
				"Failed to iterate day level",
				zap.String("key", *obj.Key),
				zap.Error(err),
			)
		} else {
			return nil
		}
	}

	return fmt.Errorf("no valid configs found")
}

func (hp *S3Helper) iterateConfigs(ctx context.Context, applyConfig applyConfig) error {
	hp.iterateMu.Lock()
	defer hp.iterateMu.Unlock()

	if strings.HasPrefix(hp.Config.ConfigurationPrefix, "s3://") {
		// s3://bucket/key
		parts := strings.Split(hp.Config.ConfigurationPrefix, "/")
		if len(parts) < 3 {
			return fmt.Errorf("invalid S3 path: %s", hp.Config.ConfigurationPrefix)
		}

		bucket := parts[2]
		key := strings.Join(parts[3:], "/")
		if !strings.HasSuffix(key, "/") {
			key = key + "/"
		}

		return hp.iterateAllDays(ctx, hp.Client, bucket, key, applyConfig)
	} else if strings.HasPrefix(hp.Config.ConfigurationPrefix, "data://") { // for testing
		var config otelcol.Config
		trimmed := strings.TrimPrefix(hp.Config.ConfigurationPrefix, "data://")
		decoded, err := base64.StdEncoding.DecodeString(trimmed)
		if err != nil {
			return fmt.Errorf("failed to decode base64: %w", err)
		}
		if err := yaml.Unmarshal(decoded, &config); err != nil {
			return fmt.Errorf("failed to unmarshal %T config: %w", config, err)
		}
		err = applyConfig(ctx, config, "local-key")
		if err != nil {
			return fmt.Errorf("failed to apply config: %w", err)
		}
		return nil
	}

	return fmt.Errorf(
		"invalid path: %s",
		hp.Config.ConfigurationPrefix,
	)
}
