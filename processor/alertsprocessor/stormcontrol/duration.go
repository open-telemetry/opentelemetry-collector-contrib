package stormcontrol

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Duration is a wrapper that unmarshals from either a string (e.g. "5s")
// or a number (interpreted as nanoseconds). It marshals back to the
// canonical string form used by time.Duration.
type Duration time.Duration

func (d Duration) Duration() time.Duration { return time.Duration(d) }
func (d Duration) String() string          { return time.Duration(d).String() }

// MarshalText implements encoding.TextMarshaler.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (d *Duration) UnmarshalText(b []byte) error {
	// Try string duration first (e.g., "5s").
	if dur, err := time.ParseDuration(string(b)); err == nil {
		*d = Duration(dur)
		return nil
	}
	// Fallback: numeric (ns) in text form.
	if n, err := strconv.ParseInt(string(b), 10, 64); err == nil {
		*d = Duration(time.Duration(n))
		return nil
	}
	return fmt.Errorf("invalid duration %q", string(b))
}

// UnmarshalJSON accepts either "5s" or 5000000000 (ns).
func (d *Duration) UnmarshalJSON(b []byte) error {
	// If it's quoted, delegate to UnmarshalText.
	if len(b) > 0 && b[0] == '"' && b[len(b)-1] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		return d.UnmarshalText([]byte(s))
	}
	// Else expect a number in nanoseconds.
	var ns int64
	if err := json.Unmarshal(b, &ns); err != nil {
		return fmt.Errorf("duration must be a string or integer nanoseconds: %w", err)
	}
	*d = Duration(time.Duration(ns))
	return nil
}

// UnmarshalYAML supports gopkg.in/yaml.v3 when present.
// The collector's config loader will generally go through confmap/mapstructure,
// but this keeps it robust for direct YAML unmarshals too.
type yamlNode interface {
	Decode(v any) error
}

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
	// Fallback: delegate to text; this will return a good error message.
	var raw any
	_ = node.Decode(&raw)
	return fmt.Errorf("invalid duration YAML value: %T (%v)", raw, raw)
}
