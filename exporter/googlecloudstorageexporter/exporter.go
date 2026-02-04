// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudstorageexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudstorageexporter"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	"github.com/lestrrat-go/strftime"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"google.golang.org/api/googleapi"
)

type signalType string

const (
	signalTypeLogs   signalType = "logs"
	signalTypeTraces signalType = "traces"
)

var (
	errNotLogsMarshaler   = errors.New("extension is not a logs marshaler")
	errNotTracesMarshaler = errors.New("extension is not a traces marshaler")
)

var validSignals = []signalType{signalTypeLogs, signalTypeTraces}

type storageExporter struct {
	cfg             *Config
	logsMarshaler   plog.Marshaler
	tracesMarshaler ptrace.Marshaler
	storageClient   *storage.Client
	bucketHandle    *storage.BucketHandle
	logger          *zap.Logger
	partitionFormat *strftime.Strftime
	signal          signalType
}

var (
	_ exporter.Logs   = (*storageExporter)(nil)
	_ exporter.Traces = (*storageExporter)(nil)
)

func newGCSExporter(
	ctx context.Context,
	cfg *Config,
	logger *zap.Logger,
	signal signalType,
) (*storageExporter, error) {
	return newStorageExporter(
		ctx,
		cfg,
		metadata.ZoneWithContext,
		metadata.ProjectIDWithContext,
		logger,
		signal,
	)
}

func newStorageExporter(
	ctx context.Context,
	cfg *Config,
	getZone func(context.Context) (string, error),
	getProjectID func(context.Context) (string, error),
	logger *zap.Logger,
	signal signalType,
) (*storageExporter, error) {
	// Validate signal type
	switch signal {
	case signalTypeLogs, signalTypeTraces: // valid
	default:
		return nil, fmt.Errorf("signal type %q not recognized, valid values are %v", signal, validSignals)
	}

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

	var partitionFormat *strftime.Strftime
	if cfg.Bucket.Partition.Format != "" {
		var err error
		partitionFormat, err = strftime.New(cfg.Bucket.Partition.Format)
		if err != nil {
			// should not happen here, prevented by config.Validate
			return nil, fmt.Errorf("failed to parse partition format: %w", err)
		}
	}

	return &storageExporter{
		cfg:             cfg,
		logger:          logger,
		partitionFormat: partitionFormat,
		signal:          signal,
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
	// Initialize default marshalers
	s.logsMarshaler = &plog.JSONMarshaler{}
	s.tracesMarshaler = &ptrace.JSONMarshaler{}

	// Load encoding extension if configured
	if s.cfg.Encoding != nil {
		switch s.signal {
		case signalTypeLogs:
			logsEncoding, err := loadExtension[plog.Marshaler](host, *s.cfg.Encoding, "logs marshaler", errNotLogsMarshaler)
			if err != nil {
				return fmt.Errorf("failed to load logs extension: %w", err)
			}
			s.logsMarshaler = logsEncoding
		case signalTypeTraces:
			tracesEncoding, err := loadExtension[ptrace.Marshaler](host, *s.cfg.Encoding, "traces marshaler", errNotTracesMarshaler)
			if err != nil {
				if !errors.Is(err, errNotTracesMarshaler) {
					return fmt.Errorf("failed to load traces extension: %w", err)
				}
				s.logger.Warn("Configured encoding extension does not support traces, falling back to JSON marshaler", zap.String("encoding", s.cfg.Encoding.String()))
			} else {
				s.tracesMarshaler = tracesEncoding
			}
		}
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

func (s *storageExporter) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	buf, err := s.tracesMarshaler.MarshalTraces(td)
	if err != nil {
		return fmt.Errorf("failed to marshal traces: %w", err)
	}

	if err = s.uploadFile(ctx, buf); err != nil {
		return fmt.Errorf("failed to upload traces: %w", err)
	}
	return nil
}

// generateFilename returns the name of the file to be uploaded.
// It starts from a unique ID, and prepends the partitionFormat and the prefix to it.
func generateFilename(
	uniqueID, filePrefix, partitionPrefix string,
	partitionFormat *strftime.Strftime,
	now time.Time,
) string {
	filename := uniqueID
	if filePrefix != "" {
		if strings.HasSuffix(filePrefix, "/") {
			filename = filePrefix + uniqueID
		} else {
			filename = filePrefix + "_" + uniqueID
		}
	}

	if partitionFormat != nil {
		partition := partitionFormat.FormatString(now)
		if !strings.HasSuffix(partition, "/") {
			partition += "/"
		}
		filename = partition + filename
	}

	if partitionPrefix != "" {
		if !strings.HasSuffix(partitionPrefix, "/") {
			partitionPrefix += "/"
		}
		filename = partitionPrefix + filename
	}
	return filename
}

func (s *storageExporter) uploadFile(ctx context.Context, content []byte) (err error) {
	if len(content) == 0 {
		s.logger.Info("No content to upload")
		return nil
	}

	// if we have multiple files coming, we need to make sure the name is unique so they do
	// not overwrite each other
	uniqueID := uuid.New().String()
	filename := generateFilename(
		uniqueID,
		s.cfg.Bucket.FilePrefix,
		s.cfg.Bucket.Partition.Prefix,
		s.partitionFormat,
		time.Now().UTC(),
	)

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
func loadExtension[T any](host component.Host, id component.ID, _ string, errNotMarshaler error) (T, error) {
	var zero T
	ext, ok := host.GetExtensions()[id]
	if !ok {
		return zero, fmt.Errorf("unknown extension %q", id)
	}
	extT, ok := ext.(T)
	if !ok {
		return zero, fmt.Errorf("extension %q: %w", id, errNotMarshaler)
	}
	return extT, nil
}
