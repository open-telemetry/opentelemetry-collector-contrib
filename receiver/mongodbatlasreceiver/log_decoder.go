package mongodbatlasreceiver

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/model"
	"go.uber.org/zap"
)

func decodeLogs(logger *zap.Logger, clusterMajorVersion string, r io.Reader) ([]model.LogEntry, error) {
	switch clusterMajorVersion {
	case mongoDBMajorVersion4_2:
		// 4.2 clusters use a console log format
		return decodeConsole(logger.Named("console_decoder"), r)
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

var consoleLogRegex = regexp.MustCompile(`^(?P<timestamp>\S+)\s+(?P<severity>\w+)\s+(?P<component>[\w-]+)\s+\[(?P<context>\S+)\]\s+(?P<message>.*)$`)

func decodeConsole(logger *zap.Logger, r io.Reader) ([]model.LogEntry, error) {

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

		submatches := consoleLogRegex.FindSubmatch(scanner.Bytes())
		if submatches == nil || len(submatches) != 6 {
			// Match failed for line; We will skip this line and continue processing others.
			logger.Error("Entry did not match regex", zap.String("entry", scanner.Text()))
			continue
		}

		rawLog := string(submatches[0])

		entry := model.LogEntry{
			Timestamp: model.LogTimestamp{
				Date: string(submatches[1]),
			},
			Severity:  string(submatches[2]),
			Component: string(submatches[3]),
			Context:   string(submatches[4]),
			Message:   string(submatches[5]),
			Raw:       &rawLog,
		}

		entries = append(entries, entry)
	}
}
