// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mongodbatlasreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver"

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
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
		return decodeJSON(r)
	}
}

func decodeJSON(r io.Reader) ([]model.LogEntry, error) {
	// Pass this into a gzip reader for decoding
	reader, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	// Logs are in JSON format so create a JSON decoder to process them
	dec := json.NewDecoder(reader)

	var entries []model.LogEntry
	for {
		var entry model.LogEntry
		err := dec.Decode(&entry)
		if errors.Is(err, io.EOF) {
			return entries, nil
		}
		if err != nil {
			return entries, fmt.Errorf("entry could not be decoded into LogEntry: %w", err)
		}

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
			Raw:       &submatches[0],
		}

		entries = append(entries, entry)
	}
}

func decodeAuditJSON(r io.Reader) ([]model.AuditLog, error) {
	// Pass this into a gzip reader for decoding
	reader, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	// Logs are in JSON format so create a JSON decoder to process them
	dec := json.NewDecoder(reader)

	var entries []model.AuditLog
	for {
		var entry model.AuditLog
		err := dec.Decode(&entry)
		if errors.Is(err, io.EOF) {
			return entries, nil
		}
		if err != nil {
			return entries, fmt.Errorf("entry could not be decoded into AuditLog: %w", err)
		}

		entries = append(entries, entry)
	}
}
