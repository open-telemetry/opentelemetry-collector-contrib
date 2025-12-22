// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package checkpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/multierr"
	"google.golang.org/protobuf/proto"

	pb "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/checkpoint/proto"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

// tryLoadProtobuf attempts to load checkpoint data using protobuf encoding
// Returns error if the data is not valid protobuf
func tryLoadProtobuf(encoded []byte) ([]*reader.Metadata, error) {
	pbList := &pb.MetadataList{}
	if err := proto.Unmarshal(encoded, pbList); err != nil {
		return nil, err
	}

	rmds := make([]*reader.Metadata, 0, len(pbList.Metadata))
	for _, pbMeta := range pbList.Metadata {
		rmd, err := pbToMetadata(pbMeta)
		if err != nil {
			return nil, err
		}
		rmds = append(rmds, rmd)
	}

	return rmds, nil
}

// SaveProto syncs the most recent set of files to the database using protobuf encoding
func SaveProto(ctx context.Context, persister operator.Persister, rmds []*reader.Metadata) error {
	return SaveKeyProto(ctx, persister, rmds, knownFilesKey)
}

func SaveKeyProto(ctx context.Context, persister operator.Persister, rmds []*reader.Metadata, key string, ops ...*storage.Operation) error {
	pbList := &pb.MetadataList{
		Metadata: make([]*pb.Metadata, 0, len(rmds)),
	}

	var errs error
	for _, rmd := range rmds {
		pbMeta, err := metadataToPb(rmd)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("convert metadata to protobuf: %w", err))
			continue
		}
		pbList.Metadata = append(pbList.Metadata, pbMeta)
	}

	data, err := proto.Marshal(pbList)
	if err != nil {
		errs = multierr.Append(errs, fmt.Errorf("marshal protobuf: %w", err))
		return errs
	}

	ops = append(ops, storage.SetOperation(key, data))
	if err := persister.Batch(ctx, ops...); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("persist known files: %w", err))
	}

	return errs
}

// LoadProto loads the most recent set of files from the database using protobuf encoding
func LoadProto(ctx context.Context, persister operator.Persister) ([]*reader.Metadata, error) {
	return LoadKeyProto(ctx, persister, knownFilesKey)
}

func LoadKeyProto(ctx context.Context, persister operator.Persister, key string) ([]*reader.Metadata, error) {
	encoded, err := persister.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if encoded == nil {
		return []*reader.Metadata{}, nil
	}

	pbList := &pb.MetadataList{}
	if err := proto.Unmarshal(encoded, pbList); err != nil {
		return nil, fmt.Errorf("unmarshal protobuf: %w", err)
	}

	rmds := make([]*reader.Metadata, 0, len(pbList.Metadata))
	var errs error
	for _, pbMeta := range pbList.Metadata {
		rmd, err := pbToMetadata(pbMeta)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("convert protobuf to metadata: %w", err))
			continue
		}
		rmds = append(rmds, rmd)
	}

	return rmds, errs
}

// metadataToPb converts reader.Metadata to protobuf Metadata
func metadataToPb(rmd *reader.Metadata) (*pb.Metadata, error) {
	pbMeta := &pb.Metadata{
		Offset:          rmd.Offset,
		RecordNum:       rmd.RecordNum,
		HeaderFinalized: rmd.HeaderFinalized,
		FileType:        rmd.FileType,
	}

	// Convert Fingerprint
	if rmd.Fingerprint != nil {
		// Extract the first bytes from the fingerprint
		fpJSON, err := json.Marshal(rmd.Fingerprint)
		if err != nil {
			return nil, fmt.Errorf("marshal fingerprint: %w", err)
		}
		var fpMap map[string][]byte
		if err := json.Unmarshal(fpJSON, &fpMap); err != nil {
			return nil, fmt.Errorf("unmarshal fingerprint: %w", err)
		}
		pbMeta.Fingerprint = &pb.Fingerprint{
			FirstBytes: fpMap["first_bytes"],
		}
	}

	// Convert FileAttributes - encode the entire map as JSON bytes
	if len(rmd.FileAttributes) > 0 {
		attrBytes, err := json.Marshal(rmd.FileAttributes)
		if err != nil {
			return nil, fmt.Errorf("marshal file attributes: %w", err)
		}
		pbMeta.FileAttributes = attrBytes
	}

	// Convert FlushState
	pbMeta.FlushState = &pb.FlushState{
		LastDataLength: int32(rmd.FlushState.LastDataLength),
	}
	// Only set LastDataChangeUnixNano if the time is not zero
	if !rmd.FlushState.LastDataChange.IsZero() {
		pbMeta.FlushState.LastDataChangeUnixNano = rmd.FlushState.LastDataChange.UnixNano()
	}

	// Convert TokenLenState
	pbMeta.TokenLenState = &pb.TokenLenState{
		MinimumLength: int32(rmd.TokenLenState.MinimumLength),
	}

	return pbMeta, nil
}

// pbToMetadata converts protobuf Metadata to reader.Metadata
func pbToMetadata(pbMeta *pb.Metadata) (*reader.Metadata, error) {
	rmd := &reader.Metadata{
		Offset:          pbMeta.Offset,
		RecordNum:       pbMeta.RecordNum,
		HeaderFinalized: pbMeta.HeaderFinalized,
		FileType:        pbMeta.FileType,
		FileAttributes:  make(map[string]any),
	}

	// Convert Fingerprint
	if pbMeta.Fingerprint != nil && len(pbMeta.Fingerprint.FirstBytes) > 0 {
		rmd.Fingerprint = fingerprint.New(pbMeta.Fingerprint.FirstBytes)
	}

	// Convert FileAttributes - decode from JSON bytes
	if len(pbMeta.FileAttributes) > 0 {
		if err := json.Unmarshal(pbMeta.FileAttributes, &rmd.FileAttributes); err != nil {
			return nil, fmt.Errorf("unmarshal file attributes: %w", err)
		}
	}

	// Convert FlushState
	if pbMeta.FlushState != nil {
		if pbMeta.FlushState.LastDataChangeUnixNano != 0 {
			rmd.FlushState.LastDataChange = time.Unix(0, pbMeta.FlushState.LastDataChangeUnixNano)
		}
		rmd.FlushState.LastDataLength = int(pbMeta.FlushState.LastDataLength)
	}

	// Convert TokenLenState
	if pbMeta.TokenLenState != nil {
		rmd.TokenLenState.MinimumLength = int(pbMeta.TokenLenState.MinimumLength)
	}

	return rmd, nil
}
