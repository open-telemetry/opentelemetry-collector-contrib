// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Deprecated: [v0.92.0] This package is deprecated and will be removed in a future release.
// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30187
package main

import (
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/components"
	internalconfigschema "github.com/open-telemetry/opentelemetry-collector-contrib/internal/configschema"
)

func main() {
	c, err := components.Components()
	if err != nil {
		panic(err)
	}
	dr := configschema.NewDirResolver(filepath.Join("..", ".."), configschema.DefaultModule)

	for _, e := range c.Extensions {
		v := reflect.ValueOf(e.CreateDefaultConfig())
		sourceDir := dr.ReflectValueToProjectPath(v)
		err := internalconfigschema.GenerateConfigDoc(sourceDir, e)
		if err != nil {
			fmt.Printf("skipped writing docs: %v\n", err)
		}
	}
	for _, e := range c.Exporters {
		v := reflect.ValueOf(e.CreateDefaultConfig())
		sourceDir := dr.ReflectValueToProjectPath(v)
		err := internalconfigschema.GenerateConfigDoc(sourceDir, e)
		if err != nil {
			fmt.Printf("skipped writing docs: %v\n", err)
		}
	}
	for _, p := range c.Processors {
		v := reflect.ValueOf(p.CreateDefaultConfig())
		sourceDir := dr.ReflectValueToProjectPath(v)
		err := internalconfigschema.GenerateConfigDoc(sourceDir, p)
		if err != nil {
			fmt.Printf("skipped writing docs: %v\n", err)
		}
	}
	for _, r := range c.Receivers {
		v := reflect.ValueOf(r.CreateDefaultConfig())
		sourceDir := dr.ReflectValueToProjectPath(v)
		err := internalconfigschema.GenerateConfigDoc(sourceDir, r)
		if err != nil {
			fmt.Printf("skipped writing docs: %v\n", err)
		}
	}
	for _, c := range c.Connectors {
		v := reflect.ValueOf(c.CreateDefaultConfig())
		sourceDir := dr.ReflectValueToProjectPath(v)
		err := internalconfigschema.GenerateConfigDoc(sourceDir, c)
		if err != nil {
			fmt.Printf("skipped writing docs: %v\n", err)
		}
	}
}
