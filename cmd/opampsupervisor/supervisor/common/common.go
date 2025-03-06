// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"io"
	"os"
)

func EnvVarMapToEnvMapSlice(m map[string]string) []string {
	// let the command initialize the env itself
	if m == nil {
		return nil
	}
	result := os.Environ()
	for key, value := range m {
		result = append(result, key+"="+value)
	}
	return result
}

func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	return err
}
