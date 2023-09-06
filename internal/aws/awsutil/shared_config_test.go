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

package awsutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFallbackSharedConfigFiles(t *testing.T) {
	noOpGetUserHomeDir := func() string { return "home" }
	t.Setenv(envAwsSdkLoadConfig, "true")
	t.Setenv(envAwsSharedCredentialsFile, "credentials")
	t.Setenv(envAwsSharedConfigFile, "config")

	got := getFallbackSharedConfigFiles(noOpGetUserHomeDir)
	assert.Equal(t, []string{"config", "credentials"}, got)

	t.Setenv(envAwsSdkLoadConfig, "false")
	got = getFallbackSharedConfigFiles(noOpGetUserHomeDir)
	assert.Equal(t, []string{"credentials"}, got)

	t.Setenv(envAwsSdkLoadConfig, "true")
	t.Setenv(envAwsSharedCredentialsFile, "")
	t.Setenv(envAwsSharedConfigFile, "")

	got = getFallbackSharedConfigFiles(noOpGetUserHomeDir)
	assert.Equal(t, []string{defaultSharedConfig("home"), defaultSharedCredentialsFile("home")}, got)
}
