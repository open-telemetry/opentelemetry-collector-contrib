// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbatlasreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver"

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"io"
	"regexp"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/model"
)

func decodeLogs(logger *zap.Logger, clusterMajorVersion string, r io.Reader) ([]model.LogEntry, error) {
	switch clusterMajorVersion {
	case mongoDBMajorVersion4_2:
		// 4.2 clusters use a console log format
		return decode4_2(logger.Named("console_decoder"), r)
	default:
		// All other versions use JSON logging
		return decodeJSON(logger.Named("json_decoder"), r)
	}
}

func decodeJSON(logger *zap.Logger, r io.Reader) ([]model.LogEntry, error) {
	// Pass this into a gzip reader for decoding
	gzipReader, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(gzipReader)
	var entries []model.LogEntry
	for {
		if !scanner.Scan() {
			// Scan failed; This might just be EOF, in which case Err will be nil, or it could be some other IO error.
			return entries, scanner.Err()
		}

		var entry model.LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			logger.Error("Failed to parse log entry as JSON", zap.String("entry", scanner.Text()))
			continue
		}

		entry.Raw = scanner.Text()

		entries = append(entries, entry)
	}
}

var mongo4_2LogRegex = regexp.MustCompile(`^(?P<timestamp>\S+)\s+(?P<severity>\w+)\s+(?P<component>[\w-]+)\s+\[(?P<context>\S+)\]\s+(?P<message>.*)$`)

func decode4_2(logger *zap.Logger, r io.Reader) ([]model.LogEntry, error) {
	// Pass this into a gzip reader for decoding
	gzipReader, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(gzipReader)
	var entries []model.LogEntry
	for {
		if !scanner.Scan() {
			// Scan failed; This might just be EOF, in which case Err will be nil, or it could be some other IO error.
			return entries, scanner.Err()
		}

		submatches := mongo4_2LogRegex.FindStringSubmatch(scanner.Text())
		if submatches == nil || len(submatches) != 6 {
			// Match failed for line; We will skip this line and continue processing others.
			logger.Error("Entry did not match regex", zap.String("entry", scanner.Text()))
			continue
		}

		entry := model.LogEntry{
			Timestamp: model.LogTimestamp{
				Date: submatches[1],
			},
			Severity:  submatches[2],
			Component: submatches[3],
			Context:   submatches[4],
			Message:   submatches[5],
			Raw:       submatches[0],
		}

		entries = append(entries, entry)
	}
}

func decodeAuditJSON(logger *zap.Logger, r io.Reader) ([]model.AuditLog, error) {
	// Pass this into a gzip reader for decoding
	gzipReader, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(gzipReader)
	var entries []model.AuditLog
	for {
		if !scanner.Scan() {
			// Scan failed; This might just be EOF, in which case Err will be nil, or it could be some other IO error.
			return entries, scanner.Err()
		}

		var entry model.AuditLog
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			logger.Error("Failed to parse audit log entry as JSON", zap.String("entry", scanner.Text()))
			continue
		}

		entry.Raw = scanner.Text()

		entries = append(entries, entry)
	}
}
