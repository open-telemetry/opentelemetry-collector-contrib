// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package dynatrace provides a detector that loads resource information from
// the dt_host_metadata.json and dt_host_metadata.properties files which are located in
// the /var/lib/dynatrace/enrichment (on *nix systems) and %ProgramData%\dynatrace\enrichment
// (on Windows) directories.

package dynatrace // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/dynatrace"
import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

const TypeStr = "dynatrace"

const dtHostMetadataJSON = "dt_host_metadata.json"
const dtHostMetadataProperties = "dt_host_metadata.properties"

type Detector struct {
	enrichmentDirectory string
}

func NewDetector(_ processor.Settings, _ internal.DetectorConfig) (internal.Detector, error) {
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
	}, nil
}

func (d Detector) Detect(_ context.Context) (pcommon.Resource, string, error) {
	res := pcommon.NewResource()

	if err := d.readPropertiesJSON(res.Attributes()); err != nil {
		return res, "", err
	}

	if err := d.readPropertiesFile(res.Attributes()); err != nil {
		return res, "", err
	}
	return res, "", nil
}

func (d Detector) readPropertiesJSON(attributes pcommon.Map) error {
	filePath := filepath.Join(d.enrichmentDirectory, dtHostMetadataJSON)

	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	m := map[string]any{}

	if err := json.Unmarshal(fileContent, &m); err != nil {
		return err
	}

	for key, value := range m {
		stringVal, ok := value.(string)
		if !ok {
			continue
		}
		attributes.PutStr(key, stringVal)
	}
	return nil
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
		// split by the first "=" character. If there is another "=" afterward, this will be part of the value
		split := strings.SplitN(scanner.Text(), "=", 2)
		if len(split) != 2 {
			continue
		}
		key, value := split[0], split[1]

		if key != "" && value != "" {
			attributes.PutStr(strings.TrimSpace(key), strings.TrimSpace(value))
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}
