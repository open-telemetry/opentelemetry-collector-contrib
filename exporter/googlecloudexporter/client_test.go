// Copyright  The OpenTelemetry Authors
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

package googlecloudexporter

import (
	"context"
	"testing"

	logging "cloud.google.com/go/logging/apiv2"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

const (
	loggingScope = "https://www.googleapis.com/auth/logging.write"
)

func TestNewClient(t *testing.T) {
	json := `{"type": "service_account"}`
	credentials, err := google.CredentialsFromJSON(context.Background(), []byte(json), loggingScope)
	require.NoError(t, err)

	options := option.WithCredentials(credentials)
	client, err := newLogClient(context.Background(), options)
	require.NoError(t, err)

	_, ok := client.(*logging.Client)
	require.True(t, ok)
}
