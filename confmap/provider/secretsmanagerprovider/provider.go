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

package secretsmanagerprovider

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"go.opentelemetry.io/collector/confmap"
)

const (
	schemeName = "secretsmanager"
)

type provider struct {
	client *secretsmanager.Client
}

// New returns a new confmap.Provider that reads the configuration from the given AWS Secrets Manager Name or ARN.
//
// This Provider supports "secretsmanager" scheme, and can be called with a selector:
// `secretsmanager:NAME_OR_ARN`
func New() confmap.Provider {
	return &provider{}
}

func (provider *provider) Retrieve(ctx context.Context, uri string, _ confmap.WatcherFunc) (*confmap.Retrieved, error) {
	if !strings.HasPrefix(uri, schemeName+":") {
		return nil, fmt.Errorf("%q uri is not supported by %q provider", uri, schemeName)
	}

	secretArn := strings.Replace(uri, schemeName+":", "", 1)

	input := &secretsmanager.GetSecretValueInput{
		SecretId: &secretArn,
	}

	response, err := provider.client.GetSecretValue(ctx, input)
	if err != nil {
		return nil, err
	}

	if response.SecretString == nil {
		return nil, nil
	}

	return confmap.NewRetrieved(*response.SecretString)
}

func (*provider) Scheme() string {
	return schemeName
}

func (*provider) Shutdown(context.Context) error {
	return nil
}
