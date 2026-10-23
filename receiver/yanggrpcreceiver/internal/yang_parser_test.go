// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const sampleYangContentA = `
module test-module-a {
    namespace "http://example.com/ns/yang/test-module-a";
    prefix "tma";

    list item {
        key "id";
        leaf id {
            type string;
        }
    }
}
`

const sampleYangContentB = `
module test-module-b {
    namespace "http://example.com/ns/yang/test-module-b";
    prefix "tmb";

    list record {
        key "code";
        leaf code {
            type string;
        }
    }
}
`

const sampleDuplicateYangV1 = `
module colliding-module {
    namespace "http://example.com/ns/yang/colliding-module";
    prefix "cm";

    list entry {
        key "key-v1";
        leaf key-v1 {
            type string;
        }
    }
}
`

const sampleDuplicateYangV2 = `
module colliding-module {
    namespace "http://example.com/ns/yang/colliding-module";
    prefix "cm";

    list entry {
        key "key-v2";
        leaf key-v2 {
            type string;
        }
    }
}
`

func TestExtractYANGFromFiles_SingleModuleNoWarning(t *testing.T) {
	core, recorded := observer.New(zap.WarnLevel)
	parser := NewYANGParser(zap.New(core))

	tempDir := t.TempDir()
	yangFile := filepath.Join(tempDir, "module_a.yang")
	require.NoError(t, os.WriteFile(yangFile, []byte(sampleYangContentA), 0o600))

	err := parser.ExtractYANGFromFiles(tempDir)
	require.NoError(t, err)

	assert.Equal(t, 0, recorded.Len(), "Expected no warnings for a single unique module")
	assert.Contains(t, parser.GetAvailableModules(), "test-module-a")
	assert.Equal(t, yangFile, parser.GetModuleSource("test-module-a"))
	assert.Equal(t, "id", parser.GetKeyForPath("test-module-a", "/item"))
}

func TestExtractYANGFromFiles_DistinctModulesNoWarning(t *testing.T) {
	core, recorded := observer.New(zap.WarnLevel)
	parser := NewYANGParser(zap.New(core))

	tempDir := t.TempDir()
	fileA := filepath.Join(tempDir, "module_a.yang")
	fileB := filepath.Join(tempDir, "module_b.yang")
	require.NoError(t, os.WriteFile(fileA, []byte(sampleYangContentA), 0o600))
	require.NoError(t, os.WriteFile(fileB, []byte(sampleYangContentB), 0o600))

	err := parser.ExtractYANGFromFiles(tempDir)
	require.NoError(t, err)

	assert.Equal(t, 0, recorded.Len(), "Expected no warnings for distinct modules")
	assert.Contains(t, parser.GetAvailableModules(), "test-module-a")
	assert.Contains(t, parser.GetAvailableModules(), "test-module-b")
	assert.Equal(t, fileA, parser.GetModuleSource("test-module-a"))
	assert.Equal(t, fileB, parser.GetModuleSource("test-module-b"))
}

func TestExtractYANGFromFiles_DuplicateModuleWarningAndLastWriteWins(t *testing.T) {
	core, recorded := observer.New(zap.WarnLevel)
	parser := NewYANGParser(zap.New(core))

	dir1 := t.TempDir()
	dir2 := t.TempDir()

	file1 := filepath.Join(dir1, "module_v1.yang")
	file2 := filepath.Join(dir2, "module_v2.yang")
	require.NoError(t, os.WriteFile(file1, []byte(sampleDuplicateYangV1), 0o600))
	require.NoError(t, os.WriteFile(file2, []byte(sampleDuplicateYangV2), 0o600))

	require.NoError(t, parser.ExtractYANGFromFiles(dir1))
	require.Equal(t, 0, recorded.Len())
	assert.Equal(t, "key-v1", parser.GetKeyForPath("colliding-module", "/entry"))
	assert.Equal(t, file1, parser.GetModuleSource("colliding-module"))

	require.NoError(t, parser.ExtractYANGFromFiles(dir2))
	require.Equal(t, 1, recorded.Len(), "Expected exactly 1 warning on duplicate module")

	warnEntry := recorded.All()[0]
	assert.Equal(t, "Duplicate YANG module detected; overwriting existing module", warnEntry.Message)
	context := warnEntry.ContextMap()
	assert.Equal(t, "colliding-module", context["module"])
	assert.Equal(t, file1, context["previous_path"])
	assert.Equal(t, file2, context["new_path"])

	// Assert last-write-wins
	assert.Equal(t, "key-v2", parser.GetKeyForPath("colliding-module", "/entry"))
	assert.Equal(t, file2, parser.GetModuleSource("colliding-module"))
}

func TestExtractYANGFromFiles_RecursiveDirectoryTraversal(t *testing.T) {
	core, recorded := observer.New(zap.WarnLevel)
	parser := NewYANGParser(zap.New(core))

	parentDir := t.TempDir()
	subDir1 := filepath.Join(parentDir, "release-1")
	subDir2 := filepath.Join(parentDir, "release-2")
	require.NoError(t, os.MkdirAll(subDir1, 0o750))
	require.NoError(t, os.MkdirAll(subDir2, 0o750))

	file1 := filepath.Join(subDir1, "colliding.yang")
	file2 := filepath.Join(subDir2, "colliding.yang")
	require.NoError(t, os.WriteFile(file1, []byte(sampleDuplicateYangV1), 0o600))
	require.NoError(t, os.WriteFile(file2, []byte(sampleDuplicateYangV2), 0o600))

	require.NoError(t, parser.ExtractYANGFromFiles(parentDir))
	require.Equal(t, 1, recorded.Len(), "Expected 1 warning for recursive duplicate module")

	warnEntry := recorded.All()[0]
	assert.Equal(t, "Duplicate YANG module detected; overwriting existing module", warnEntry.Message)
	context := warnEntry.ContextMap()
	assert.Equal(t, "colliding-module", context["module"])
	assert.Equal(t, file1, context["previous_path"])
	assert.Equal(t, file2, context["new_path"])
	assert.Equal(t, file2, parser.GetModuleSource("colliding-module"))
}

func TestExtractYANGFromFiles_OverwriteBuiltinModule(t *testing.T) {
	core, recorded := observer.New(zap.WarnLevel)
	parser := NewYANGParser(zap.New(core))
	parser.LoadBuiltinModules()

	assert.Equal(t, "<builtin>", parser.GetModuleSource("Cisco-IOS-XE-interfaces-oper"))

	customInterfaceYang := `
module Cisco-IOS-XE-interfaces-oper {
    namespace "http://cisco.com/ns/yang/Cisco-IOS-XE-interfaces-oper";
    prefix "interfaces-ios-xe-oper";

    list interface {
        key "custom-key";
        leaf custom-key {
            type string;
        }
    }
}
`
	tempDir := t.TempDir()
	customFile := filepath.Join(tempDir, "custom_interfaces.yang")
	require.NoError(t, os.WriteFile(customFile, []byte(customInterfaceYang), 0o600))

	require.NoError(t, parser.ExtractYANGFromFiles(tempDir))
	require.Equal(t, 1, recorded.Len(), "Expected warning when overwriting builtin module")

	warnEntry := recorded.All()[0]
	context := warnEntry.ContextMap()
	assert.Equal(t, "Cisco-IOS-XE-interfaces-oper", context["module"])
	assert.Equal(t, "<builtin>", context["previous_path"])
	assert.Equal(t, customFile, context["new_path"])

	assert.Equal(t, "custom-key", parser.GetKeyForPath("Cisco-IOS-XE-interfaces-oper", "/interface"))
	assert.Equal(t, customFile, parser.GetModuleSource("Cisco-IOS-XE-interfaces-oper"))
}

func TestExtractYANGFromFiles_IdenticalPathLoadedTwice(t *testing.T) {
	core, recorded := observer.New(zap.WarnLevel)
	parser := NewYANGParser(zap.New(core))

	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "single.yang")
	require.NoError(t, os.WriteFile(filePath, []byte(sampleYangContentA), 0o600))

	require.NoError(t, parser.ExtractYANGFromFiles(tempDir))
	assert.Equal(t, 0, recorded.Len())

	// Loading again from the same directory/path warns about overwrite
	require.NoError(t, parser.ExtractYANGFromFiles(tempDir))
	require.Equal(t, 1, recorded.Len())

	warnEntry := recorded.All()[0]
	context := warnEntry.ContextMap()
	assert.Equal(t, "test-module-a", context["module"])
	assert.Equal(t, filePath, context["previous_path"])
	assert.Equal(t, filePath, context["new_path"])
}

func TestExtractYANGFromFiles_MalformedAndNonExistent(t *testing.T) {
	core, recorded := observer.New(zap.WarnLevel)
	parser := NewYANGParser(zap.New(core))

	// Non-existent directory returns error
	err := parser.ExtractYANGFromFiles("/non/existent/path/that/does/not/exist")
	require.Error(t, err)

	// Directory with invalid YANG syntax and non-yang file
	tempDir := t.TempDir()
	txtFile := filepath.Join(tempDir, "ignore.txt")
	require.NoError(t, os.WriteFile(txtFile, []byte("plain text"), 0o600))

	malformedYang := filepath.Join(tempDir, "invalid.yang")
	require.NoError(t, os.WriteFile(malformedYang, []byte("this is not valid yang syntax"), 0o600))

	err = parser.ExtractYANGFromFiles(tempDir)
	require.NoError(t, err)
	assert.Equal(t, 0, recorded.Len())
	assert.Empty(t, parser.GetAvailableModules())
}

func TestNewYANGParser(t *testing.T) {
	logger := zap.NewNop()
	p := NewYANGParser(logger)
	require.NotNil(t, p)
	assert.NotNil(t, p.modules)
	assert.NotNil(t, p.moduleSources)
	assert.Equal(t, logger, p.logger)
}

func TestExtractYANGFromFiles_MultipleConfiguredPaths(t *testing.T) {
	core, recorded := observer.New(zap.WarnLevel)
	parser := NewYANGParser(zap.New(core))

	dir1 := t.TempDir()
	dir2 := t.TempDir()

	file1 := filepath.Join(dir1, "colliding.yang")
	file2 := filepath.Join(dir2, "colliding.yang")
	require.NoError(t, os.WriteFile(file1, []byte(sampleDuplicateYangV1), 0o600))
	require.NoError(t, os.WriteFile(file2, []byte(sampleDuplicateYangV2), 0o600))

	configuredPaths := []string{dir1, dir2}
	for _, p := range configuredPaths {
		require.NoError(t, parser.ExtractYANGFromFiles(p))
	}

	require.Equal(t, 1, recorded.Len())
	warnEntry := recorded.All()[0]
	context := warnEntry.ContextMap()
	assert.Equal(t, "colliding-module", context["module"])
	assert.Equal(t, file1, context["previous_path"])
	assert.Equal(t, file2, context["new_path"])
	assert.Equal(t, "key-v2", parser.GetKeyForPath("colliding-module", "/entry"))
}

func TestExtractYANGFromFiles_ExistingLookupUnchanged(t *testing.T) {
	core, recorded := observer.New(zap.WarnLevel)
	parser := NewYANGParser(zap.New(core))
	parser.LoadBuiltinModules()

	// Builtin lookups
	assert.Equal(t, "name", parser.GetKeyForPath("Cisco-IOS-XE-interfaces-oper", "/interfaces/interface"))
	assert.Equal(t, []string{"name"}, parser.GetKeysForList("Cisco-IOS-XE-interfaces-oper", "/interfaces/interface"))
	dt := parser.GetDataTypeForField("Cisco-IOS-XE-interfaces-oper", "/interfaces/interface/statistics/in-octets")
	require.NotNil(t, dt)
	assert.Equal(t, "uint64", dt.Type)
	assert.True(t, dt.IsNumericType())
	assert.True(t, dt.IsCounterType())
	assert.False(t, dt.IsGaugeType())

	// Telemetry analysis
	analysis := parser.AnalyzeEncodingPath("Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics")
	require.NotNil(t, analysis)
	assert.Equal(t, "Cisco-IOS-XE-interfaces-oper", analysis.ModuleName)
	assert.Equal(t, "/interfaces/interface", analysis.ListPath)
	assert.Equal(t, "name", analysis.Keys["/interfaces/interface"])

	// Extracted module with keys
	tempDir := t.TempDir()
	yangFile := filepath.Join(tempDir, "mod_lookup.yang")
	require.NoError(t, os.WriteFile(yangFile, []byte(sampleYangContentA), 0o600))
	require.NoError(t, parser.ExtractYANGFromFiles(tempDir))
	assert.Equal(t, 0, recorded.Len())

	assert.Equal(t, "id", parser.GetKeyForPath("test-module-a", "/item"))
	assert.Equal(t, []string{"id"}, parser.GetKeysForList("test-module-a", "/item"))

	analysis2 := parser.AnalyzeEncodingPath("test-module-a:folder/item")
	require.NotNil(t, analysis2)
	assert.Equal(t, "test-module-a", analysis2.ModuleName)
	assert.Equal(t, "/folder/item", analysis2.ListPath)
	assert.Equal(t, "id", analysis2.Keys["/folder/item"])
}
