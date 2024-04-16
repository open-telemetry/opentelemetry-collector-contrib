// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type valueSource interface {
	getValue() (string, error)
}

type rawSource struct {
	value string
}

func (v rawSource) getValue() (string, error) {
	return v.value, nil
}

func readCredentialsFile(path string) (string, error) {
	f, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read credentials file %q: %w", path, err)
	}

	credential := strings.TrimSpace(string(f))
	if credential == "" {
		return "", fmt.Errorf("empty credentials file %q", path)
	}
	return credential, nil
}

type fileSource struct {
	path string
}

func (v fileSource) getValue() (string, error) {
	return readCredentialsFile(v.path)
}

func executeCommand(cmdArgs ...string) (string, error) {
	if len(cmdArgs) == 0 {
		return "", fmt.Errorf("empty command provided")
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...) // #nosec G204
	outBytes, err := cmd.Output()
	if err != nil {
		return "", err
	}

	output := strings.TrimSpace(string(outBytes))
	if output == "" {
		return "", fmt.Errorf("command %q returned empty response", strings.Join(cmdArgs, " "))
	}

	return output, nil
}

type cmdSource struct {
	cmd []string
}

func (v cmdSource) getValue() (string, error) {
	return executeCommand(v.cmd...)
}
