// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package awscredentialsproviderextension

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
)

func TestStartStaticCredentials(t *testing.T) {
	ext := newAWSCredentialsProviderExtension(&Config{
		Credentials: configoptional.Some(CredentialsConfig{
			AccessKeyID:     "AKID",
			SecretAccessKey: "SECRET",
			SessionToken:    "TOKEN",
		}),
	})

	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	provider := ext.GetCredentialsProvider()
	require.NotNil(t, provider)

	creds, err := provider.Retrieve(t.Context())
	require.NoError(t, err)
	require.Equal(t, "AKID", creds.AccessKeyID)
	require.Equal(t, "SECRET", creds.SecretAccessKey)
	require.Equal(t, "TOKEN", creds.SessionToken)

	require.NoError(t, ext.Shutdown(t.Context()))
}

func TestStartAssumeRole(t *testing.T) {
	ext := newAWSCredentialsProviderExtension(&Config{
		Credentials: configoptional.Some(CredentialsConfig{
			AccessKeyID:     "AKID",
			SecretAccessKey: "SECRET",
		}),
		AssumeRole: configoptional.Some(AssumeRoleConfig{
			ARN:         "arn:aws:iam::123456789012:role/monitoring",
			ExternalID:  "my-external-id",
			SessionName: "otel-collector",
			STSRegion:   "us-east-1",
		}),
	})

	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	// Role assumption wraps the base credentials in an STS-backed cache. Retrieving
	// from it would call STS, so only the wiring is asserted here.
	require.IsType(t, &aws.CredentialsCache{}, ext.GetCredentialsProvider())
	require.NoError(t, ext.Shutdown(t.Context()))
}

func TestStartAssumeRoleWithDefaultChainBase(t *testing.T) {
	// assume_role without static credentials: the default chain provides the base
	// identity for the AssumeRole call.
	ext := newAWSCredentialsProviderExtension(&Config{
		AssumeRole: configoptional.Some(AssumeRoleConfig{
			ARN: "arn:aws:iam::123456789012:role/monitoring",
		}),
	})

	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	require.IsType(t, &aws.CredentialsCache{}, ext.GetCredentialsProvider())
	require.NoError(t, ext.Shutdown(t.Context()))
}
