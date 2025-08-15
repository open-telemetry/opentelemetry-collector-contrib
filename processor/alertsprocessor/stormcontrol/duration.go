package stormcontrol // import "github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/stormcontrol"

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Duration is stored as nanoseconds internally.
// It accepts YAML/JSON as string ("250ms") or integer (ns).
type Duration int64 // nanoseconds

func (d Duration) Duration() time.Duration { return time.Duration(d) }
func (d Duration) Nanoseconds() int64      { return int64(d) }
func (d Duration) String() string          { return time.Duration(d).String() }

// MarshalText implements encoding.TextMarshaler.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (d *Duration) UnmarshalText(b []byte) error {
	// Try Go duration string first.
	if dur, err := time.ParseDuration(string(b)); err == nil {
		*d = Duration(dur)
		return nil
	}
	// Fallback: numeric nanoseconds in text.
	if n, err := strconv.ParseInt(string(b), 10, 64); err == nil {
		*d = Duration(time.Duration(n))
		return nil
	}
	return fmt.Errorf("invalid duration %q", string(b))
}

// UnmarshalJSON accepts either "250ms" or 250000000 (ns).
func (d *Duration) UnmarshalJSON(b []byte) error {
	// Quoted? -> string path.
	if len(b) > 0 && b[0] == '"' && b[len(b)-1] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		return d.UnmarshalText([]byte(s))
	}
	// Else numeric nanoseconds.
	var ns int64
	if err := json.Unmarshal(b, &ns); err != nil {
		return fmt.Errorf("duration must be a string or integer nanoseconds: %w", err)
	}
	*d = Duration(time.Duration(ns))
	return nil
}

// UnmarshalYAML supports yaml.Node.Decode without importing yaml here.
type yamlNode interface{ Decode(v any) error }

func (d *Duration) UnmarshalYAML(node yamlNode) error {
	// Try as string.
	var s string
	if err := node.Decode(&s); err == nil {
		return d.UnmarshalText([]byte(s))
	}
	// Try as integer nanoseconds.
	var ns int64
	if err := node.Decode(&ns); err == nil {
		*d = Duration(time.Duration(ns))
		return nil
	}
	var raw any
	_ = node.Decode(&raw)
	return fmt.Errorf("invalid duration YAML value: %T (%v)", raw, raw)
}
