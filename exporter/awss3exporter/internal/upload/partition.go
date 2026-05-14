// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package upload // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/upload"

import (
	"bytes"
	"io"
	"log/slog"
	"math/rand/v2"
	"path"
	"strconv"
	"strings"
	"text/template"
	"text/template/parse"
	"time"

	"github.com/google/uuid"
	"github.com/itchyny/timefmt-go"
	"go.opentelemetry.io/collector/config/configcompression"
)

// legacyTemplateFuncs provides the Sprig-compatible template functions
// used by generated configs (dateInZone, now, randAlpha).
// NOTE: the old sawmills-collector fork used the full sprig.TxtFuncMap().
// We only provide the three functions actually referenced by generated configs
// to avoid pulling in the sprig dependency.  If future templates need
// additional Sprig functions, consider adding github.com/Masterminds/sprig/v3.
var legacyTemplateFuncs = template.FuncMap{
	"now": time.Now,
	"dateInZone": func(layout string, t time.Time, zone string) string {
		loc, err := time.LoadLocation(zone)
		if err != nil {
			loc = time.UTC
		}
		return t.In(loc).Format(layout)
	},
	"randAlpha": func(n int) string {
		const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		b := make([]byte, n)
		for i := range b {
			b[i] = letters[rand.IntN(len(letters))]
		}
		return string(b)
	},
}

var compressionFileExtensions = map[configcompression.Type]string{
	configcompression.TypeGzip: ".gz",
	configcompression.TypeZstd: ".zst",
}

type PartitionKeyBuilder struct {
	// PartitionBasePrefix defines the root S3
	// directory (key) prefix used to write the file.
	PartitionBasePrefix string
	// PartitionPrefix defines the S3 directory (key)
	// prefix used to write the file.
	// Appended to PartitionBasePrefix if provided.
	PartitionPrefix string
	// LegacyS3KeyTemplate preserves the legacy key template behavior used by generated configs.
	LegacyS3KeyTemplate string
	// LegacyS3KeyTemplateParsed stores the parsed legacy template so hot-path
	// key generation does not reparse on every upload.
	LegacyS3KeyTemplateParsed *template.Template
	// PartitionFormat is used to separate values into
	// different time buckets.
	// Uses [strftime](https://www.man7.org/linux/man-pages/man3/strftime.3.html) formatting.
	PartitionFormat string
	// PartitionTimeLocation is used to provide timezone for partition time. Defaults to Local time location.
	PartitionTimeLocation *time.Location
	// FilePrefix is used to define the prefix of the file written
	// to the directory in S3.
	FilePrefix string
	// FileFormat defines what encoding was used to write
	// the content to s3
	FileFormat string
	// Metadata provides additional details regarding the file
	// Expected to be one of "metrics", "traces", or "logs"
	Metadata string
	// Compression defines algorithm used on the
	// body before upload.
	Compression configcompression.Type
	// UniqueKeyFunc allows for overwriting the default behavior of
	// generating a new unique string to avoid collisions on file upload
	// across many different instances.
	UniqueKeyFunc func() string
	// IsCompressed when true keeps files compressed in S3
	// by omitting ContentEncoding headers. When false, ContentEncoding
	// is set for HTTP transfer compression (AWS auto-decompresses).
	IsCompressed bool
}

func (pki *PartitionKeyBuilder) Build(ts time.Time, overridePrefix string) string {
	if pki.LegacyS3KeyTemplate != "" {
		tmpl := pki.LegacyS3KeyTemplateParsed
		if tmpl == nil {
			var err error
			tmpl, err = parseLegacyTemplate(pki.LegacyS3KeyTemplate)
			if err != nil {
				slog.Error("failed to parse legacy s3 key template", "error", err)
				return ""
			}
		}
		return buildLegacyTemplateKey(pki.legacyTemplatePrefix(overridePrefix), tmpl, ts)
	}

	return path.Join(pki.bucketKeyPrefix(ts, overridePrefix), pki.fileName())
}

type legacyTemplateData struct {
	Prefix string
	Date   string
	UUID   string
}

func buildLegacyTemplateKey(prefix string, tmpl *template.Template, ts time.Time) string {
	var rendered bytes.Buffer
	err := tmpl.Execute(&rendered, legacyTemplateData{
		Prefix: prefix,
		Date:   ts.UTC().Format("2006/01/02"),
		UUID:   uuid.NewString(),
	})
	if err != nil {
		slog.Error("failed to execute legacy s3 key template", "error", err)
		return ""
	}

	key := rendered.String()

	// SAW-7554: legacy-template mode must honor s3_prefix as an invariant,
	// symmetric with default mode (which always joins the prefix via
	// bucketKeyPrefix). When the template author forgot to reference
	// {{.Prefix}}, the configured prefix would otherwise be silently
	// dropped. Auto-prepend it; templates that already reference .Prefix
	// stay unchanged (no double-prepend).
	if prefix != "" && !templateReferencesPrefix(tmpl) {
		return path.Join(prefix, key)
	}

	return key
}

func parseLegacyTemplate(templateText string) (*template.Template, error) {
	return template.New("legacy-s3-key").Funcs(legacyTemplateFuncs).Option("missingkey=error").Parse(templateText)
}

// templateReferencesPrefix reports whether tmpl reads the .Prefix field of
// legacyTemplateData anywhere in its body. Used by buildLegacyTemplateKey
// to decide whether to auto-prepend the configured prefix (SAW-7554).
func templateReferencesPrefix(tmpl *template.Template) bool {
	if tmpl == nil || tmpl.Tree == nil {
		return false
	}
	return walkForPrefix(tmpl.Tree.Root)
}

func walkForPrefix(n parse.Node) bool {
	switch node := n.(type) {
	case nil:
		return false
	case *parse.ListNode:
		if node == nil {
			return false
		}
		for _, child := range node.Nodes {
			if walkForPrefix(child) {
				return true
			}
		}
	case *parse.ActionNode:
		return walkForPrefix(node.Pipe)
	case *parse.IfNode:
		return walkForPrefix(node.Pipe) ||
			walkForPrefix(node.List) ||
			walkForPrefix(node.ElseList)
	case *parse.RangeNode:
		return walkForPrefix(node.Pipe) ||
			walkForPrefix(node.List) ||
			walkForPrefix(node.ElseList)
	case *parse.WithNode:
		return walkForPrefix(node.Pipe) ||
			walkForPrefix(node.List) ||
			walkForPrefix(node.ElseList)
	case *parse.PipeNode:
		if node == nil {
			return false
		}
		for _, cmd := range node.Cmds {
			if cmd == nil {
				continue
			}
			for _, arg := range cmd.Args {
				if walkForPrefix(arg) {
					return true
				}
			}
		}
	case *parse.FieldNode:
		// `{{.Prefix}}` parses as FieldNode with Ident=["Prefix"].
		for _, ident := range node.Ident {
			if ident == "Prefix" {
				return true
			}
		}
	case *parse.ChainNode:
		// e.g. `{{$x.Prefix}}` would land here, though our schema doesn't
		// use it. Cover defensively.
		for _, ident := range node.Field {
			if ident == "Prefix" {
				return true
			}
		}
	}
	return false
}

func ParseLegacyTemplateForValidation(templateText string) (*template.Template, error) {
	return parseLegacyTemplate(templateText)
}

func ValidateLegacyTemplateForValidation(tmpl *template.Template) error {
	return tmpl.Execute(io.Discard, legacyTemplateData{
		Prefix: "prefix",
		Date:   "2006/01/02",
		UUID:   uuid.NewString(),
	})
}

func (pki *PartitionKeyBuilder) legacyTemplatePrefix(overridePrefix string) string {
	prefix := pki.PartitionPrefix
	if overridePrefix != "" {
		prefix = overridePrefix
	}

	var pathParts []string

	if pki.PartitionBasePrefix != "" {
		pathParts = append(pathParts, pki.PartitionBasePrefix)
	}

	if prefix != "" {
		pathParts = append(pathParts, prefix)
	}

	return strings.Join(pathParts, "/")
}

func (pki *PartitionKeyBuilder) bucketKeyPrefix(ts time.Time, overridePrefix string) string {
	// Don't want to overwrite the actual value
	prefix := pki.legacyTemplatePrefix(overridePrefix)
	var pathParts []string

	if prefix != "" {
		pathParts = append(pathParts, prefix)
	}

	if pki.Metadata != "" {
		pathParts = append(pathParts, pki.Metadata)
	}

	location := pki.PartitionTimeLocation
	if location == nil {
		location = time.Local
	}
	pathParts = append(pathParts, timefmt.Format(ts.In(location), pki.PartitionFormat))

	return strings.Join(pathParts, "/")
}

func (pki *PartitionKeyBuilder) fileName() string {
	var suffix string

	if pki.FileFormat != "" {
		suffix = "." + pki.FileFormat
	}

	if ext, ok := compressionFileExtensions[pki.Compression]; ok {
		suffix += ext
	}

	return pki.FilePrefix + pki.Metadata + "_" + pki.uniqueKey() + suffix
}

func (pki *PartitionKeyBuilder) uniqueKey() string {
	// If a custom function is provided, use it to generate the unique key.
	// If it fails, fall back to the default random integer generation
	// so that uploads are not blocked.
	if pki.UniqueKeyFunc != nil {
		if k := pki.UniqueKeyFunc(); k != "" {
			return k
		}
	}

	return pki.randInt()
}

func GenerateUUIDv7() string {
	id, err := uuid.NewV7()
	if err != nil {
		return ""
	}
	return id.String()
}

func (*PartitionKeyBuilder) randInt() string {
	// This follows the original "uniqueness" algorithm
	// to avoid collisions on file uploads across different nodes.
	const (
		uniqueValues = 999999999
		minOffset    = 100000000
	)

	return strconv.Itoa(minOffset + rand.IntN(uniqueValues-minOffset))
}
