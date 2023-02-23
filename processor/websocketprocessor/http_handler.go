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

package websocketprocessor

import (
	"net/http"

	"go.uber.org/zap"
	"golang.org/x/net/websocket"
)

type httpHandler struct {
	logger *zap.Logger
	cs     *channelSet
}

func (h *httpHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	h.logger.Debug("got request from", zap.String("remote addr", req.RemoteAddr))
	wssvr := websocket.Server{Handler: h.handleWebsocket}
	wssvr.ServeHTTP(resp, req)
}

func (h *httpHandler) handleWebsocket(conn *websocket.Conn) {
	ch := make(chan []byte)
	idx := h.cs.add(ch)
	for bytes := range ch {
		_, err := conn.Write(bytes)
		if err != nil {
			h.logger.Debug("websocket write error: %w", zap.Error(err))
			h.cs.closeAndRemove(idx)
			break
		}
	}
}
