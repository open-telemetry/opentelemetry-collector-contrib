// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package dynatrace provides a detector that loads resource information from
// the dt_host_metadata.properties file which is located in
// the /var/lib/dynatrace/enrichment (on *nix systems) and %ProgramData%\dynatrace\enrichment
// (on Windows) directories.

package dynatrace // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/dynatrace"
import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

const TypeStr = "dynatrace"

const dtHostMetadataProperties = "dt_host_metadata.properties"

var dtHostProperties = []string{"dt.entity.host", "host.name"}

type Detector struct {
	enrichmentDirectory string
	logger              *zap.Logger
}

func NewDetector(set processor.Settings, _ internal.DetectorConfig) (internal.Detector, error) {
	enrichmentDir := "/var/lib/dynatrace/enrichment"
	if runtime.GOOS == "windows" {
		// Windows default is "%ProgramData%\dynatrace\enrichment"
		// If the ProgramData environment variable is not set,
		// it falls back to C:\ProgramData
		programDataDir := os.Getenv("ProgramData")
		if programDataDir == "" {
			programDataDir = `C:\ProgramData`
		}

		enrichmentDir = filepath.Join(programDataDir, "dynatrace", "enrichment")
	}

	return &Detector{
		enrichmentDirectory: enrichmentDir,
		logger:              set.Logger,
	}, nil
}

func (d Detector) Detect(_ context.Context) (pcommon.Resource, string, error) {
	res := pcommon.NewResource()

	if err := d.readPropertiesFile(res.Attributes()); err != nil {
		return res, "", err
	}
	return res, "", nil
}

func (d Detector) readPropertiesFile(attributes pcommon.Map) error {
	filePath := filepath.Join(d.enrichmentDirectory, dtHostMetadataProperties)

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		// split by the first "=" character. If there is another "=" afterward, this will be part of the value
		split := strings.SplitN(line, "=", 2)
		if len(split) != 2 {
			d.logger.Warn("Skipping line as it does not match the expected format of '<key>=<value>", zap.String("line", line))
			continue
		}
		key, value := split[0], split[1]

		if key != "" && value != "" {
			if !slices.Contains(dtHostProperties, key) {
				continue
			}
			attributes.PutStr(strings.TrimSpace(key), strings.TrimSpace(value))
		}
	}

	return scanner.Err()
}
