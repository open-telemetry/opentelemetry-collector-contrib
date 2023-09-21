// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

const (
	envAwsSdkLoadConfig = "AWS_SDK_LOAD_CONFIG"
	// nolint:gosec
	envAwsSharedCredentialsFile = "AWS_SHARED_CREDENTIALS_FILE"
	envAwsSharedConfigFile      = "AWS_CONFIG_FILE"
)

// getFallbackSharedConfigFiles follows the same logic as the AWS SDK but takes a getUserHomeDir
// function.
func getFallbackSharedConfigFiles(userHomeDirProvider func() string) []string {
	var sharedCredentialsFile, sharedConfigFile string
	setFromEnvVal(&sharedCredentialsFile, envAwsSharedCredentialsFile)
	setFromEnvVal(&sharedConfigFile, envAwsSharedConfigFile)
	if sharedCredentialsFile == "" {
		sharedCredentialsFile = defaultSharedCredentialsFile(userHomeDirProvider())
	}
	if sharedConfigFile == "" {
		sharedConfigFile = defaultSharedConfig(userHomeDirProvider())
	}
	var cfgFiles []string
	enableSharedConfig, _ := strconv.ParseBool(os.Getenv(envAwsSdkLoadConfig))
	if enableSharedConfig {
		cfgFiles = append(cfgFiles, sharedConfigFile)
	}
	return append(cfgFiles, sharedCredentialsFile)
}

func setFromEnvVal(dst *string, keys ...string) {
	for _, k := range keys {
		if v := os.Getenv(k); len(v) != 0 {
			*dst = v
			break
		}
	}
}

func defaultSharedCredentialsFile(dir string) string {
	return filepath.Join(dir, ".aws", "credentials")
}

func defaultSharedConfig(dir string) string {
	return filepath.Join(dir, ".aws", "config")
}

// backwardsCompatibleUserHomeDir provides the home directory based on
// environment variables.
//
// Based on v1.44.106 of the AWS SDK.
func backwardsCompatibleUserHomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

// currentUserHomeDir attempts to use the environment variables before falling
// back on the current user's home directory.
//
// Based on v1.44.332 of the AWS SDK.
func currentUserHomeDir() string {
	var home string

	home = backwardsCompatibleUserHomeDir()
	if len(home) > 0 {
		return home
	}

	currUser, _ := user.Current()
	if currUser != nil {
		home = currUser.HomeDir
	}

	return home
}
