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
	"go.opentelemetry.io/collector/component"
	"golang.org/x/time/rate"
)

const defaultPort = 12001

type Config struct {
	// Port indicates the port used by the web socket listener started by this processor.
	// Defaults to 12001.
	Port int `mapstructure:"port"`
	// Limit is a float that indicates the maximum number of messages repeated
	// through the websocket by this processor in messages per second.
	Limit rate.Limit `mapstructure:"limit"`
}

func createDefaultConfig() component.Config {
	return &Config{
		Port:  defaultPort,
		Limit: 1,
	}
}
