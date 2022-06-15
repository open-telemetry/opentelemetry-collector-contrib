// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// To generate the protobuf model, the protoc-gen-go package must be installed:
// https://developers.google.com/protocol-buffers/docs/reference/go-generated
// To format the code correctly, we call goimports
// https://pkg.go.dev/golang.org/x/tools/cmd/goimports

//go:generate protoc --go_out=./ --go_opt=paths=import --go_opt=Mreceive_v1.proto=pdata/v1 receive_v1.proto
//go:generate goimports -w pdata/v1/

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"
