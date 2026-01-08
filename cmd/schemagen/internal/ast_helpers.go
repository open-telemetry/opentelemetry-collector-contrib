// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"go/ast"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

func ExtractNameFromTag(field *ast.Field) (string, bool) {
	if field.Tag == nil {
		return "", false
	}

	tagLiteral := field.Tag.Value
	if tagLiteral == "" {
		return "", false
	}

	unquoted, err := strconv.Unquote(tagLiteral)
	if err != nil {
		unquoted = strings.Trim(tagLiteral, "`")
	}
	if unquoted == "" {
		return "", false
	}

	tag := reflect.StructTag(unquoted)

	if name := normalizeTagValue(tag.Get("mapstructure")); name != "" {
		return name, true
	}

	return "", false
}

func normalizeTagValue(value string) string {
	if value == "" {
		return ""
	}
	if idx := strings.IndexByte(value, ','); idx >= 0 {
		value = value[:idx]
	}
	value = strings.TrimSpace(value)
	if value == "" || value == "-" {
		return ""
	}
	return value
}

func ExtractDescriptionFromComment(group *ast.CommentGroup) (string, bool) {
	if group == nil {
		return "", false
	}
	var comments []string
	for _, comment := range group.List {
		cleaned := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
		cleaned = strings.TrimSpace(strings.TrimPrefix(cleaned, "/*"))
		cleaned = strings.TrimSpace(strings.TrimSuffix(cleaned, "*/"))
		if cleaned != "" {
			comments = append(comments, cleaned)
		}
	}
	if len(comments) == 0 {
		return "", false
	}
	return strings.Join(comments, " "), true
}

func goPrimitiveToSchemaType(typeName string) (SchemaType, bool) {
	switch typeName {
	case "string":
		return SchemaTypeString, false
	case "rune", "byte":
		return SchemaTypeString, true
	case "int", "int8", "int16", "int32", "int64":
		return SchemaTypeInteger, typeName != "int"
	case "float32", "float64":

		return SchemaTypeNumber, true
	case "bool":
		return SchemaTypeBoolean, false
	case "any":
		return SchemaTypeAny, true
	default:
		return SchemaTypeUnknown, false
	}
}

var importRegExp = regexp.MustCompile(`^(.+?)(?:/([^/"]+))?$`)

func ParseImport(imp *ast.ImportSpec) (string, string) {
	importStr := imp.Path.Value
	if unquoted, err := strconv.Unquote(imp.Path.Value); err == nil {
		importStr = unquoted
	}

	matches := importRegExp.FindStringSubmatch(importStr)
	full := matches[0]
	name := matches[2]
	if name == "" {
		name = full
	}
	if imp.Name != nil {
		name = imp.Name.Name
	}
	return full, name
}
