// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
