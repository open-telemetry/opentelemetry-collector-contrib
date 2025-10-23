// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudstorageexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudstorageexporter"

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"google.golang.org/api/googleapi"
)

type storageExporter struct {
	cfg           *Config
	logsMarshaler plog.Marshaler
	storageClient *storage.Client
	bucketHandle  *storage.BucketHandle
	logger        *zap.Logger
}

var _ exporter.Logs = (*storageExporter)(nil)

func newGCSExporter(
	ctx context.Context,
	cfg *Config,
	logger *zap.Logger,
) (*storageExporter, error) {
	return newStorageExporter(
		ctx,
		cfg,
		metadata.ZoneWithContext,
		metadata.ProjectIDWithContext,
		logger,
	)
}

func newStorageExporter(
	ctx context.Context,
	cfg *Config,
	getZone func(context.Context) (string, error),
	getProjectID func(context.Context) (string, error),
	logger *zap.Logger,
) (*storageExporter, error) {
	errMsg := "failed to determine %s: not set in exporter config '%s' and unable to retrieve from metadata: %w"

	if cfg.Bucket.Region == "" {
		var err error
		if cfg.Bucket.Region, err = getZone(ctx); err != nil {
			return nil, fmt.Errorf(errMsg, "region", "bucket.region", err)
		}
	}

	if cfg.Bucket.ProjectID == "" {
		var err error
		if cfg.Bucket.ProjectID, err = getProjectID(ctx); err != nil {
			return nil, fmt.Errorf(errMsg, "project ID", "bucket.project_id", err)
		}
	}

	return &storageExporter{
		cfg:    cfg,
		logger: logger,
	}, nil
}

func isBucketConflictError(err error) bool {
	var gErr *googleapi.Error
	if !errors.As(err, &gErr) {
		return false
	}
	return gErr.Code == http.StatusConflict
}

func (s *storageExporter) Start(ctx context.Context, host component.Host) error {
	s.logsMarshaler = &plog.JSONMarshaler{}
	if s.cfg.Encoding != nil {
		encoding, err := loadExtension[plog.Marshaler](host, *s.cfg.Encoding, "logs marshaler")
		if err != nil {
			return fmt.Errorf("failed to load logs extension: %w", err)
		}
		s.logsMarshaler = encoding
	}

	// TODO Add option for authenticator
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %w", err)
	}
	err = client.Bucket(s.cfg.Bucket.Name).Create(ctx, s.cfg.Bucket.ProjectID, &storage.BucketAttrs{
		Location: s.cfg.Bucket.Region,
	})
	if err != nil {
		if !s.cfg.Bucket.ReuseIfExists {
			return fmt.Errorf("failed to create storage bucket %q: %w", s.cfg.Bucket.Name, err)
		}
		if !isBucketConflictError(err) {
			return fmt.Errorf("unexpected error creating the storage bucket %q: %w", s.cfg.Bucket.Name, err)
		}
		// otherwise bucket exists and will be reused
		s.logger.Info("Existing bucket will be used", zap.String("bucket", s.cfg.Bucket.Name))
	} else {
		s.logger.Info("Created bucket", zap.String("bucket", s.cfg.Bucket.Name))
	}
	s.bucketHandle = client.Bucket(s.cfg.Bucket.Name)
	s.storageClient = client
	return nil
}

func (s *storageExporter) Shutdown(_ context.Context) error {
	if s.storageClient != nil {
		return s.storageClient.Close()
	}
	return nil
}

func (*storageExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (s *storageExporter) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	buf, err := s.logsMarshaler.MarshalLogs(ld)
	if err != nil {
		return fmt.Errorf("failed to marshal logs: %w", err)
	}

	if err = s.uploadFile(ctx, buf); err != nil {
		return fmt.Errorf("failed to upload logs: %w", err)
	}
	return nil
}

func (s *storageExporter) uploadFile(ctx context.Context, content []byte) (err error) {
	if len(content) == 0 {
		s.logger.Info("No content to upload")
		return nil
	}

	// if we have multiple files coming, we need to make sure the name is unique so they do
	// not overwrite each other
	filename := uuid.New().String()
	if s.cfg.Bucket.FilePrefix != "" {
		filename = s.cfg.Bucket.FilePrefix + "_" + filename
	}
	writer := s.bucketHandle.Object(filename).NewWriter(ctx)
	defer func() {
		err = writer.Close()
		if err != nil {
			s.logger.Error(
				"Failed to close file writer",
				zap.Error(err),
				zap.String("filename", filename),
				zap.String("bucket", s.cfg.Bucket.Name),
			)
		}
	}()
	if _, err = writer.Write(content); err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}
	s.logger.Info(
		"New file uploaded",
		zap.String("filename", filename),
		zap.String("bucket", s.cfg.Bucket.Name),
		zap.Int("size", len(content)),
	)
	return nil
}

// loadExtension tries to load an available extension for the given id.
func loadExtension[T any](host component.Host, id component.ID, extensionType string) (T, error) {
	var zero T
	ext, ok := host.GetExtensions()[id]
	if !ok {
		return zero, fmt.Errorf("unknown extension %q", id)
	}
	extT, ok := ext.(T)
	if !ok {
		return zero, fmt.Errorf("extension %q is not a %s", id, extensionType)
	}
	return extT, nil
}
