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

// logDecoder is an interface that decodes logs from an io.Reader into LogEntry structs.
type logDecoder interface {
	Decode(r io.Reader) ([]model.LogEntry, error)
}

func decoderForVersion(logger *zap.Logger, clusterMajorVersion string) logDecoder {
	var decoder logDecoder
	switch clusterMajorVersion {
	case mongoDBMajorVersion4_2:
		// 4.2 clusters use a console log format
		decoder = newConsoleLogDecoder(logger.Named("consoledecoder"))
	default:
		// All other versions use JSON logging
		decoder = newJSONLogDecoder(logger.Named("jsondecoder"))
	}

	return decoder
}

// jsonLogDecoder is a logDecoder that decodes JSON formatted mongodb logs.
// This is the format used for mongodb 4.4+
type jsonLogDecoder struct {
	logger *zap.Logger
}

func newJSONLogDecoder(logger *zap.Logger) *jsonLogDecoder {
	return &jsonLogDecoder{
		logger: logger,
	}
}

func (j *jsonLogDecoder) Decode(r io.Reader) ([]model.LogEntry, error) {
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

// consoleLogDecoder is a logDecoder that decodes "console" formatted mongodb logs.
// This is the format used for mongodb 4.2
type consoleLogDecoder struct {
	logger *zap.Logger
}

func newConsoleLogDecoder(logger *zap.Logger) *consoleLogDecoder {
	return &consoleLogDecoder{
		logger: logger,
	}
}

func (c *consoleLogDecoder) Decode(r io.Reader) ([]model.LogEntry, error) {

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
			// Match failed for line
			c.logger.Error("Entry did not match regex", zap.String("entry", scanner.Text()))
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
