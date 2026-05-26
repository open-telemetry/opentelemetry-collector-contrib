// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package schemagen

import (
	"go/ast"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

func extractDescriptionFromComment(group *ast.CommentGroup) (string, bool) {
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

func goPrimitiveToSchemaType(typeName string) (schemaType, bool) {
	switch typeName {
	case "string":
		return schemaTypeString, false
	case "rune", "byte":
		return schemaTypeString, true
	case "int", "uint", "int8", "uint8", "int16", "uint16", "int32", "uint32", "int64", "uint64":
		return schemaTypeInteger, typeName != "int"
	case "float32", "float64":
		return schemaTypeNumber, true
	case "bool":
		return schemaTypeBoolean, false
	case "any":
		return schemaTypeAny, true
	default:
		return schemaTypeUnknown, false
	}
}

var importRegExp = regexp.MustCompile(`^(.+?)(?:/([^/"]+))?$`)

func parseImport(imp *ast.ImportSpec) (string, string) {
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

type tagInfo struct {
	Name      string
	OmitEmpty bool
	Squash    bool
}

func parseTag(tag *ast.BasicLit) (*tagInfo, bool) {
	if tag == nil {
		return nil, false
	}
	unquoted, err := strconv.Unquote(tag.Value)
	if err != nil {
		unquoted = strings.Trim(tag.Value, "`")
	}
	if unquoted == "" {
		return nil, false
	}

	structTag := reflect.StructTag(unquoted)
	mapstructureTag := structTag.Get("mapstructure")
	if mapstructureTag == "" {
		return nil, false
	}

	parts := strings.Split(mapstructureTag, ",")
	info := &tagInfo{
		Name: parts[0],
	}
	if info.Name == "-" {
		return nil, false
	}
	for _, part := range parts[1:] {
		if part == "omitempty" {
			info.OmitEmpty = true
		}
		if part == "squash" {
			info.Squash = true
		}
	}
	return info, true
}
