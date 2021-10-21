// Copyright 2020, OpenTelemetry Authors
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

package awsxrayexporter

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client/metadata"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

func TestUserAgent(t *testing.T) {
	logger := zap.NewNop()

	buildInfo := component.BuildInfo{
		Version: "1.0",
	}

	session, _ := session.NewSession()
	xray := newXRay(logger, &aws.Config{}, buildInfo, session)
	x := xray.xRay

	req := request.New(aws.Config{}, metadata.ClientInfo{}, x.Handlers, nil, &request.Operation{
		HTTPMethod: "GET",
		HTTPPath:   "/",
	}, nil, nil)

	x.Handlers.Build.Run(req)
	assert.Contains(t, req.HTTPRequest.UserAgent(), "opentelemetry-collector-contrib/1.0")
}
