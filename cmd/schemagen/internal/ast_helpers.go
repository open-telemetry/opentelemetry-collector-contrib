// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"errors"
	"go/ast"
	"net/url"
	"os/exec"
	"path/filepath"
	"reflect"
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

func goPrimitiveToSchemaType(typeName string) SchemaType {
	switch typeName {
	case "string":
		return SchemaTypeString
	case "int", "int8", "int16", "int32", "int64":
		return SchemaTypeInteger
	case "float32", "float64":

		return SchemaTypeNumber
	case "bool":
		return SchemaTypeBoolean
	case "any":
		return ""
	default:
		return SchemaTypeNull
	}
}

func GetSchemaID(file *ast.File, cfg *Config) (string, error) {
	absolutePath, _ := filepath.Abs(cfg.DirPath)
	basePath, err := getBasePath(absolutePath)
	relPath := ""
	if err == nil {
		relPath, _ = filepath.Rel(basePath, absolutePath)
	}

	if cfg.SchemaIDPrefix != "" {
		id, e := url.JoinPath(cfg.SchemaIDPrefix, relPath, filepath.Base(cfg.SchemaPath))
		if e != nil {
			return "", e
		}
		return filepath.ToSlash(id), nil
	}

	packageImportPath, err := getPackageImportPath(file)
	if err == nil {
		rootURL := strings.TrimSuffix(packageImportPath, filepath.ToSlash(relPath))
		id, e := url.JoinPath("https://", rootURL, "blob/main", relPath, filepath.Base(cfg.SchemaPath))
		if e != nil {
			return "", e
		}
		return filepath.ToSlash(id), nil
	}

	return "", err
}

func getBasePath(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.Trim(filepath.ToSlash(string(output)), "\n"), nil
}

func getPackageImportPath(file *ast.File) (string, error) {
	for _, commentGroup := range file.Comments {
		for _, comment := range commentGroup.List {
			importPrefix := "// import "
			if len(comment.Text) > len(importPrefix) && comment.Text[:len(importPrefix)] == importPrefix {
				return comment.Text[len(importPrefix)+1 : len(comment.Text)-1], nil
			}
		}
	}
	return "", errors.New("could not find import path, please provide SchemaIDPrefix with the flag")
}
