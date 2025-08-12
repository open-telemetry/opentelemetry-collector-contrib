// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8scfgprovider

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// valueSpec contains the result of the parseURI
type valueSpec struct {
	kind      string
	namespace string
	name      string
	dataType  string
	key       string
}

// parseURI parses and validates
func parseURI(uri string) (*valueSpec, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	// Test scheme
	if u.Scheme != schemeName {
		return nil, errors.New("expected k8scfg scheme")
	}
	// Opaque should be set
	if u.Opaque == "" {
		return nil, errors.New("not opaque")
	}

	parts := strings.Split(u.Opaque, "/")
	if len(parts) != 5 {
		return nil, fmt.Errorf("need 5 parts, got %d", len(parts))
	}

	vs := &valueSpec{
		kind:      parts[0],
		namespace: parts[1],
		name:      parts[2],
		dataType:  parts[3],
		key:       parts[4],
	}
	if err := vs.validate(); err != nil {
		return nil, fmt.Errorf("validate failed: %w", err)
	}
	return vs, nil
}

// The regex for a DNS-1123 label, which is required for resource names
// like Secrets and ConfigMaps.
const k8sNameRegexString = `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`

// We use MustCompile which panics if the regex is invalid.
// This is safe for a constant, known-good pattern.
var k8sNameValidator = regexp.MustCompile(k8sNameRegexString)

// isValidK8sName checks if a string is a valid Kubernetes resource name.
// It also includes the mandatory length check.
func isValidK8sName(name string) bool {
	// Rule: Must be no more than 253 characters.
	if len(name) > 253 {
		return false
	}
	// Rule: Must match the DNS-1123 label regex.
	return k8sNameValidator.MatchString(name)
}

// The regex for a valid key in a Kubernetes Secret or ConfigMap.
const k8sKeyRegexString = `^[-._a-zA-Z0-9]+$`

// Compile the regex once for efficiency.
// MustCompile panics if the regex is invalid, which is fine for a static, known-good pattern.
var k8sKeyValidator = regexp.MustCompile(k8sKeyRegexString)

// isValidK8sKey checks if a string is a valid key.
func isValidK8sKey(key string) bool {
	return k8sKeyValidator.MatchString(key)
}

// validate checks if the fields have valid values
//
//	kind should be secret or configMap (case insensitive)
//	namepace can be a valid name or a dot
//	name must be a valid name
//	dataType should be valid for the kind
//	kind: secret
//		dataType: data or stringData
//	kind: configMap:
//		dataType: data or binaryData
//	key must be a valid kubernetes name
//
// returns nil if valid and an joined error for all errors found
func (v *valueSpec) validate() error {
	// validate the namespace
	if v.namespace == "" {
		return errors.New("namespace is mandatore")
	}

	// validate the name length
	if len(v.name) > 253 {
		return errors.New("name should not be longer than 253 characters")
	}
	// validate the name
	if !isValidK8sName(v.name) {
		return fmt.Errorf("%s is not a valid k8s name", v.name)
	}

	// validate the key
	if !isValidK8sKey(v.key) {
		return fmt.Errorf("%s is not a valid key", v.key)
	}

	// validate the kind and delegate further validation
	switch strings.ToLower(v.kind) {
	case "secret":
		return v.validateSecret()
	case "configmap":
		return v.validateConfigMap()
	default:
		return fmt.Errorf("bad kind %s, valid kinds are secret and configMap", v.kind)
	}
}

func (v *valueSpec) validateConfigMap() error {
	// only check the type
	switch v.dataType {
	case "data", "binaryData":
		break
	default:
		return errors.New("configMap: need data or binaryData")
	}

	return nil
}

func (v *valueSpec) validateSecret() error {
	// only check the type
	switch v.dataType {
	case "data", "stringData":
		break
	default:
		return errors.New("secret: need data or stringData")
	}

	return nil
}
