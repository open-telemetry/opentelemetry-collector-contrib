// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRFC6020ParserBuiltinTypes(t *testing.T) {
	parser := NewRFC6020Parser()

	// Test that all RFC 6020/7950 built-in types are loaded
	expectedTypes := []string{
		"int8", "int16", "int32", "int64",
		"uint8", "uint16", "uint32", "uint64",
		"decimal64", "string", "boolean", "enumeration",
		"bits", "binary", "leafref", "identityref",
		"empty", "union", "instance-identifier",
	}

	builtins := parser.GetBuiltinTypes()
	if len(builtins) != len(expectedTypes) {
		t.Errorf("Expected %d built-in types, got %d", len(expectedTypes), len(builtins))
	}

	for _, typeName := range expectedTypes {
		if builtin, exists := builtins[typeName]; !exists {
			t.Errorf("Missing built-in type: %s", typeName)
		} else {
			// Validate builtin type properties
			if builtin.Name != typeName {
				t.Errorf("Type name mismatch for %s: got %s", typeName, builtin.Name)
			}

			// Check numeric types have correct properties
			if strings.HasPrefix(typeName, "int") || strings.HasPrefix(typeName, "uint") {
				if !builtin.IsNumeric {
					t.Errorf("Type %s should be numeric", typeName)
				}
				if len(builtin.Restrictions) == 0 {
					t.Errorf("Numeric type %s should have range restrictions", typeName)
				}
			}
		}
	}
}

func TestRFC6020ParserTokenization(t *testing.T) {
	parser := NewRFC6020Parser()

	// Test C-style comment removal (RFC 6020 Section 6.1.1)
	yangContent := `
module test-module {
    // This is a single-line comment
    yang-version 1.1;
    /* This is a 
       multi-line comment */
    namespace "urn:test:module";
    prefix "test";
}
`

	tokens, err := parser.TokenizeYANG(yangContent)
	if err != nil {
		t.Fatalf("Tokenization failed: %v", err)
	}

	// Check that we have the expected number of tokens
	expectedTokenCount := 13 // Based on actual tokenizer output with proper semicolon separation
	if len(tokens) != expectedTokenCount {
		t.Errorf("Expected %d tokens, got %d. Tokens: %v", expectedTokenCount, len(tokens), tokens)
	}

	// Verify key tokens are present
	tokenStr := strings.Join(tokens, " ")
	keyTokens := []string{"module", "test-module", "yang-version", "1.1", "namespace", "prefix"}
	for _, expected := range keyTokens {
		if !strings.Contains(tokenStr, expected) {
			t.Errorf("Expected token '%s' not found in tokens", expected)
		}
	}

	// Verify that no comment content exists in tokens
	for _, token := range tokens {
		if strings.Contains(token, "//") || strings.Contains(token, "/*") ||
			strings.Contains(token, "single-line") || strings.Contains(token, "multi-line") {
			t.Errorf("Comment content found in tokens: %s", token)
		}
	}
}

func TestRFC6020ParserModuleParsing(t *testing.T) {
	parser := NewRFC6020Parser()

	// Test basic RFC compliance with simple module
	yangContent := `module test-rfc { yang-version 1.1; namespace "urn:test:rfc"; prefix "rfc"; }`

	module, err := parser.ParseYANGModule(yangContent, "test-module.yang")
	if err != nil {
		t.Fatalf("Module parsing failed: %v", err)
	}

	// Validate RFC compliance basics
	if module.Name != "test-rfc" {
		t.Errorf("Expected module name 'test-rfc', got '%s'", module.Name)
	}

	if module.YangVersion != "1.1" {
		t.Errorf("Expected yang-version '1.1', got '%s'", module.YangVersion)
	}

	// Test RFC built-in types are available
	builtinTypes := parser.GetBuiltinTypes()
	if len(builtinTypes) != 19 {
		t.Errorf("Expected 19 RFC built-in types, got %d", len(builtinTypes))
	}

	// Verify key RFC types exist
	requiredTypes := []string{
		"int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64",
		"decimal64", "string", "boolean", "enumeration", "bits", "binary", "leafref", "identityref",
		"empty", "union", "instance-identifier",
	}

	for _, typeName := range requiredTypes {
		if _, exists := builtinTypes[typeName]; !exists {
			t.Errorf("Missing required RFC built-in type: %s", typeName)
		}
	}
}

func TestRFC6020SemanticAnalysis(t *testing.T) {
	parser := NewRFC6020Parser()

	// Test semantic analysis for counter vs gauge classification
	yangContent := `
module test-semantics {
    yang-version 1.1;
    namespace "urn:test:semantics";
    prefix "sem";
    
    container stats {
        config false;
        
        leaf byte-counter {
            type uint64;
            units "bytes";
            description "Byte counter - should be classified as counter";
        }
        
        leaf packet-rate {
            type uint32;
            units "packets-per-second";
            description "Packet rate - should be classified as gauge";
        }
        
        leaf cpu-percent {
            type uint8 {
                range "0..100";
            }
            units "percent";
            description "CPU utilization - should be classified as gauge";
        }
        
        leaf error-count {
            type uint32;
            units "errors";
            description "Error count - should be classified as counter";
        }
        
        leaf interface-name {
            type string;
            description "Interface name - should be info";
        }
    }
}
`

	module, err := parser.ParseYANGModule(yangContent, "test-semantics.yang")
	if err != nil {
		t.Fatalf("Module parsing failed: %v", err)
	}

	// Check counter classification
	expectedCounters := []string{"/stats/byte-counter", "/stats/error-count"}
	for _, path := range expectedCounters {
		found := slices.Contains(module.Counters, path)
		if !found {
			t.Errorf("Expected counter path %s not found in counters: %v", path, module.Counters)
		}

		// Check data type classification
		if dataType, exists := module.DataTypes[path]; exists {
			if !dataType.IsCounter {
				t.Errorf("Path %s should be classified as counter", path)
			}
			if dataType.SemanticType != "counter" {
				t.Errorf("Path %s should have semantic type 'counter', got '%s'", path, dataType.SemanticType)
			}
		}
	}

	// Check gauge classification
	expectedGauges := []string{"/stats/packet-rate", "/stats/cpu-percent"}
	for _, path := range expectedGauges {
		found := slices.Contains(module.Gauges, path)
		if !found {
			t.Errorf("Expected gauge path %s not found in gauges: %v", path, module.Gauges)
		}

		// Check data type classification
		if dataType, exists := module.DataTypes[path]; exists {
			if !dataType.IsGauge {
				t.Errorf("Path %s should be classified as gauge", path)
			}
			if dataType.SemanticType != "gauge" {
				t.Errorf("Path %s should have semantic type 'gauge', got '%s'", path, dataType.SemanticType)
			}
		}
	}

	// Check info classification
	infoPath := "/stats/interface-name"
	if dataType, exists := module.DataTypes[infoPath]; exists {
		if dataType.SemanticType != "info" {
			t.Errorf("Path %s should have semantic type 'info', got '%s'", infoPath, dataType.SemanticType)
		}
	}

	// Validate config vs state classification
	for path, dataType := range module.DataTypes {
		if strings.HasPrefix(path, "/stats/") {
			if dataType.IsConfiguration {
				t.Errorf("Path %s under stats should not be configuration data", path)
			}
			if !dataType.IsState {
				t.Errorf("Path %s under stats should be state data", path)
			}
		}
	}
}

func TestRFC6020TypeResolution(t *testing.T) {
	parser := NewRFC6020Parser()

	yangContent := `
module test-types {
    yang-version 1.1;
    namespace "urn:test:types";
    prefix "types";
    
    typedef custom-string {
        type string {
            length "1..255";
            pattern "[A-Za-z0-9_-]+";
        }
        description "Custom string type with restrictions";
    }
    
    typedef bandwidth-type {
        type uint64;
        units "bits-per-second";
        description "Bandwidth in bits per second";
    }
    
    container test-data {
        leaf name {
            type custom-string;
            description "Name field using custom string type";
        }
        
        leaf speed {
            type bandwidth-type;
            description "Interface speed";
        }
        
        leaf utilization {
            type decimal64 {
                fraction-digits 3;
                range "0.000..100.000";
            }
            units "percent";
            description "Utilization percentage with 3 decimal places";
        }
    }
}
`

	module, err := parser.ParseYANGModule(yangContent, "test-types.yang")
	if err != nil {
		t.Fatalf("Module parsing failed: %v", err)
	}

	// Test custom-string typedef resolution
	if dataType, exists := module.DataTypes["/test-data/name"]; exists {
		if dataType.BaseBuiltinType != "string" {
			t.Errorf("Expected base builtin type 'string', got '%s'", dataType.BaseBuiltinType)
		}
		if dataType.OriginalType != "custom-string" {
			t.Errorf("Expected original type 'custom-string', got '%s'", dataType.OriginalType)
		}
	} else {
		t.Error("Missing data type for /test-data/name")
	}

	// Test bandwidth-type typedef resolution
	if dataType, exists := module.DataTypes["/test-data/speed"]; exists {
		if dataType.BaseBuiltinType != "uint64" {
			t.Errorf("Expected base builtin type 'uint64', got '%s'", dataType.BaseBuiltinType)
		}
		if dataType.Units != "bits-per-second" {
			t.Errorf("Expected units 'bits-per-second', got '%s'", dataType.Units)
		}
	} else {
		t.Error("Missing data type for /test-data/speed")
	}

	// Test decimal64 with fraction-digits
	if dataType, exists := module.DataTypes["/test-data/utilization"]; exists {
		if dataType.BaseBuiltinType != "decimal64" {
			t.Errorf("Expected base builtin type 'decimal64', got '%s'", dataType.BaseBuiltinType)
		}
		if dataType.FractionDigits != 3 {
			t.Errorf("Expected fraction digits 3, got %d", dataType.FractionDigits)
		}
	} else {
		t.Error("Missing data type for /test-data/utilization")
	}
}

func TestRFC6020ExportImport(t *testing.T) {
	parser := NewRFC6020Parser()

	// Parse a simple module
	yangContent := `
module test-export {
    yang-version 1.1;
    namespace "urn:test:export";
    prefix "exp";
    
    container test {
        leaf value {
            type uint32;
            units "count";
        }
    }
}
`

	_, err := parser.ParseYANGModule(yangContent, "test-export.yang")
	if err != nil {
		t.Fatalf("Module parsing failed: %v", err)
	}

	// Test export
	jsonData, err := parser.ExportModules()
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("Exported JSON data is empty")
	}

	// Verify JSON contains expected module data
	jsonStr := string(jsonData)
	if !strings.Contains(jsonStr, "test-export") {
		t.Error("Exported JSON does not contain module name")
	}

	if !strings.Contains(jsonStr, "urn:test:export") {
		t.Error("Exported JSON does not contain namespace")
	}

	// Test save to file
	tmpFile := filepath.Join(t.TempDir(), "test-modules.json")
	err = parser.SaveModules(tmpFile)
	if err != nil {
		t.Fatalf("Save modules failed: %v", err)
	}

	// Clean up
	// Remove temp file - ignore error
	_ = parser.modules["test-export"] // Just to avoid unused
}

func TestRFC6020ComplianceValidation(t *testing.T) {
	parser := NewRFC6020Parser()

	// Test various RFC compliance aspects
	tests := []struct {
		name        string
		yangContent string
		shouldFail  bool
		description string
	}{
		{
			name: "valid_module_structure",
			yangContent: `
module valid-test {
    yang-version 1.1;
    namespace "urn:valid:test";
    prefix "val";
    description "Valid module structure";
}`,
			shouldFail:  false,
			description: "Basic valid module should parse successfully",
		},
		{
			name: "yang_version_11",
			yangContent: `
module yang11-test {
    yang-version 1.1;
    namespace "urn:yang11:test";
    prefix "y11";
}`,
			shouldFail:  false,
			description: "YANG 1.1 version should be supported",
		},
		{
			name: "complex_data_model",
			yangContent: `
module complex-test {
    yang-version 1.1;
    namespace "urn:complex:test";
    prefix "complex";
    
    typedef custom-type {
        type string {
            pattern "[A-Z][a-z0-9]*";
        }
    }
    
    container system {
        list interface {
            key "name type";
            
            leaf name {
                type string;
            }
            
            leaf type {
                type custom-type;
            }
            
            container statistics {
                config false;
                
                leaf packets {
                    type uint64;
                    units "packets";
                }
            }
        }
    }
}`,
			shouldFail:  false,
			description: "Complex data model with multiple keys should work",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module, err := parser.ParseYANGModule(test.yangContent, test.name+".yang")

			if test.shouldFail {
				if err == nil {
					t.Errorf("Expected parsing to fail for %s, but it succeeded", test.description)
				}
			} else {
				if err != nil {
					t.Errorf("Expected parsing to succeed for %s, but got error: %v", test.description, err)
				} else if module == nil {
					t.Errorf("Expected non-nil module for %s", test.description)
				}
			}
		})
	}
}
