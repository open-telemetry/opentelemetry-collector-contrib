package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"gopkg.in/yaml.v3"
)

// marshal the metrics as yaml, putting attributes and datapoints on a single
// line for better readability
func marshal(md pmetric.Metrics) []byte {
	data, err := (&pmetric.JSONMarshaler{}).MarshalMetrics(md)
	if err != nil {
		panic(err)
	}

	var ty map[string]Any
	if err := json.Unmarshal(data, &ty); err != nil {
		panic(err)
	}

	data, err = yaml.Marshal(ty)
	if err != nil {
		panic(err)
	}

	data = bytes.ReplaceAll(data, []byte("'#["), nil)
	data = bytes.ReplaceAll(data, []byte("]#'"), nil)
	return data
}

// Any represents any type.
// It treats attributes and datapoints specially, unmarshalling them into
// [Compact]
type Any struct {
	v any
}

func (any *Any) UnmarshalJSON(data []byte) error {
	var m map[string]Any
	if err := json.Unmarshal(data, &m); err == nil {
		has := has(m)

		// if attribute or datapoint, use compact (single line) representation
		if has("key", "value") || has("asInt") || has("asDouble") {
			any.v = Compact(data)
		} else {
			any.v = m
		}
		return nil
	}

	var a []Any
	if err := json.Unmarshal(data, &a); err == nil {
		any.v = a
		return nil
	}

	return json.Unmarshal(data, &any.v)
}

func (any Any) MarshalYAML() (any, error) {
	return any.v, nil
}

// Compact (JSON-like) representation of a value
type Compact []byte

func (c Compact) MarshalYAML() (any, error) {
	msg := c
	msg = commaExpr.ReplaceAll(msg, []byte(", $1"))
	msg = keyExpr.ReplaceAll(msg, []byte("$1: "))
	return fmt.Sprintf("#[%s]#", msg), nil
}

var (
	commaExpr = regexp.MustCompile(`,("[^"]+":)`)
	keyExpr   = regexp.MustCompile(`"([^"]+)":`)
)

func has[K comparable, V any](m map[K]V) func(...K) bool {
	return func(keys ...K) bool {
		all := true
		for _, k := range keys {
			_, ok := m[k]
			all = all && ok
		}
		return all
	}
}
