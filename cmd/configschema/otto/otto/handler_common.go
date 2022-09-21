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
	"log"

	"go.opentelemetry.io/collector/confmap"
	"golang.org/x/net/websocket"
	"gopkg.in/yaml.v2"
)

func sendErr(ws *websocket.Conn, logger *log.Logger, msg string, err error) {
	envelopeJson, jsonErr := json.Marshal(wsMessageEnvelope{Error: err})
	if jsonErr != nil {
		const fmt = "%s due to %v. also failed to marshal envelope containing the error due to %v"
		logger.Fatalf(fmt, msg, err, jsonErr)
	}
	_, err = ws.Write(envelopeJson)
}

func readSocket(ws *websocket.Conn) (string, *confmap.Conf, error) {
	msg, err := readStartComponentMessage(ws)
	if err != nil {
		return "", nil, err
	}
	m := map[string]interface{}{}
	err = yaml.Unmarshal([]byte(msg.ComponentYAML), &m)
	if err != nil {
		return "", nil, err
	}
	return msg.PipelineType, confmap.NewFromStringMap(m), nil
}
