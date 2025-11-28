// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal"

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// RFC6020Parser implements RFC 6020 (YANG 1.0) and RFC 7950 (YANG 1.1) compliant YANG parsing
type RFC6020Parser struct {
	modules          map[string]*RFC6020Module
	builtinTypes     map[string]*RFC6020BuiltinType
	typeRestrictions map[string]*RFC6020TypeRestriction
	logger           *log.Logger
}

// RFC6020Module represents a complete YANG module based on RFC specifications
type RFC6020Module struct {
	// Module header statements (RFC 7.1)
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Prefix       string `json:"prefix"`
	YangVersion  string `json:"yang_version"` // "1" or "1.1"
	Organization string `json:"organization"`
	Contact      string `json:"contact"`
	Description  string `json:"description"`
	Reference    string `json:"reference"`

	// Import/Include statements (RFC 7.1.5, 7.1.6)
	Imports  map[string]*RFC6020Import  `json:"imports"`
	Includes map[string]*RFC6020Include `json:"includes"`

	// Revision history (RFC 7.1.9)
	Revisions []*RFC6020Revision `json:"revisions"`

	// Type definitions (RFC 7.3)
	Typedefs map[string]*RFC6020Typedef `json:"typedefs"`

	// Groupings (RFC 7.12)
	Groupings map[string]*RFC6020Grouping `json:"groupings"`

	// Features (RFC 7.20.1)
	Features map[string]*RFC6020Feature `json:"features"`

	// Data nodes (RFC 4.2.2)
	DataNodes map[string]*RFC6020DataNode `json:"data_nodes"`

	// Semantic analysis results
	KeyedPaths  map[string]string               `json:"keyed_paths"`  // path -> primary key
	ListKeys    map[string][]string             `json:"list_keys"`    // list path -> all keys
	DataTypes   map[string]*RFC6020ResolvedType `json:"data_types"`   // field path -> resolved type
	Counters    []string                        `json:"counters"`     // paths that are counter semantics
	Gauges      []string                        `json:"gauges"`       // paths that are gauge semantics
	ConfigPaths []string                        `json:"config_paths"` // configuration data paths
	StatePaths  []string                        `json:"state_paths"`  // state data paths
}

// RFC6020Import represents an import statement (RFC 7.1.5)
type RFC6020Import struct {
	Module       string `json:"module"`
	Prefix       string `json:"prefix"`
	RevisionDate string `json:"revision_date,omitempty"`
	Description  string `json:"description,omitempty"`
	Reference    string `json:"reference,omitempty"`
}

// RFC6020Include represents an include statement (RFC 7.1.6)
type RFC6020Include struct {
	Submodule    string `json:"submodule"`
	RevisionDate string `json:"revision_date,omitempty"`
	Description  string `json:"description,omitempty"`
	Reference    string `json:"reference,omitempty"`
}

// RFC6020Revision represents a revision statement (RFC 7.1.9)
type RFC6020Revision struct {
	Date        string `json:"date"`
	Description string `json:"description,omitempty"`
	Reference   string `json:"reference,omitempty"`
}

// RFC6020Typedef represents a typedef statement (RFC 7.3)
type RFC6020Typedef struct {
	Name        string       `json:"name"`
	Type        *RFC6020Type `json:"type"`
	Units       string       `json:"units,omitempty"`
	Default     string       `json:"default,omitempty"`
	Status      string       `json:"status,omitempty"` // current, deprecated, obsolete
	Description string       `json:"description,omitempty"`
	Reference   string       `json:"reference,omitempty"`
}

// RFC6020Type represents a type statement with all restrictions (RFC 7.4, Section 9)
type RFC6020Type struct {
	Name            string           `json:"name"`
	Base            string           `json:"base,omitempty"`             // for identityref
	Path            string           `json:"path,omitempty"`             // for leafref
	Patterns        []RFC6020Pattern `json:"patterns,omitempty"`         // for string
	Ranges          []RFC6020Range   `json:"ranges,omitempty"`           // for numeric types
	Lengths         []RFC6020Range   `json:"lengths,omitempty"`          // for string/binary
	Enums           []RFC6020Enum    `json:"enums,omitempty"`            // for enumeration
	Bits            []RFC6020Bit     `json:"bits,omitempty"`             // for bits
	FractionDigits  int              `json:"fraction_digits,omitempty"`  // for decimal64
	RequireInstance bool             `json:"require_instance,omitempty"` // for leafref/instance-identifier
	UnionTypes      []RFC6020Type    `json:"union_types,omitempty"`      // for union
}

// RFC6020Pattern represents a pattern restriction (RFC 9.4.6)
type RFC6020Pattern struct {
	Value        string `json:"value"`
	Modifier     string `json:"modifier,omitempty"` // "invert-match" for YANG 1.1
	Description  string `json:"description,omitempty"`
	Reference    string `json:"reference,omitempty"`
	ErrorAppTag  string `json:"error_app_tag,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// RFC6020Range represents a range or length restriction (RFC 9.2.4, 9.4.4)
type RFC6020Range struct {
	Min          string `json:"min"` // "min" or numeric value
	Max          string `json:"max"` // "max" or numeric value
	Description  string `json:"description,omitempty"`
	Reference    string `json:"reference,omitempty"`
	ErrorAppTag  string `json:"error_app_tag,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// RFC6020Enum represents an enum statement (RFC 9.6.4)
type RFC6020Enum struct {
	Name        string `json:"name"`
	Value       *int64 `json:"value,omitempty"`
	Status      string `json:"status,omitempty"`
	Description string `json:"description,omitempty"`
	Reference   string `json:"reference,omitempty"`
	IfFeature   string `json:"if_feature,omitempty"` // YANG 1.1
}

// RFC6020Bit represents a bit statement (RFC 9.7.4)
type RFC6020Bit struct {
	Name        string `json:"name"`
	Position    *int   `json:"position,omitempty"`
	Status      string `json:"status,omitempty"`
	Description string `json:"description,omitempty"`
	Reference   string `json:"reference,omitempty"`
	IfFeature   string `json:"if_feature,omitempty"` // YANG 1.1
}

// RFC6020Feature represents a feature statement (RFC 7.20.1)
type RFC6020Feature struct {
	Name        string   `json:"name"`
	Status      string   `json:"status,omitempty"`
	Description string   `json:"description,omitempty"`
	Reference   string   `json:"reference,omitempty"`
	IfFeatures  []string `json:"if_features,omitempty"` // dependencies
}

// RFC6020DataNode represents any data definition node (RFC 4.2.2)
type RFC6020DataNode struct {
	Name        string                      `json:"name"`
	NodeType    string                      `json:"node_type"` // container, leaf, leaf-list, list, choice, case, anyxml, anydata
	Type        *RFC6020Type                `json:"type,omitempty"`
	Config      *bool                       `json:"config,omitempty"`
	Mandatory   *bool                       `json:"mandatory,omitempty"`
	Presence    string                      `json:"presence,omitempty"`
	Keys        []string                    `json:"keys,omitempty"`
	Unique      []string                    `json:"unique,omitempty"`
	MinElements *int                        `json:"min_elements,omitempty"`
	MaxElements *int                        `json:"max_elements,omitempty"`
	OrderedBy   string                      `json:"ordered_by,omitempty"`
	Default     []string                    `json:"default,omitempty"`
	Units       string                      `json:"units,omitempty"`
	Status      string                      `json:"status,omitempty"`
	Description string                      `json:"description,omitempty"`
	Reference   string                      `json:"reference,omitempty"`
	IfFeatures  []string                    `json:"if_features,omitempty"`
	Children    map[string]*RFC6020DataNode `json:"children,omitempty"`
	Path        string                      `json:"path"` // Full XPath
}

// RFC6020Grouping represents a grouping statement (RFC 7.12)
type RFC6020Grouping struct {
	Name        string                      `json:"name"`
	Status      string                      `json:"status,omitempty"`
	Description string                      `json:"description,omitempty"`
	Reference   string                      `json:"reference,omitempty"`
	Typedefs    map[string]*RFC6020Typedef  `json:"typedefs,omitempty"`
	Groupings   map[string]*RFC6020Grouping `json:"groupings,omitempty"`
	DataNodes   map[string]*RFC6020DataNode `json:"data_nodes,omitempty"`
}

// RFC6020BuiltinType represents YANG built-in types (RFC Section 9)
type RFC6020BuiltinType struct {
	Name            string   `json:"name"`
	BaseType        string   `json:"base_type,omitempty"`
	DefaultValue    string   `json:"default_value,omitempty"`
	Restrictions    []string `json:"restrictions,omitempty"`
	LexicalFormat   string   `json:"lexical_format,omitempty"`
	CanonicalFormat string   `json:"canonical_format,omitempty"`
	ValueSpace      string   `json:"value_space,omitempty"`
	IsNumeric       bool     `json:"is_numeric"`
	IsSigned        bool     `json:"is_signed,omitempty"`
	BitSize         int      `json:"bit_size,omitempty"`
}

// RFC6020TypeRestriction represents type restriction rules
type RFC6020TypeRestriction struct {
	AllowedRestrictions []string `json:"allowed_restrictions"`
	DefaultRange        string   `json:"default_range,omitempty"`
	DefaultLength       string   `json:"default_length,omitempty"`
}

// RFC6020ResolvedType represents a fully resolved type with semantic information
type RFC6020ResolvedType struct {
	OriginalType    string           `json:"original_type"`
	ResolvedType    string           `json:"resolved_type"`
	BaseBuiltinType string           `json:"base_builtin_type"`
	Units           string           `json:"units,omitempty"`
	Range           *RFC6020Range    `json:"range,omitempty"`
	Enumeration     map[string]int64 `json:"enumeration,omitempty"`
	Patterns        []string         `json:"patterns,omitempty"`
	FractionDigits  int              `json:"fraction_digits,omitempty"`
	IsCounter       bool             `json:"is_counter"`
	IsGauge         bool             `json:"is_gauge"`
	IsConfiguration bool             `json:"is_configuration"`
	IsState         bool             `json:"is_state"`
	SemanticType    string           `json:"semantic_type"` // counter, gauge, info
	Description     string           `json:"description,omitempty"`
}

// NewRFC6020Parser creates a new RFC-compliant YANG parser
func NewRFC6020Parser() *RFC6020Parser {
	parser := &RFC6020Parser{
		modules:          make(map[string]*RFC6020Module),
		builtinTypes:     make(map[string]*RFC6020BuiltinType),
		typeRestrictions: make(map[string]*RFC6020TypeRestriction),
		logger:           log.New(os.Stdout, "[RFC6020Parser] ", log.LstdFlags),
	}

	parser.initializeBuiltinTypes()
	return parser
}

// initializeBuiltinTypes initializes all YANG built-in types according to RFC 6020/7950
func (p *RFC6020Parser) initializeBuiltinTypes() {
	// RFC 9.2: Numeric types
	p.builtinTypes["int8"] = &RFC6020BuiltinType{
		Name: "int8", BaseType: "integer", IsNumeric: true, IsSigned: true, BitSize: 8,
		ValueSpace: "-128 to 127", Restrictions: []string{"range"},
		LexicalFormat:   "Decimal number with optional leading sign",
		CanonicalFormat: "Decimal number with no leading zeros, no plus sign",
	}

	p.builtinTypes["int16"] = &RFC6020BuiltinType{
		Name: "int16", BaseType: "integer", IsNumeric: true, IsSigned: true, BitSize: 16,
		ValueSpace: "-32768 to 32767", Restrictions: []string{"range"},
		LexicalFormat:   "Decimal number with optional leading sign",
		CanonicalFormat: "Decimal number with no leading zeros, no plus sign",
	}

	p.builtinTypes["int32"] = &RFC6020BuiltinType{
		Name: "int32", BaseType: "integer", IsNumeric: true, IsSigned: true, BitSize: 32,
		ValueSpace: "-2147483648 to 2147483647", Restrictions: []string{"range"},
		LexicalFormat:   "Decimal number with optional leading sign",
		CanonicalFormat: "Decimal number with no leading zeros, no plus sign",
	}

	p.builtinTypes["int64"] = &RFC6020BuiltinType{
		Name: "int64", BaseType: "integer", IsNumeric: true, IsSigned: true, BitSize: 64,
		ValueSpace: "-9223372036854775808 to 9223372036854775807", Restrictions: []string{"range"},
		LexicalFormat:   "Decimal number with optional leading sign",
		CanonicalFormat: "Decimal number with no leading zeros, no plus sign",
	}

	p.builtinTypes["uint8"] = &RFC6020BuiltinType{
		Name: "uint8", BaseType: "integer", IsNumeric: true, IsSigned: false, BitSize: 8,
		ValueSpace: "0 to 255", Restrictions: []string{"range"},
		LexicalFormat:   "Decimal number without leading sign",
		CanonicalFormat: "Decimal number with no leading zeros",
	}

	p.builtinTypes["uint16"] = &RFC6020BuiltinType{
		Name: "uint16", BaseType: "integer", IsNumeric: true, IsSigned: false, BitSize: 16,
		ValueSpace: "0 to 65535", Restrictions: []string{"range"},
		LexicalFormat:   "Decimal number without leading sign",
		CanonicalFormat: "Decimal number with no leading zeros",
	}

	p.builtinTypes["uint32"] = &RFC6020BuiltinType{
		Name: "uint32", BaseType: "integer", IsNumeric: true, IsSigned: false, BitSize: 32,
		ValueSpace: "0 to 4294967295", Restrictions: []string{"range"},
		LexicalFormat:   "Decimal number without leading sign",
		CanonicalFormat: "Decimal number with no leading zeros",
	}

	p.builtinTypes["uint64"] = &RFC6020BuiltinType{
		Name: "uint64", BaseType: "integer", IsNumeric: true, IsSigned: false, BitSize: 64,
		ValueSpace: "0 to 18446744073709551615", Restrictions: []string{"range"},
		LexicalFormat:   "Decimal number without leading sign",
		CanonicalFormat: "Decimal number with no leading zeros",
	}

	// RFC 9.3: decimal64
	p.builtinTypes["decimal64"] = &RFC6020BuiltinType{
		Name: "decimal64", BaseType: "decimal", IsNumeric: true, IsSigned: true,
		ValueSpace: "Decimal numbers with 1-18 fraction digits", Restrictions: []string{"range", "fraction-digits"},
		LexicalFormat:   "Decimal number with mandatory fraction-digits",
		CanonicalFormat: "Decimal representation with required fraction digits",
	}

	// RFC 9.4: string
	p.builtinTypes["string"] = &RFC6020BuiltinType{
		Name: "string", BaseType: "string", IsNumeric: false,
		ValueSpace:      "Unicode/ISO 10646 characters excluding C0 controls, surrogates, noncharacters",
		Restrictions:    []string{"length", "pattern"},
		LexicalFormat:   "UTF-8 character sequence",
		CanonicalFormat: "Same as lexical representation",
	}

	// RFC 9.5: boolean
	p.builtinTypes["boolean"] = &RFC6020BuiltinType{
		Name: "boolean", BaseType: "boolean", IsNumeric: false,
		ValueSpace: "true, false", Restrictions: []string{},
		LexicalFormat:   "true or false",
		CanonicalFormat: "true or false",
	}

	// RFC 9.6: enumeration
	p.builtinTypes["enumeration"] = &RFC6020BuiltinType{
		Name: "enumeration", BaseType: "enumeration", IsNumeric: false,
		ValueSpace: "Defined by enum statements", Restrictions: []string{"enum"},
		LexicalFormat:   "Enum name string",
		CanonicalFormat: "Same as lexical representation",
	}

	// RFC 9.7: bits
	p.builtinTypes["bits"] = &RFC6020BuiltinType{
		Name: "bits", BaseType: "bits", IsNumeric: false,
		ValueSpace: "Set of bit positions defined by bit statements", Restrictions: []string{"bit"},
		LexicalFormat:   "Space-separated list of bit names",
		CanonicalFormat: "Space-separated list ordered by position",
	}

	// RFC 9.8: binary
	p.builtinTypes["binary"] = &RFC6020BuiltinType{
		Name: "binary", BaseType: "binary", IsNumeric: false,
		ValueSpace: "Any binary data", Restrictions: []string{"length"},
		LexicalFormat:   "Base64 encoded string",
		CanonicalFormat: "Base64 with standard alphabet, no line breaks",
	}

	// RFC 9.9: leafref
	p.builtinTypes["leafref"] = &RFC6020BuiltinType{
		Name: "leafref", BaseType: "leafref", IsNumeric: false,
		ValueSpace: "Same as referenced leaf", Restrictions: []string{"path", "require-instance"},
		LexicalFormat:   "Same as referenced leaf type",
		CanonicalFormat: "Same as referenced leaf type",
	}

	// RFC 9.10: identityref
	p.builtinTypes["identityref"] = &RFC6020BuiltinType{
		Name: "identityref", BaseType: "identityref", IsNumeric: false,
		ValueSpace: "Identity names derived from base identity", Restrictions: []string{"base"},
		LexicalFormat:   "QName with optional prefix",
		CanonicalFormat: "QName in module's namespace",
	}

	// RFC 9.11: empty
	p.builtinTypes["empty"] = &RFC6020BuiltinType{
		Name: "empty", BaseType: "empty", IsNumeric: false,
		ValueSpace: "No value", Restrictions: []string{},
		LexicalFormat:   "Not applicable",
		CanonicalFormat: "Not applicable",
	}

	// RFC 9.12: union
	p.builtinTypes["union"] = &RFC6020BuiltinType{
		Name: "union", BaseType: "union", IsNumeric: false,
		ValueSpace: "Union of member types", Restrictions: []string{"type"},
		LexicalFormat:   "Any valid member type format",
		CanonicalFormat: "First matching member type canonical form",
	}

	// RFC 9.13: instance-identifier
	p.builtinTypes["instance-identifier"] = &RFC6020BuiltinType{
		Name: "instance-identifier", BaseType: "instance-identifier", IsNumeric: false,
		ValueSpace: "XPath expressions identifying data nodes", Restrictions: []string{"require-instance"},
		LexicalFormat:   "XPath subset identifying instance nodes",
		CanonicalFormat: "Absolute path with predicates in canonical order",
	}

	p.logger.Printf("Initialized %d built-in YANG types per RFC 6020/7950", len(p.builtinTypes))
}

// ParseYANGModule parses a YANG module from content according to RFC specifications
func (p *RFC6020Parser) ParseYANGModule(content, filename string) (*RFC6020Module, error) {
	module := &RFC6020Module{
		Imports:     make(map[string]*RFC6020Import),
		Includes:    make(map[string]*RFC6020Include),
		Revisions:   make([]*RFC6020Revision, 0),
		Typedefs:    make(map[string]*RFC6020Typedef),
		Groupings:   make(map[string]*RFC6020Grouping),
		Features:    make(map[string]*RFC6020Feature),
		DataNodes:   make(map[string]*RFC6020DataNode),
		KeyedPaths:  make(map[string]string),
		ListKeys:    make(map[string][]string),
		DataTypes:   make(map[string]*RFC6020ResolvedType),
		Counters:    make([]string, 0),
		Gauges:      make([]string, 0),
		ConfigPaths: make([]string, 0),
		StatePaths:  make([]string, 0),
	}

	// Tokenize and parse according to RFC 6020 Section 6
	tokens, err := p.TokenizeYANG(content)
	if err != nil {
		return nil, fmt.Errorf("tokenization failed: %w", err)
	}

	p.parseTokens(tokens, module)

	// Perform semantic analysis
	p.performSemanticAnalysis(module)

	p.modules[module.Name] = module
	p.logger.Printf("Successfully parsed YANG module '%s' from %s", module.Name, filename)

	return module, nil
}

// tokenizeYANG performs lexical analysis according to RFC 6020 Section 6.1
func (*RFC6020Parser) TokenizeYANG(content string) ([]string, error) {
	var tokens []string

	// Remove C-style comments (RFC 6020 Section 6.1.1)
	// Single-line comments: // comment
	singleLineCommentRe := regexp.MustCompile(`//.*?(?:\r?\n|$)`)
	content = singleLineCommentRe.ReplaceAllString(content, "\n")

	// Block comments: /* comment */ (including multiline)
	blockCommentRe := regexp.MustCompile(`(?s)/\*.*?\*/`)
	content = blockCommentRe.ReplaceAllString(content, " ")

	// Tokenize according to RFC 6020 Section 6.1.2
	// Strings (with newlines), keywords, semicolons, braces, numbers
	tokenRe := regexp.MustCompile(`(?s)"[^"]*"|'[^']*'|[a-zA-Z_][a-zA-Z0-9_.-]*|[0-9]+(?:\.[0-9]+)?|[{};]`)
	matches := tokenRe.FindAllString(content, -1)

	for _, match := range matches {
		match = strings.TrimSpace(match)
		if match != "" && match != "\n" && match != "\r" {
			tokens = append(tokens, match)
		}
	}

	return tokens, nil
}

// parseTokens parses tokenized YANG content according to RFC grammar
func (p *RFC6020Parser) parseTokens(tokens []string, module *RFC6020Module) {
	i := 0

	for i < len(tokens) {
		switch tokens[i] {
		case "module":
			if i+1 < len(tokens) {
				module.Name = p.unquoteString(tokens[i+1])
				i += 2
			}
		case "yang-version":
			if i+1 < len(tokens) {
				value := p.unquoteString(tokens[i+1])
				// Remove trailing semicolon if present
				value = strings.TrimSuffix(value, ";")
				module.YangVersion = value
				i += 2
				// Skip semicolon if it's the next token
				if i < len(tokens) && tokens[i] == ";" {
					i++
				}
			}
		case "namespace":
			if i+1 < len(tokens) {
				module.Namespace = p.unquoteString(tokens[i+1])
				i += 2
				// Skip semicolon if it's the next token
				if i < len(tokens) && tokens[i] == ";" {
					i++
				}
			}
		case "prefix":
			if i+1 < len(tokens) {
				module.Prefix = p.unquoteString(tokens[i+1])
				i += 2
				// Skip semicolon if it's the next token
				if i < len(tokens) && tokens[i] == ";" {
					i++
				}
			}
		case "organization":
			if i+1 < len(tokens) {
				module.Organization = p.unquoteString(tokens[i+1])
				i += 2
			}
		case "contact":
			if i+1 < len(tokens) {
				module.Contact = p.unquoteString(tokens[i+1])
				i += 2
			}
		case "description":
			if i+1 < len(tokens) {
				module.Description = p.unquoteString(tokens[i+1])
				i += 2
			}
		case "reference":
			if i+1 < len(tokens) {
				module.Reference = p.unquoteString(tokens[i+1])
				i += 2
			}
		case "revision":
			rev, consumed := p.parseRevision(tokens[i:])
			if rev != nil {
				module.Revisions = append(module.Revisions, rev)
			}
			i += consumed
		case "import":
			imp, consumed := p.parseImport(tokens[i:])
			if imp != nil {
				module.Imports[imp.Module] = imp
			}
			i += consumed
		case "typedef":
			td, consumed := p.parseTypedef(tokens[i:])
			if td != nil {
				module.Typedefs[td.Name] = td
			}
			i += consumed
		case "feature":
			feat, consumed := p.parseFeature(tokens[i:])
			if feat != nil {
				module.Features[feat.Name] = feat
			}
			i += consumed
		case "container", "leaf", "leaf-list", "list":
			node, consumed := p.parseDataNode(tokens[i:], "")
			if node != nil {
				module.DataNodes[node.Name] = node
				node.Path = "/" + node.Name
			}
			i += consumed
		default:
			i++
		}
	}

	// Sort revisions by date (newest first)
	sort.Slice(module.Revisions, func(i, j int) bool {
		return module.Revisions[i].Date > module.Revisions[j].Date
	})
}

// parseRevision parses a revision statement (RFC 7.1.9)
func (p *RFC6020Parser) parseRevision(tokens []string) (*RFC6020Revision, int) {
	if len(tokens) < 2 {
		return nil, 1
	}

	rev := &RFC6020Revision{
		Date: p.unquoteString(tokens[1]),
	}

	i := 2
	if i < len(tokens) && tokens[i] == "{" {
		i++
		for i < len(tokens) && tokens[i] != "}" {
			switch tokens[i] {
			case "description":
				if i+1 < len(tokens) {
					rev.Description = p.unquoteString(tokens[i+1])
					i += 2
				}
			case "reference":
				if i+1 < len(tokens) {
					rev.Reference = p.unquoteString(tokens[i+1])
					i += 2
				}
			default:
				i++
			}
		}
		if i < len(tokens) && tokens[i] == "}" {
			i++
		}
	}

	return rev, i
}

// parseImport parses an import statement (RFC 7.1.5)
func (p *RFC6020Parser) parseImport(tokens []string) (*RFC6020Import, int) {
	if len(tokens) < 2 {
		return nil, 1
	}

	imp := &RFC6020Import{
		Module: p.unquoteString(tokens[1]),
	}

	i := 2
	if i < len(tokens) && tokens[i] == "{" {
		i++
		for i < len(tokens) && tokens[i] != "}" {
			switch tokens[i] {
			case "prefix":
				if i+1 < len(tokens) {
					imp.Prefix = p.unquoteString(tokens[i+1])
					i += 2
				}
			case "revision-date":
				if i+1 < len(tokens) {
					imp.RevisionDate = p.unquoteString(tokens[i+1])
					i += 2
				}
			default:
				i++
			}
		}
		if i < len(tokens) && tokens[i] == "}" {
			i++
		}
	}

	return imp, i
}

// parseTypedef parses a typedef statement (RFC 7.3)
func (p *RFC6020Parser) parseTypedef(tokens []string) (*RFC6020Typedef, int) {
	if len(tokens) < 2 {
		return nil, 1
	}

	td := &RFC6020Typedef{
		Name: p.unquoteString(tokens[1]),
	}

	i := 2
	if i < len(tokens) && tokens[i] == "{" {
		i++
		for i < len(tokens) && tokens[i] != "}" {
			switch tokens[i] {
			case "type":
				typ, consumed := p.parseType(tokens[i:])
				if typ != nil {
					td.Type = typ
				}
				i += consumed
			case "units":
				if i+1 < len(tokens) {
					td.Units = p.unquoteString(tokens[i+1])
					i += 2
				}
			case "default":
				if i+1 < len(tokens) {
					td.Default = p.unquoteString(tokens[i+1])
					i += 2
				}
			case "description":
				if i+1 < len(tokens) {
					td.Description = p.unquoteString(tokens[i+1])
					i += 2
				}
			default:
				i++
			}
		}
		if i < len(tokens) && tokens[i] == "}" {
			i++
		}
	}

	return td, i
}

// parseType parses a type statement with all restrictions (RFC 7.4)
func (p *RFC6020Parser) parseType(tokens []string) (*RFC6020Type, int) {
	if len(tokens) < 2 {
		return nil, 1
	}

	typ := &RFC6020Type{
		Name: p.unquoteString(tokens[1]),
	}

	i := 2
	if i < len(tokens) && tokens[i] == "{" {
		i++
		for i < len(tokens) && tokens[i] != "}" {
			switch tokens[i] {
			case "range":
				if i+1 < len(tokens) {
					ranges := p.parseRangeExpression(tokens[i+1])
					typ.Ranges = ranges
					i += 2
				}
			case "length":
				if i+1 < len(tokens) {
					lengths := p.parseRangeExpression(tokens[i+1])
					typ.Lengths = lengths
					i += 2
				}
			case "pattern":
				if i+1 < len(tokens) {
					pattern := RFC6020Pattern{
						Value: p.unquoteString(tokens[i+1]),
					}
					typ.Patterns = append(typ.Patterns, pattern)
					i += 2
				}
			case "enum":
				if i+1 < len(tokens) {
					enum := RFC6020Enum{
						Name: p.unquoteString(tokens[i+1]),
					}
					i += 2
					// Check if there's a block for the enum
					if i < len(tokens) && tokens[i] == "{" {
						i++ // Skip opening brace
						for i < len(tokens) && tokens[i] != "}" {
							switch tokens[i] {
							case "description":
								if i+1 < len(tokens) {
									enum.Description = p.unquoteString(tokens[i+1])
									i += 2
									// Skip semicolon if present
									if i < len(tokens) && tokens[i] == ";" {
										i++
									}
								}
							case "value":
								if i+1 < len(tokens) {
									if val, err := strconv.ParseInt(tokens[i+1], 10, 64); err == nil {
										enum.Value = &val
									}
									i += 2
									// Skip semicolon if present
									if i < len(tokens) && tokens[i] == ";" {
										i++
									}
								}
							default:
								i++
							}
						}
						if i < len(tokens) && tokens[i] == "}" {
							i++ // Skip closing brace
						}
					}
					typ.Enums = append(typ.Enums, enum)
				}
			case "bit":
				if i+1 < len(tokens) {
					bit := RFC6020Bit{
						Name: p.unquoteString(tokens[i+1]),
					}
					typ.Bits = append(typ.Bits, bit)
					i += 2
				}
			case "fraction-digits":
				if i+1 < len(tokens) {
					if fd, err := strconv.Atoi(tokens[i+1]); err == nil {
						typ.FractionDigits = fd
					}
					i += 2
				}
			case "path":
				if i+1 < len(tokens) {
					typ.Path = p.unquoteString(tokens[i+1])
					i += 2
				}
			default:
				i++
			}
		}
		if i < len(tokens) && tokens[i] == "}" {
			i++
		}
	}

	return typ, i
}

// parseFeature parses a feature statement (RFC 7.20.1)
func (p *RFC6020Parser) parseFeature(tokens []string) (*RFC6020Feature, int) {
	if len(tokens) < 2 {
		return nil, 1
	}

	feat := &RFC6020Feature{
		Name: p.unquoteString(tokens[1]),
	}

	i := 2
	if i < len(tokens) && tokens[i] == "{" {
		i++
		for i < len(tokens) && tokens[i] != "}" {
			switch tokens[i] {
			case "description":
				if i+1 < len(tokens) {
					feat.Description = p.unquoteString(tokens[i+1])
					i += 2
				}
			default:
				i++
			}
		}
		if i < len(tokens) && tokens[i] == "}" {
			i++
		}
	}

	return feat, i
}

// parseDataNode parses data definition statements (RFC 4.2.2)
func (p *RFC6020Parser) parseDataNode(tokens []string, parentPath string) (*RFC6020DataNode, int) {
	if len(tokens) < 2 {
		return nil, 1
	}

	node := &RFC6020DataNode{
		NodeType: tokens[0],
		Name:     p.unquoteString(tokens[1]),
		Children: make(map[string]*RFC6020DataNode),
	}

	i := 2
	if i < len(tokens) && tokens[i] == "{" {
		i++
		for i < len(tokens) && tokens[i] != "}" {
			switch tokens[i] {
			case "type":
				typ, consumed := p.parseType(tokens[i:])
				if typ != nil {
					node.Type = typ
				}
				i += consumed
			case "key":
				if i+1 < len(tokens) {
					keyStr := p.unquoteString(tokens[i+1])
					node.Keys = strings.Fields(keyStr)
					i += 2
				}
			case "config":
				if i+1 < len(tokens) {
					config := p.unquoteString(tokens[i+1]) == "true"
					node.Config = &config
					i += 2
				}
			case "mandatory":
				if i+1 < len(tokens) {
					mandatory := p.unquoteString(tokens[i+1]) == "true"
					node.Mandatory = &mandatory
					i += 2
				}
			case "description":
				if i+1 < len(tokens) {
					node.Description = p.unquoteString(tokens[i+1])
					i += 2
				}
			case "units":
				if i+1 < len(tokens) {
					node.Units = p.unquoteString(tokens[i+1])
					i += 2
				}
			case "container", "leaf", "leaf-list", "list":
				childPath := parentPath + "/" + node.Name
				child, consumed := p.parseDataNode(tokens[i:], childPath)
				if child != nil {
					node.Children[child.Name] = child
					child.Path = childPath + "/" + child.Name
				}
				i += consumed
			default:
				i++
			}
		}
		if i < len(tokens) && tokens[i] == "}" {
			i++
		}
	}

	return node, i
}

// performSemanticAnalysis analyzes the parsed module for semantic information
func (p *RFC6020Parser) performSemanticAnalysis(module *RFC6020Module) {
	// Analyze data nodes for keys, types, and semantics
	p.analyzeDataNodes(module, module.DataNodes, "")

	// Resolve all type references
	p.resolveTypes(module)

	// Classify metrics as counters or gauges
	p.classifyMetrics(module)

	p.logger.Printf("Semantic analysis complete for module %s: %d keyed paths, %d data types",
		module.Name, len(module.KeyedPaths), len(module.DataTypes))
}

// analyzeDataNodes recursively analyzes data nodes
func (p *RFC6020Parser) analyzeDataNodes(module *RFC6020Module, nodes map[string]*RFC6020DataNode, parentPath string) {
	for _, node := range nodes {
		fullPath := parentPath + "/" + node.Name

		// Handle lists with keys
		if node.NodeType == "list" && len(node.Keys) > 0 {
			module.KeyedPaths[fullPath] = node.Keys[0] // Primary key
			module.ListKeys[fullPath] = node.Keys
		}

		// Classify as config or state data
		if node.Config != nil {
			if *node.Config {
				module.ConfigPaths = append(module.ConfigPaths, fullPath)
			} else {
				module.StatePaths = append(module.StatePaths, fullPath)
			}
		}

		// Recursively process children
		if len(node.Children) > 0 {
			p.analyzeDataNodes(module, node.Children, fullPath)
		}
	}
}

// resolveTypes resolves all type references to their base built-in types
func (p *RFC6020Parser) resolveTypes(module *RFC6020Module) {
	// Resolve typedef types
	for _, typedef := range module.Typedefs {
		if typedef.Type != nil {
			resolved := p.resolveTypeRecursive(typedef.Type, module)
			if resolved != nil {
				module.DataTypes[typedef.Name] = resolved
			}
		}
	}

	// Resolve data node types
	p.resolveDataNodeTypes(module, module.DataNodes, "", module)
}

// resolveDataNodeTypes resolves types for all data nodes
func (p *RFC6020Parser) resolveDataNodeTypes(module *RFC6020Module, nodes map[string]*RFC6020DataNode, parentPath string, currentModule *RFC6020Module) {
	for _, node := range nodes {
		fullPath := parentPath + "/" + node.Name

		if node.Type != nil {
			resolved := p.resolveTypeRecursive(node.Type, currentModule)
			if resolved != nil {
				resolved.Description = node.Description
				// Use node units if available, otherwise keep typedef units
				if node.Units != "" {
					resolved.Units = node.Units
				}

				// Set configuration vs state
				if node.Config != nil {
					resolved.IsConfiguration = *node.Config
					resolved.IsState = !(*node.Config)
				} else {
					// Default to configuration true if not specified
					resolved.IsConfiguration = true
					resolved.IsState = false
				}

				module.DataTypes[fullPath] = resolved
			}
		}

		// Recursively process children
		if len(node.Children) > 0 {
			// Propagate config setting to children if parent has config false
			for _, child := range node.Children {
				if child.Config == nil && node.Config != nil && !*node.Config {
					config := false
					child.Config = &config
				}
			}
			p.resolveDataNodeTypes(module, node.Children, fullPath, currentModule)
		}
	}
}

// resolveTypeRecursive recursively resolves a type to its base built-in type
func (p *RFC6020Parser) resolveTypeRecursive(yangType *RFC6020Type, module *RFC6020Module) *RFC6020ResolvedType {
	if yangType == nil {
		return nil
	}

	resolved := &RFC6020ResolvedType{
		OriginalType: yangType.Name,
		ResolvedType: yangType.Name,
	}

	// Check if it's a built-in type
	if builtin, exists := p.builtinTypes[yangType.Name]; exists {
		resolved.BaseBuiltinType = builtin.Name
		resolved.ResolvedType = builtin.Name

		// Copy restrictions
		if len(yangType.Ranges) > 0 {
			resolved.Range = &yangType.Ranges[0] // Simplified
		}

		if len(yangType.Enums) > 0 {
			resolved.Enumeration = make(map[string]int64)
			for _, enum := range yangType.Enums {
				if enum.Value != nil {
					resolved.Enumeration[enum.Name] = *enum.Value
				}
			}
		}
		resolved.FractionDigits = yangType.FractionDigits
	} else if typedef, exists := module.Typedefs[yangType.Name]; exists {
		// Recursively resolve typedef
		baseResolved := p.resolveTypeRecursive(typedef.Type, module)
		if baseResolved != nil {
			resolved.BaseBuiltinType = baseResolved.BaseBuiltinType
			resolved.ResolvedType = baseResolved.ResolvedType
			if resolved.Units == "" {
				resolved.Units = typedef.Units
			}
			// Propagate fraction digits from typedef
			if baseResolved.FractionDigits > 0 {
				resolved.FractionDigits = baseResolved.FractionDigits
			} else if typedef.Type.FractionDigits > 0 {
				resolved.FractionDigits = typedef.Type.FractionDigits
			}
		}
	}

	return resolved
}

// classifyMetrics classifies data types as counters or gauges based on semantic analysis
func (p *RFC6020Parser) classifyMetrics(module *RFC6020Module) {
	for path, dataType := range module.DataTypes {
		if dataType.BaseBuiltinType == "" {
			continue
		}

		isCounter := p.isCounterSemantic(dataType)
		isGauge := p.isGaugeSemantic(dataType)

		switch {
		case isCounter:
			dataType.IsCounter = true
			dataType.SemanticType = "counter"
			module.Counters = append(module.Counters, path)
		case isGauge:
			dataType.IsGauge = true
			dataType.SemanticType = "gauge"
			module.Gauges = append(module.Gauges, path)
		default:
			dataType.SemanticType = "info"
		}
	}
}

// isCounterSemantic determines if a data type represents a counter metric
func (*RFC6020Parser) isCounterSemantic(dataType *RFC6020ResolvedType) bool {
	// Counters are typically unsigned integers with accumulating units
	if !strings.HasPrefix(dataType.BaseBuiltinType, "uint") {
		return false
	}

	// Check for rate units (these are gauges, not counters)
	rateUnits := []string{"per-second", "pps", "bps", "kbps", "mbps", "gbps", "rate"}
	for _, rate := range rateUnits {
		if strings.Contains(strings.ToLower(dataType.Units), rate) {
			return false
		}
	}

	// Counter units
	counterUnits := []string{"bytes", "octets", "packets", "count", "total", "errors", "discards", "drops"}
	for _, counter := range counterUnits {
		if strings.Contains(strings.ToLower(dataType.Units), counter) {
			return true
		}
	}

	return false
}

// isGaugeSemantic determines if a data type represents a gauge metric
func (*RFC6020Parser) isGaugeSemantic(dataType *RFC6020ResolvedType) bool {
	// Gauge units include rates, percentages, current values
	gaugeUnits := []string{
		"percent", "per-second", "pps", "bps", "kbps", "mbps", "gbps",
		"utilization", "rate", "current", "level", "temperature",
		"voltage", "frequency", "load", "usage",
	}

	for _, gauge := range gaugeUnits {
		if strings.Contains(strings.ToLower(dataType.Units), gauge) {
			return true
		}
	}

	return false
}

// Helper functions

func (*RFC6020Parser) unquoteString(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func (p *RFC6020Parser) parseRangeExpression(expr string) []RFC6020Range {
	expr = p.unquoteString(expr)
	parts := strings.Split(expr, "|")
	var ranges []RFC6020Range

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "..") {
			bounds := strings.Split(part, "..")
			if len(bounds) == 2 {
				ranges = append(ranges, RFC6020Range{
					Min: strings.TrimSpace(bounds[0]),
					Max: strings.TrimSpace(bounds[1]),
				})
			}
		} else {
			// Single value range
			ranges = append(ranges, RFC6020Range{
				Min: part,
				Max: part,
			})
		}
	}

	return ranges
}

// Public API methods

// GetModules returns all loaded modules
func (p *RFC6020Parser) GetModules() map[string]*RFC6020Module {
	return p.modules
}

// GetModuleByName returns a specific module by name
func (p *RFC6020Parser) GetModuleByName(name string) *RFC6020Module {
	return p.modules[name]
}

// GetBuiltinTypes returns all YANG built-in types
func (p *RFC6020Parser) GetBuiltinTypes() map[string]*RFC6020BuiltinType {
	return p.builtinTypes
}

// ExportModules exports all modules to JSON for external use
func (p *RFC6020Parser) ExportModules() ([]byte, error) {
	return json.MarshalIndent(p.modules, "", "  ")
}

// SaveModules saves all modules to a file
func (p *RFC6020Parser) SaveModules(filename string) error {
	data, err := p.ExportModules()
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0o600)
}

// AnalyzeTelemetryPath analyzes a telemetry encoding path and provides YANG context
func (p *RFC6020Parser) AnalyzeTelemetryPath(encodingPath string) *RFC6020TelemetryAnalysis {
	// Extract module name from encoding path (e.g., "Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics")
	parts := strings.SplitN(encodingPath, ":", 2)
	if len(parts) != 2 {
		return &RFC6020TelemetryAnalysis{
			EncodingPath: encodingPath,
			IsValid:      false,
			ErrorReason:  "Invalid encoding path format - missing module prefix",
		}
	}

	moduleName := parts[0]
	xpath := parts[1]

	// Check if we have this module loaded
	module := p.modules[moduleName]
	if module == nil {
		// Create a dynamic module for unknown modules
		module = p.createDynamicModule(moduleName, encodingPath)
		p.modules[moduleName] = module
		p.logger.Printf("Created dynamic YANG module for: %s", moduleName)
	}

	// Parse the XPath to identify data nodes and list keys
	pathSegments := strings.Split(strings.Trim(xpath, "/"), "/")

	analysis := &RFC6020TelemetryAnalysis{
		EncodingPath:    encodingPath,
		ModuleName:      moduleName,
		XPath:           xpath,
		PathSegments:    pathSegments,
		IsValid:         true,
		Module:          module,
		DataNodes:       make(map[string]*RFC6020DataNode),
		SemanticContext: make(map[string]string),
	}

	// Build the full path and identify list paths
	fullPath := ""
	for _, segment := range pathSegments {
		if fullPath != "" {
			fullPath += "/"
		}
		fullPath += segment

		// Check if this segment represents a list in the module
		if dataNode := p.findDataNodeByPath(module, fullPath); dataNode != nil {
			analysis.DataNodes[fullPath] = dataNode
			if dataNode.NodeType == "list" {
				analysis.ListPath = "/" + fullPath
				// Extract list keys if available
				if len(dataNode.Keys) > 0 {
					analysis.ListKeys = dataNode.Keys
				}
			}
		}
	}

	// Set default semantic classifications for known operational data patterns
	p.applySemanticHeuristics(analysis)

	return analysis
}

// createDynamicModule creates a YANG module definition for unknown modules based on encoding path
func (p *RFC6020Parser) createDynamicModule(moduleName, encodingPath string) *RFC6020Module {
	module := &RFC6020Module{
		Name:        moduleName,
		Namespace:   fmt.Sprintf("urn:ietf:params:xml:ns:yang:%s", moduleName),
		Prefix:      moduleName,
		YangVersion: "1.0",
		Description: fmt.Sprintf("Dynamically created module for %s based on telemetry data", moduleName),
		DataNodes:   make(map[string]*RFC6020DataNode),
		Typedefs:    make(map[string]*RFC6020Typedef),
		Groupings:   make(map[string]*RFC6020Grouping),
		Features:    make(map[string]*RFC6020Feature),
		Imports:     make(map[string]*RFC6020Import),
		Includes:    make(map[string]*RFC6020Include),
	}

	// Add a revision
	module.Revisions = []*RFC6020Revision{{
		Date:        "2024-01-01",
		Description: "Dynamic module creation from telemetry data",
	}}

	// Extract path and create basic data node structure
	parts := strings.SplitN(encodingPath, ":", 2)
	if len(parts) == 2 {
		xpath := parts[1]
		p.createDataNodesFromPath(module, xpath)
	}

	return module
}

// createDataNodesFromPath creates basic data node structure from XPath
func (p *RFC6020Parser) createDataNodesFromPath(module *RFC6020Module, xpath string) {
	segments := strings.Split(strings.Trim(xpath, "/"), "/")
	currentPath := ""

	for i, segment := range segments {
		if currentPath != "" {
			currentPath += "/"
		}
		currentPath += segment

		// Create a basic data node if it doesn't exist
		if _, exists := module.DataNodes[currentPath]; !exists {
			nodeType := "leaf"
			if i < len(segments)-1 {
				nodeType = "container"
			}

			// Detect if this is likely a list based on common patterns
			if p.isLikelyListNode(segment, segments, i) {
				nodeType = "list"
			}

			config := false
			mandatory := false
			dataNode := &RFC6020DataNode{
				Name:        segment,
				NodeType:    nodeType,
				Path:        "/" + currentPath,
				Description: fmt.Sprintf("Auto-generated %s for %s", nodeType, segment),
				Type:        p.InferDataTypeFromPath(segment),
				Children:    make(map[string]*RFC6020DataNode),
				Config:      &config, // Operational data
				Mandatory:   &mandatory,
			}

			// Add common list keys for known patterns
			if nodeType == "list" {
				dataNode.Keys = p.inferListKeys(segment)
			}

			module.DataNodes[currentPath] = dataNode
		}
	}
}

// isLikelyListNode determines if a path segment represents a list
func (*RFC6020Parser) isLikelyListNode(segment string, allSegments []string, index int) bool {
	// Common list node patterns
	listPatterns := []string{
		"interface", "interfaces", "interface-state",
		"neighbor", "neighbors", "peer", "peers",
		"route", "routes", "entry", "entries",
		"session", "sessions", "connection", "connections",
		"policy", "policies", "rule", "rules",
		"memory-statistic", "cpu-usage", "process",
	}

	lowerSegment := strings.ToLower(segment)
	for _, pattern := range listPatterns {
		if strings.Contains(lowerSegment, pattern) {
			return true
		}
	}

	// If followed by what looks like statistics or state, probably a list
	if index < len(allSegments)-1 {
		nextSegment := strings.ToLower(allSegments[index+1])
		if strings.Contains(nextSegment, "statistic") ||
			strings.Contains(nextSegment, "state") ||
			strings.Contains(nextSegment, "status") {
			return true
		}
	}

	return false
}

// inferListKeys infers likely key fields for list nodes
func (*RFC6020Parser) inferListKeys(segment string) []string {
	lowerSegment := strings.ToLower(segment)

	// Common key patterns based on list type
	keyMappings := map[string][]string{
		"interface":        {"name"},
		"interfaces":       {"name"},
		"interface-state":  {"name"},
		"neighbor":         {"address"},
		"neighbors":        {"address"},
		"peer":             {"id", "address"},
		"peers":            {"id", "address"},
		"route":            {"prefix"},
		"routes":           {"prefix"},
		"memory-statistic": {"name"},
		"cpu-usage":        {"id"},
		"process":          {"pid", "name"},
		"session":          {"id"},
		"entry":            {"id"},
	}

	for pattern, keys := range keyMappings {
		if strings.Contains(lowerSegment, pattern) {
			return keys
		}
	}

	// Default key
	return []string{"name"}
}

// InferDataTypeFromPath infers YANG data type from path segment
func (*RFC6020Parser) InferDataTypeFromPath(segment string) *RFC6020Type {
	lowerSegment := strings.ToLower(segment)

	// Infer type based on common naming patterns
	if strings.Contains(lowerSegment, "count") ||
		strings.Contains(lowerSegment, "total") ||
		strings.Contains(lowerSegment, "bytes") ||
		strings.Contains(lowerSegment, "packets") ||
		strings.Contains(lowerSegment, "errors") ||
		strings.Contains(lowerSegment, "drops") {
		return &RFC6020Type{
			Name: "uint64",
		}
	}

	if strings.Contains(lowerSegment, "rate") ||
		strings.Contains(lowerSegment, "pps") ||
		strings.Contains(lowerSegment, "bps") ||
		strings.Contains(lowerSegment, "kbps") ||
		strings.Contains(lowerSegment, "mbps") ||
		strings.Contains(lowerSegment, "usage") ||
		strings.Contains(lowerSegment, "utilization") {
		return &RFC6020Type{
			Name: "uint32",
		}
	}

	if strings.Contains(lowerSegment, "name") ||
		strings.Contains(lowerSegment, "description") ||
		strings.Contains(lowerSegment, "type") ||
		strings.Contains(lowerSegment, "status") ||
		strings.Contains(lowerSegment, "state") {
		return &RFC6020Type{
			Name: "string",
		}
	}

	if strings.Contains(lowerSegment, "time") ||
		strings.Contains(lowerSegment, "timestamp") {
		return &RFC6020Type{
			Name: "yang:date-and-time",
		}
	}

	// Default to string for unknown types
	return &RFC6020Type{
		Name: "string",
	}
} // findDataNodeByPath finds a data node by its path in the module
func (*RFC6020Parser) findDataNodeByPath(module *RFC6020Module, path string) *RFC6020DataNode {
	cleanPath := strings.Trim(path, "/")
	return module.DataNodes[cleanPath]
}

// applySemanticHeuristics applies semantic classification heuristics
func (*RFC6020Parser) applySemanticHeuristics(analysis *RFC6020TelemetryAnalysis) {
	// Extract the leaf name (last segment)
	if len(analysis.PathSegments) > 0 {
		leafName := analysis.PathSegments[len(analysis.PathSegments)-1]
		lowerLeaf := strings.ToLower(leafName)

		// Counter patterns
		if strings.Contains(lowerLeaf, "count") ||
			strings.Contains(lowerLeaf, "total") ||
			strings.Contains(lowerLeaf, "bytes") ||
			strings.Contains(lowerLeaf, "packets") ||
			strings.Contains(lowerLeaf, "errors") ||
			strings.Contains(lowerLeaf, "drops") ||
			strings.Contains(lowerLeaf, "discards") ||
			strings.Contains(lowerLeaf, "octets") {
			analysis.SemanticType = "counter"
		}

		// Gauge patterns
		if strings.Contains(lowerLeaf, "rate") ||
			strings.Contains(lowerLeaf, "pps") ||
			strings.Contains(lowerLeaf, "bps") ||
			strings.Contains(lowerLeaf, "kbps") ||
			strings.Contains(lowerLeaf, "mbps") ||
			strings.Contains(lowerLeaf, "usage") ||
			strings.Contains(lowerLeaf, "utilization") ||
			strings.Contains(lowerLeaf, "load") {
			analysis.SemanticType = "gauge"
		}

		// Info patterns
		if strings.Contains(lowerLeaf, "name") ||
			strings.Contains(lowerLeaf, "description") ||
			strings.Contains(lowerLeaf, "type") ||
			strings.Contains(lowerLeaf, "status") ||
			strings.Contains(lowerLeaf, "state") ||
			strings.Contains(lowerLeaf, "time") {
			analysis.SemanticType = "info"
		}

		// Default to gauge if no clear pattern
		if analysis.SemanticType == "" {
			analysis.SemanticType = "gauge"
		}
	}
}

// RFC6020TelemetryAnalysis represents the analysis of a telemetry encoding path
type RFC6020TelemetryAnalysis struct {
	EncodingPath    string                      `json:"encoding_path"`
	ModuleName      string                      `json:"module_name"`
	XPath           string                      `json:"xpath"`
	PathSegments    []string                    `json:"path_segments"`
	ListPath        string                      `json:"list_path"`
	ListKeys        []string                    `json:"list_keys"`
	SemanticType    string                      `json:"semantic_type"` // "counter", "gauge", "info"
	DataNodes       map[string]*RFC6020DataNode `json:"data_nodes"`
	SemanticContext map[string]string           `json:"semantic_context"`
	IsValid         bool                        `json:"is_valid"`
	ErrorReason     string                      `json:"error_reason,omitempty"`
	Module          *RFC6020Module              `json:"module,omitempty"`
}
