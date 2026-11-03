// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package signingprocessor

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

const (
	vectorsPinFile = "testdata/rfc8785/requirements.txt"
	vectorsWheel   = "https://files.pythonhosted.org/packages/py3/a/agent-evidence-vectors/agent_evidence_vectors-%s-py3-none-any.whl"
	a2aCorpus      = "agent_evidence_vectors/corpora/vectors-a2a-jcs-v01"
	byteCorpus     = "agent_evidence_vectors/corpora/corpora/jcs-byte-vectors/cases.json"

	// minVectors stops a bump that drops vectors from passing on fewer of them.
	minVectors = 50
)

var (
	pinnedVersion = regexp.MustCompile(`agent-evidence-vectors==(\S+)`)
	canonicalLeaf = regexp.MustCompile(`"(?:[^"\\]|\\.)*":?|true|false|-?\d[-+.0-9e]*`)
)

// rfc8785Vector is one published input and the bytes RFC 8785 requires for it.
type rfc8785Vector struct {
	id           string
	input        []byte
	canonicalHex string
}

// TestRFC8785Vectors runs the published RFC 8785 vectors through
// serializeLogRecord, so a jcs or Go bump that moves the signed bytes fails
// here. The vectors live in a package pinned by version and hash, which
// Renovate bumps.
//
// Only MUST-ACCEPT inputs are run. Lone surrogates and non-finite numbers stop
// before jcs.Transform, and TestInvalidUTF8Rejected and
// TestFloat64SpecialValuesRejected cover them. The A2 group is left out because
// it tests removing the signatures member of an A2A Agent Card.
func TestRFC8785Vectors(t *testing.T) {
	wheel := fetchVectorsWheel(t)
	vectors := append(a2aVectors(t, wheel), byteVectors(t, wheel)...)
	require.GreaterOrEqual(t, len(vectors), minVectors)
	t.Logf("running %d vectors", len(vectors))

	p := &signingProcessor{}
	for _, vector := range vectors {
		t.Run(vector.id, func(t *testing.T) {
			var decoded any
			require.NoError(t, json.Unmarshal(vector.input, &decoded))
			value := pcommon.NewValueEmpty()
			require.NoError(t, value.FromRaw(decoded))
			lr := plog.NewLogRecord()
			value.CopyTo(lr.Attributes().PutEmpty("v"))

			got, err := p.serializeLogRecord(lr)
			require.NoError(t, err)

			canonical, err := hex.DecodeString(vector.canonicalHex)
			require.NoError(t, err)
			require.Equal(t, `{"attributes":{"v":`+wrapLeaves(string(canonical))+`}}`, string(got))
		})
	}
}

// wrapLeaves gives canonical JSON the shape valueToInterface gives the same
// value: each scalar becomes a one-member object, and member order is unchanged.
func wrapLeaves(canonical string) string {
	return canonicalLeaf.ReplaceAllStringFunc(canonical, func(leaf string) string {
		switch {
		case strings.HasSuffix(leaf, `":`):
			return leaf // a member name
		case leaf[0] == '"':
			return `{"stringValue":` + leaf + `}`
		case leaf == "true" || leaf == "false":
			return `{"boolValue":` + leaf + `}`
		default:
			return `{"doubleValue":` + leaf + `}`
		}
	})
}

func fetchVectorsWheel(t *testing.T) *zip.Reader {
	t.Helper()
	pin, err := os.ReadFile(vectorsPinFile)
	require.NoError(t, err)
	version := pinnedVersion.FindSubmatch(pin)
	require.NotNil(t, version, "no pinned version in %s", vectorsPinFile)

	url := fmt.Sprintf(vectorsWheel, version[1])
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	require.NoError(t, err)
	client := &http.Client{
		Timeout:   2 * time.Minute,
		Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, DisableKeepAlives: true},
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, url)
	wheel, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	sum := sha256.Sum256(wheel)
	require.Contains(t, string(pin), "sha256:"+hex.EncodeToString(sum[:]), "%s does not match %s", url, vectorsPinFile)
	reader, err := zip.NewReader(bytes.NewReader(wheel), int64(len(wheel)))
	require.NoError(t, err)
	return reader
}

func a2aVectors(t *testing.T, wheel fs.FS) []rfc8785Vector {
	t.Helper()
	var manifest struct {
		Vectors []struct {
			Path string `json:"path"`
		} `json:"vectors"`
	}
	readJSON(t, wheel, a2aCorpus+"/MANIFEST.json", &manifest)

	var vectors []rfc8785Vector
	for _, entry := range manifest.Vectors {
		if strings.HasPrefix(entry.Path, "a2-") {
			continue
		}
		var vector struct {
			ID          string          `json:"id"`
			Disposition string          `json:"disposition"`
			Input       json.RawMessage `json:"input"`
			Expected    struct {
				CanonicalHex string `json:"canonical_utf8_hex"`
			} `json:"expected"`
		}
		readJSON(t, wheel, a2aCorpus+"/"+entry.Path, &vector)
		if vector.Disposition == "MUST-ACCEPT" {
			vectors = append(vectors, rfc8785Vector{vector.ID, vector.Input, vector.Expected.CanonicalHex})
		}
	}
	return vectors
}

func byteVectors(t *testing.T, wheel fs.FS) []rfc8785Vector {
	t.Helper()
	var cases struct {
		CanonicalCases []struct {
			ID           string `json:"id"`
			Input        string `json:"input"`
			CanonicalHex string `json:"canonicalHex"`
		} `json:"canonicalCases"`
	}
	readJSON(t, wheel, byteCorpus, &cases)

	var vectors []rfc8785Vector
	for _, c := range cases.CanonicalCases {
		vectors = append(vectors, rfc8785Vector{c.ID, []byte(c.Input), c.CanonicalHex})
	}
	return vectors
}

func readJSON(t *testing.T, wheel fs.FS, name string, into any) {
	t.Helper()
	data, err := fs.ReadFile(wheel, name)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, into), name)
}
