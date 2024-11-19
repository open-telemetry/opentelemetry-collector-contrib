// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testar

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testar/crlf"
)

func ExampleRead() {
	data := []byte(`
-- foo --
hello

-- bar --
world
`)

	var into struct {
		Foo string `testar:"foo"`
		Bar []byte `testar:"bar"`
	}

	_ = Read(data, &into)
	fmt.Printf("foo: %T(%q)\n", into.Foo, into.Foo)
	fmt.Printf("bar: %T(%q)\n", into.Bar, into.Bar)

	// Output:
	// foo: string("hello\n\n")
	// bar: []uint8("world\n")
}

func ExampleParser() {
	data := []byte(`
-- foobar --
377927
`)

	var into struct {
		Foobar int `testar:"foobar,atoi"`
	}

	_ = Read(data, &into, Parser("atoi", func(file []byte, into *int) error {
		n, err := strconv.Atoi(strings.TrimSpace(string(file)))
		if err != nil {
			return err
		}
		*into = n
		return nil
	}))

	fmt.Printf("foobar: %T(%d)\n", into.Foobar, into.Foobar)

	// Output:
	// foobar: int(377927)
}

func TestCRLF(t *testing.T) {
	data := crlf.Join(
		"-- string --",
		"foobar",
	)

	var into struct {
		String string `testar:"string"`
	}

	err := Read(data, &into)
	if err != nil {
		t.Fatal(err)
	}

	must(t, into.String, "foobar\n")
}

func must[T string | int](t *testing.T, v, want T) {
	t.Helper()
	if v != want {
		t.Fatalf("got '%q' != '%q' want", v, want)
	}
}
