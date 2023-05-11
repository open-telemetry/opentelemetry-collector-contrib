// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awss3exporter

import (
	"regexp"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestS3TimeKey(t *testing.T) {
	const layout = "2006-01-02"

	tm, err := time.Parse(layout, "2022-06-05")
	timeKey := getTimeKey(tm, "hour")

	assert.NoError(t, err)
	require.NotNil(t, tm)
	assert.Equal(t, "year=2022/month=06/day=05/hour=00", timeKey)

	timeKey = getTimeKey(tm, "minute")
	assert.Equal(t, "year=2022/month=06/day=05/hour=00/minute=00", timeKey)
}

func TestS3Key(t *testing.T) {
	const layout = "2006-01-02"

	tm, err := time.Parse(layout, "2022-06-05")

	assert.NoError(t, err)
	require.NotNil(t, tm)

	re := regexp.MustCompile(`keyprefix/year=2022/month=06/day=05/hour=00/minute=00/fileprefixlogs_([0-9]+).json`)
	s3Key := getS3Key(tm, "keyprefix", "minute", "fileprefix", "logs", "json")
	matched := re.MatchString(s3Key)
	assert.Equal(t, true, matched)
}

func TestGetSessionConfigWithEndpoint(t *testing.T) {
	const endpoint = "https://endpoint.com"
	const region = "region"
	config := &Config{
		S3Uploader: S3UploaderConfig{
			Region:   region,
			Endpoint: endpoint,
		},
	}
	sessionConfig := getSessionConfig(config)
	assert.Equal(t, sessionConfig.Endpoint, aws.String(endpoint))
	assert.Equal(t, sessionConfig.Region, aws.String(region))
}

func TestGetSessionConfigNoEndpoint(t *testing.T) {
	const region = "region"
	config := &Config{
		S3Uploader: S3UploaderConfig{
			Region: region,
		},
	}
	sessionConfig := getSessionConfig(config)
	assert.Empty(t, sessionConfig.Endpoint)
	assert.Equal(t, sessionConfig.Region, aws.String(region))
}
