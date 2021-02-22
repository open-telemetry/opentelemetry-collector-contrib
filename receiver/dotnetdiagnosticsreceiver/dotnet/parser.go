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

package dotnet

import (
	"io"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/network"
)

// Parser encapsulates all of the functionality to parse an IPC stream.
type Parser struct {
	r      network.MultiReader
	logger *zap.Logger
}

// NewParser accepts an io.Reader and logger returns a Parser for processing an
// IPC stream.
func NewParser(rdr io.Reader, logger *zap.Logger) *Parser {
	r := network.NewMultiReader(rdr)
	return &Parser{r: r, logger: logger}
}

// ParseIPC parses the IPC response from the initial request to a dotnet process.
func (p *Parser) ParseIPC() error {
	return parseIPC(p.r)
}

// ParseNettrace parses the nettrace magic message.
func (p *Parser) ParseNettrace() error {
	return parseNettrace(p.r)
}
