package genainormalizerprocessor

import (
	"strings"
	"testing"
)

func TestValidate_ValidConfig(t *testing.T) {
	c := &Config{
		SemConvVersion:  "1.39.0",
		Profiles:        []string{"openinference", "openllmetry"},
		RemoveOriginals: true,
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_EmptySemConvVersion(t *testing.T) {
	c := &Config{
		Profiles: []string{"openinference"},
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("empty semconv_version should be allowed: %v", err)
	}
}

func TestValidate_UnsupportedSemConvVersion(t *testing.T) {
	c := &Config{
		SemConvVersion: "99.0.0",
		Profiles:       []string{"openinference"},
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("expected error for unsupported semconv_version")
	}
	if !strings.Contains(err.Error(), "99.0.0") {
		t.Errorf("error should mention the bad version: %v", err)
	}
}

func TestValidate_NoProfiles(t *testing.T) {
	c := &Config{Profiles: []string{}}
	err := c.Validate()
	if err == nil {
		t.Fatal("expected error for empty profiles")
	}
	if !strings.Contains(err.Error(), "at least one profile") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_UnknownProfile(t *testing.T) {
	c := &Config{Profiles: []string{"openinference", "bogus"}}
	err := c.Validate()
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should mention the bad profile: %v", err)
	}
}
