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

type JVMProcessData struct {
	PID        int    `json:"pid"`
	Name       string `json:"name"`
	JvmName    string `json:"jvm.name"`
	JvmVendor  string `json:"jvm.vendor"`
	JvmVersion string `json:"jvm.version"`
}

func NewJvmRuntimePlugin(data JVMProcessData) instanaacceptor.PluginPayload {
	const pluginName = "com.instana.plugin.java"

	return instanaacceptor.PluginPayload{
		Name:     pluginName,
		EntityID: strconv.Itoa(data.PID),
		Data:     data,
	}
}
