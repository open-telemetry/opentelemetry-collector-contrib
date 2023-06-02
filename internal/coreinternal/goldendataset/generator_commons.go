// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package goldendataset // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/goldendataset"

import (
	"encoding/csv"
	"os"
	"path/filepath"
)

func loadPictOutputFile(fileName string) ([][]string, error) {
	file, err := os.Open(filepath.Clean(fileName))
	if err != nil {
		return nil, err
	}
	defer func() {
		cerr := file.Close()
		if err == nil {
			err = cerr
		}
	}()

	reader := csv.NewReader(file)
	reader.Comma = '\t'

	return reader.ReadAll()
}
