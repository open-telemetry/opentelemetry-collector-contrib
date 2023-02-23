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

package websocketviewerextension

import (
	"net/http"

	"go.uber.org/zap"
)

type procInfoHandler struct {
	logger *zap.Logger
	wpr    *wsProcRegistry
}

func (p procInfoHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	bytes, err := p.wpr.toJSON()
	if err != nil {
		p.logger.Error("http handler failed to marshal JSON", zap.Error(err))
	}
	_, err = resp.Write(bytes)
	if err != nil {
		p.logger.Error("http handler failed to write response", zap.Error(err))
	}
}
