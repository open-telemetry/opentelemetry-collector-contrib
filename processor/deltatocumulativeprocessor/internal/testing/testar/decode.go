// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// testar is a textual archive (based on [golang.org/x/tools/txtar]) to define
// test fixtures.
//
// Archive data is read into struct fields, optionally calling parsers for field
// types other than string or []byte:
//
//	type T struct {
//		Literal string `testar:"file1"`
//		Parsed  int    `testar:"file2,myparser"`
//	}
//
//	var into T
//	err := Read(data, &into)
//
// See [Read] and [Parser] for examples.
package testar // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testing/testar"

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"strings"

	"golang.org/x/tools/txtar"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testing/testar/crlf"
)

// Read archive data into the fields of struct *T
func Read[T any](data []byte, into *T, parsers ...Format) error {
	data = crlf.Strip(data)
	ar := txtar.Parse(data)
	return Decode(ar, into, parsers...)
}

func ReadFile[T any](file string, into *T, parsers ...Format) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	return Read(data, into, parsers...)
}

func Decode[T any](ar *txtar.Archive, into *T, parsers ...Format) error {
	arfs, err := txtar.FS(ar)
	if err != nil {
		return err
	}

	pv := reflect.ValueOf(into)
	if pv.Kind() != reflect.Pointer {
		return errors.New("into must be pointer")
	}
	sv := pv.Elem()

	for i := range sv.NumField() {
		f := sv.Type().Field(i)
		tag := f.Tag.Get("testar")
		if tag == "" {
			continue
		}

		name, format, _ := strings.Cut(tag, ",")
		data, err := fs.ReadFile(arfs, name)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}

		err = formats(parsers).Parse(format, data, sv.Field(i).Addr().Interface())
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

type formats []Format

func (fmts formats) Parse(name string, data []byte, into any) error {
	if name == "" {
		return LiteralParser(data, into)
	}

	for _, f := range fmts {
		if f.name == name {
			return f.parse(data, into)
		}
	}
	return fmt.Errorf("no such format: %q", name)
}

type Format struct {
	name  string
	parse func(file []byte, into any) error
}

func Parser[T any](name string, fn func([]byte, *T) error) Format {
	return Format{name: name, parse: func(file []byte, ptr any) error {
		return fn(file, ptr.(*T))
	}}
}

// LiteralParser sets data unaltered into a []byte or string
func LiteralParser(data []byte, into any) error {
	switch ptr := into.(type) {
	case *[]byte:
		*ptr = append([]byte(nil), data...)
	case *string:
		*ptr = string(data)
	default:
		return fmt.Errorf("pass *[]byte, *string or use a parser. got %T", into)
	}
	return nil
}
