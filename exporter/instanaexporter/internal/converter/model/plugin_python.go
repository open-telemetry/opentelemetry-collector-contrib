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

package model

import (
	"strconv"

	instanaacceptor "github.com/instana/go-sensor/acceptor"
)

type PythonProcessData struct {
	PID           int    `json:"pid"`
	Name          string `json:"snapshot.name"`
	PythonVersion string `json:"snapshot.version"`
	PythonArch    string `json:"snapshot.a"`
	PythonFlavor  string `json:"snapshot.f"`
}

func NewPythonRuntimePlugin(data PythonProcessData) instanaacceptor.PluginPayload {
	const pluginName = "com.instana.plugin.python"

	return instanaacceptor.PluginPayload{
		Name:     pluginName,
		EntityID: strconv.Itoa(data.PID),
		Data:     data,
	}
}
