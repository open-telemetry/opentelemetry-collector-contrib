package oidcauthextension

import (
	"reflect"
	"testing"
)

func TestAccessingJWTClaims(t *testing.T) {
	data := authData{
		raw:        "raw-jwt",
		subject:    "test-subject",
		membership: []string{"group1", "group2"},
		claims: map[string]any{
			"tenant": "test-tenant",
		},
	}

	if data.GetAttribute("subject") != "test-subject" {
		t.Errorf("Expected subject to be 'test-subject', got '%v'", data.GetAttribute("subject"))
	}

	// Accessing JWT claim with proper prefix
	if data.GetAttribute("claims.tenant") != "test-tenant" {
		t.Errorf("Expected claims.tenant to be 'test-tenant', got '%v'", data.GetAttribute("claims.tenant"))
	}

	// Access claim without proper prefix should return nil
	if data.GetAttribute("tenant") != nil {
		t.Errorf("Expected tenant to be nil, got '%v'", data.GetAttribute("tenant"))
	}

	// Access claim with proper prefix but non-existent claim should return nil
	if data.GetAttribute("claims.nonexistent") != nil {
		t.Errorf("Expected claims.nonexistent to be nil, got '%v'", data.GetAttribute("claims.nonexistent"))
	}

	// Accessing all claims should return the claims map
	allClaims := data.GetAttribute("claims")
	expectedClaims := map[string]any{"tenant": "test-tenant"}
	if !reflect.DeepEqual(allClaims, expectedClaims) {
		t.Errorf("Expected claims to be %v, got %v", expectedClaims, allClaims)
	}

	attributes := data.GetAttributeNames()
	attributesExpected := []string{"subject", "membership", "raw", "claims", "claims.tenant"}

	if !reflect.DeepEqual(attributes, attributesExpected) {
		t.Errorf("Expected attribute names to be %v, got %v", attributesExpected, attributes)
	}
}
