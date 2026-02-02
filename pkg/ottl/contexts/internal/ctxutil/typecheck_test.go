// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxutil

import "testing"

func TestExpectTypeSuccess(t *testing.T) {
	got, err := ExpectType[int](42)
	if err != nil {
		t.Fatalf("expect success, got error %v", err)
	}
	if got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
}

func TestExpectTypeError(t *testing.T) {
	_, err := ExpectType[string](123)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
