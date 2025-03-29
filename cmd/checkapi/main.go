// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/checkapi/internal"

	"gopkg.in/yaml.v3"
)

func main() {
	folder := flag.String("folder", ".", "folder investigated for modules")
	configPath := flag.String("config", "cmd/checkapi/config.yaml", "configuration file")
	flag.Parse()
	if err := run(*folder, *configPath); err != nil {
		log.Fatal(err)
	}
}

func run(folder string, configPath string) error {
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var cfg internal.Config
	err = yaml.Unmarshal(configData, &cfg)
	if err != nil {
		return err
	}
	var errs []error
	err = filepath.Walk(folder, func(path string, info fs.FileInfo, _ error) error {
		if info.Name() == "go.mod" {
			base := filepath.Dir(path)
			relativeBase, err2 := filepath.Rel(folder, base)
			if err2 != nil {
				return err2
			}
			// no code paths under internal need to be inspected
			if strings.HasPrefix(relativeBase, "internal") {
				return nil
			}

			for _, a := range cfg.IgnoredPaths {
				if filepath.Join(filepath.SplitList(a)...) == relativeBase {
					fmt.Printf("Ignoring %s per denylist\n", base)
					return nil
				}
			}
			componentType, err3 := internal.ReadComponentType(base)
			if err3 != nil {
				return err3
			}
			if err = walkFolder(cfg, base, componentType); err != nil {
				errs = append(errs, err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func walkFolder(cfg internal.Config, folder string, componentType string) error {
	result, err := internal.Read(folder, cfg.IgnoredFunctions)
	if err != nil {
		return err
	}

	sort.Slice(result.Structs, func(i int, j int) bool {
		return strings.Compare(result.Structs[i].Name, result.Structs[j].Name) > 0
	})
	sort.Strings(result.Values)
	sort.Slice(result.Functions, func(i int, j int) bool {
		return strings.Compare(result.Functions[i].Name, result.Functions[j].Name) < 0
	})
	fnNames := make([]string, len(result.Functions))
	for i, fn := range result.Functions {
		fnNames[i] = fn.Name
	}
	if len(result.Structs) == 0 && len(result.Values) == 0 && len(result.Functions) == 0 {
		// nothing to validate, return
		return nil
	}

	var errs []error

	if len(cfg.AllowedFunctions) > 0 {

		functionsPresent := map[string]struct{}{}
	OUTER:
		for _, fnDesc := range cfg.AllowedFunctions {
			matchComponentType := false
			for _, c := range fnDesc.Classes {
				if c == componentType {
					matchComponentType = true
					break
				}
			}
			if !matchComponentType {
				continue
			}
			for _, fn := range result.Functions {
				if fn.Name == fnDesc.Name &&
					slices.Equal(fn.ParamTypes, fnDesc.Parameters) &&
					slices.Equal(fn.ReturnTypes, fnDesc.ReturnTypes) {
					functionsPresent[fn.Name] = struct{}{}
					break OUTER
				}
			}
		}

		if len(functionsPresent) == 0 {
			errs = append(errs, fmt.Errorf("[%s] no function matching configuration found", folder))
		}
	}

	if cfg.UnkeyedLiteral.Enabled {
		for _, s := range result.Structs {
			if err := checkStructDisallowUnkeyedLiteral(cfg, s, folder); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}

func checkStructDisallowUnkeyedLiteral(cfg internal.Config, s *internal.Apistruct, folder string) error {
	if !unicode.IsUpper(rune(s.Name[0])) {
		return nil
	}
	if len(s.Fields) > cfg.UnkeyedLiteral.Limit {
		return nil
	}
	for _, f := range s.Fields {
		if !unicode.IsUpper(rune(f[0])) {
			return nil
		}
	}
	return fmt.Errorf("%s struct %q does not prevent unkeyed literal initialization", folder, s.Name)
}
