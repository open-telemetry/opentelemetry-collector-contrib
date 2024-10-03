// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testar // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testar"

import (
	"fmt"
	"io/fs"
	"reflect"
	"strings"

	"golang.org/x/tools/txtar"
)

type Parser = func([]byte, any) error

type Formats map[string]Parser

func (fmts Formats) Parse(format string, data []byte, into any) error {
	parse, ok := fmts[format]
	if !ok {
		return fmt.Errorf("no such format: %q", format)
	}
	return parse(data, into)
}

type Reader struct {
	Formats Formats
}

func (rd Reader) ReadFile(file string, into any) error {
	ar, err := txtar.ParseFile(file)
	if err != nil {
		return err
	}

	arfs, err := txtar.FS(ar)
	if err != nil {
		return err
	}

	pv := reflect.ValueOf(into)
	if pv.Kind() != reflect.Pointer {
		return fmt.Errorf("into must be pointer")
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

		err = rd.Formats.Parse(format, data, sv.Field(i).Addr().Interface())
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}
