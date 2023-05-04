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

package handler // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs/handler"

import "github.com/aws/aws-sdk-go/aws/request"

// RequestStructuredLogHandler emf header
var RequestStructuredLogHandler = request.NamedHandler{Name: "RequestStructuredLogHandler", Fn: AddStructuredLogHeader}

// AddStructuredLogHeader emf log
func AddStructuredLogHeader(req *request.Request) {
	req.HTTPRequest.Header.Set("x-amzn-logs-format", "json/emf")
}
