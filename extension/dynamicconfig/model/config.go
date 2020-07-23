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

// Package model contains data structures intended to be in-memory
// representations of config information passed to a file backend. The data
// from a "schedules.yaml" is parsed directly into the data structures.
// The data structures include utilities for hashing and compiling their
// data into protobufs.
package model

import (
	"fmt"
	"strings"

	com "github.com/open-telemetry/opentelemetry-proto/gen/go/common/v1"
	res "github.com/open-telemetry/opentelemetry-proto/gen/go/resource/v1"
)

// A Config collects together all the ConfigBlocks specified in a schedules.yaml.
// It has the ability to generate a new ConfigBlock that contains the relevant
// configuration data matching a particular resource (see Match).
type Config struct {
	ConfigBlocks []*ConfigBlock
}

// Given a resource, Match will compile a config block that contains the
// relevant configs. If all resource labels in a config block match a key-value
// pair in the given resource, then the configs from this block will be included
// in the returned config block.  If a config block specifies no resource
// labels, then it will be included in all matches. In this way, a user may
// specify default configs to be included for all resources.
func (config *Config) Match(resource *res.Resource) *ConfigBlock {
	labelSet, labelList := embed(resource)
	totalBlock := &ConfigBlock{
		Resource: labelList,
	}

	for _, block := range config.ConfigBlocks {
		if doInclude(block, labelSet) {
			totalBlock.Add(block)
		}
	}

	return totalBlock
}

func embed(resource *res.Resource) (map[string]bool, []string) {
	if resource == nil {
		resource = &res.Resource{}
	}

	labelSet := make(map[string]bool)
	labelList := make([]string, len(resource.Attributes))

	for i, attr := range resource.Attributes {
		attrString := attrToString(attr)
		labelSet[attrString] = true
		labelList[i] = attrString
	}

	return labelSet, labelList
}

func attrToString(attr *com.KeyValue) string {
	rawValue := attr.Value.String()
	value := strings.Split(rawValue, ":")[1]
	attrString := clean(fmt.Sprintf("%s:%s", attr.Key, value))

	return attrString
}

func doInclude(block *ConfigBlock, labelSet map[string]bool) bool {
	include := true
	for _, label := range block.Resource {
		label = clean(label)
		include = include && labelSet[label]
	}

	return include
}

func clean(label string) string {
	label = strings.ReplaceAll(label, " ", "")
	label = strings.ReplaceAll(label, `"`, "")

	return label
}
