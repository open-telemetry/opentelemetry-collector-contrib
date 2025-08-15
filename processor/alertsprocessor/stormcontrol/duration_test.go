// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stormcontrol

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDurationJSON(t *testing.T) {
	var d Duration
	if err := json.Unmarshal([]byte(`"250ms"`), &d); err != nil {
		t.Fatal(err)
	}
	if time.Duration(d) != 250*time.Millisecond {
		t.Fatalf("got %v", d)
	}
	if err := json.Unmarshal([]byte(`1000000`), &d); err != nil {
		t.Fatal(err)
	}
	if time.Duration(d) != time.Millisecond {
		t.Fatalf("got %v", d)
	}
}

func TestDurationText(t *testing.T) {
	var d Duration
	if err := d.UnmarshalText([]byte("5s")); err != nil {
		t.Fatal(err)
	}
	if d.Duration() != 5*time.Second {
		t.Fatalf("got %v", d)
	}
}
