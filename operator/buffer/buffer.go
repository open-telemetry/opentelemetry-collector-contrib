// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package buffer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
)

// Buffer is an interface for an entry buffer
type Buffer interface {
	Add(context.Context, *entry.Entry) error
	Read([]*entry.Entry) (Clearer, int, error)
	ReadWait(context.Context, []*entry.Entry) (Clearer, int, error)
	ReadChunk(context.Context) ([]*entry.Entry, Clearer, error)
	Close() error
}

// Config is a struct that wraps a Builder
type Config struct {
	Builder
}

// NewConfig returns a default Config
func NewConfig() Config {
	return Config{
		Builder: NewMemoryBufferConfig(),
	}
}

// Builder builds a Buffer given build context
type Builder interface {
	Build(context operator.BuildContext, pluginID string) (Buffer, error)
}

// UnmarshalJSON unmarshals JSON
func (bc *Config) UnmarshalJSON(data []byte) error {
	return bc.unmarshal(func(dst interface{}) error {
		return json.Unmarshal(data, dst)
	})
}

// UnmarshalYAML unmarshals YAML
func (bc *Config) UnmarshalYAML(f func(interface{}) error) error {
	return bc.unmarshal(f)
}

func (bc *Config) unmarshal(unmarshal func(interface{}) error) error {
	var m map[string]interface{}
	err := unmarshal(&m)
	if err != nil {
		return err
	}

	switch m["type"] {
	case "memory":
		bc.Builder = NewMemoryBufferConfig()
		return unmarshal(bc.Builder)
	case "disk":
		bc.Builder = NewDiskBufferConfig()
		return unmarshal(bc.Builder)
	default:
		return fmt.Errorf("unknown buffer type '%s'", m["type"])
	}
}

func (bc Config) MarshalYAML() (interface{}, error) {
	return bc.Builder, nil
}

func (bc Config) MarshalJSON() ([]byte, error) {
	return json.Marshal(bc.Builder)
}

type Clearer interface {
	MarkAllAsFlushed() error
	MarkRangeAsFlushed(uint, uint) error
}
