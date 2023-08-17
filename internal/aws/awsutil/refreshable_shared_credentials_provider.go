// Copyright The OpenTelemetry Authors
// Portions of this file Copyright 2018-2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
)

const (
	defaultExpiryWindow = time.Minute * 10
)

type RefreshableSharedCredentialsProvider struct {
	credentials.Expiry
	sharedCredentialsProvider *credentials.SharedCredentialsProvider

	// Retrival frequency, if the value is 15 minutes, the credentials will be retrieved every 15 minutes.
	ExpiryWindow time.Duration
}

// Retrieve reads and extracts the shared credentials from the current
// users home directory.
func (p *RefreshableSharedCredentialsProvider) Retrieve() (credentials.Value, error) {

	if p.ExpiryWindow == 0 {
		p.ExpiryWindow = defaultExpiryWindow
	}
	p.SetExpiration(time.Now().Add(p.ExpiryWindow), 0)
	creds, err := p.sharedCredentialsProvider.Retrieve()
	creds.ProviderName = "RefreshableSharedCredentialsProvider"

	return creds, err
}
