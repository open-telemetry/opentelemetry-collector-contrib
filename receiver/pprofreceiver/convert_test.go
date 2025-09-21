package pprofreceiver

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/pprof/profile"
)

func TestConvertPprofToPprofile(t *testing.T) {
	tests := map[string]struct {
		expectedError error
	}{
		"cppbench.cpu": {},
		"gobench.cpu":  {},
		"java.cpu":     {},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			inbytes, err := os.ReadFile(filepath.Join("internal/testdata/", name))
			if err != nil {
				t.Fatal(err)
			}
			p, err := profile.Parse(bytes.NewBuffer(inbytes))
			if err != nil {
				t.Fatalf("%s: %s", name, err)
			}

			pprofile, err := convertPprofToPprofile(p)
			if err != nil {
				t.Fatalf("%s: %s", name, err)
			}
			_ = pprofile
			_ = tc.expectedError
		})
	}
}
