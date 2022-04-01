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

package otto

import (
	"encoding/json"

	"golang.org/x/net/websocket"
)

type startComponentMessage struct {
	PipelineType  string
	ComponentYAML string
}

func readStartComponentMessage(websocketConn *websocket.Conn) startComponentMessage {
	msg := make([]byte, 4096)
	numBytesRead, err := websocketConn.Read(msg)
	if err != nil {
		panic(err)
	}
	out := startComponentMessage{}
	err = json.Unmarshal(msg[:numBytesRead], &out)
	if err != nil {
		panic(err)
	}
	return out
}
