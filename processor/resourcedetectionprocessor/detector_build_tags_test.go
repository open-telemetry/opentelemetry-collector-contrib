// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcedetectionprocessor

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Discover factory declarations rather than keeping another detector list. This
// also catches newly added detectors that forget the opt-in or omitted factory.
func TestDetectorBuildTags(t *testing.T) {
	type factoryFile struct {
		name           string
		implementation bool
	}
	factories := map[string][]factoryFile{}
	omitTags := map[string]string{}
	omitPattern := regexp.MustCompile(`\bomit_detector_\w+\b`)
	err := filepath.WalkDir("internal", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if parseErr != nil {
			return parseErr
		}
		for _, decl := range file.Decls {
			implementation := false
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.Name != "NewDetector" || d.Recv != nil {
					continue
				}
				implementation = true
			case *ast.GenDecl:
				if d.Tok != token.VAR {
					continue
				}
				found := false
				for _, spec := range d.Specs {
					for _, name := range spec.(*ast.ValueSpec).Names {
						found = found || name.Name == "NewDetector"
					}
				}
				if !found {
					continue
				}
			default:
				continue
			}
			dir := filepath.Dir(path)
			factories[dir] = append(factories[dir], factoryFile{filepath.Base(path), implementation})
			for _, group := range file.Comments {
				for _, comment := range group.List {
					if strings.HasPrefix(comment.Text, "//go:build ") {
						for _, tag := range omitPattern.FindAllString(comment.Text, -1) {
							if previous := omitTags[dir]; previous != "" {
								assert.Equal(t, previous, tag, "inconsistent omit tags in %s", dir)
							}
							omitTags[dir] = tag
						}
					}
				}
			}
		}
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, factories)

	for dir, files := range factories {
		t.Run(filepath.ToSlash(dir), func(t *testing.T) {
			omit := omitTags[dir]
			require.NotEmpty(t, omit, "detector needs an omit tag and complementary factory files")
			enable := "enable_detector_" + strings.TrimPrefix(omit, "omit_detector_")
			for _, removeAll := range []bool{false, true} {
				for _, enabled := range []bool{false, true} {
					for _, omitted := range []bool{false, true} {
						ctx := build.Default
						ctx.BuildTags = nil
						if removeAll {
							ctx.BuildTags = append(ctx.BuildTags, "remove_all_resourcedetection_detectors")
						}
						if enabled {
							ctx.BuildTags = append(ctx.BuildTags, enable)
						}
						if omitted {
							ctx.BuildTags = append(ctx.BuildTags, omit)
						}
						wantImplementation := (!removeAll || enabled) && !omitted
						selected := 0
						for _, file := range files {
							matches, matchErr := ctx.MatchFile(dir, file.name)
							require.NoError(t, matchErr)
							if matches {
								selected++
								assert.Equal(t, wantImplementation, file.implementation, "file %s with tags %v", file.name, ctx.BuildTags)
							}
						}
						assert.Equal(t, 1, selected, "exactly one factory must compile with tags %v", ctx.BuildTags)
					}
				}
			}
		})
	}
}

func TestEC2TrimmedBuildExcludesKubernetesDependencies(t *testing.T) {
	// Inspect production imports, not the test binary: existing config tests
	// import other detector implementations even in a targeted trimmed test run.
	cmd := exec.CommandContext(t.Context(), "go", "list", "-deps",
		"-tags=remove_all_resourcedetection_detectors,enable_detector_aws_ec2", ".")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	packages := strings.Fields(string(output))
	const rdpInternal = "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/"
	require.Contains(t, packages, rdpInternal+"aws/ec2")
	for _, pkg := range packages {
		for _, prefix := range []string{"k8s.io/", "sigs.k8s.io/", "github.com/openshift/"} {
			assert.False(t, strings.HasPrefix(pkg, prefix), "EC2-only build imports %s", pkg)
		}
	}
}
