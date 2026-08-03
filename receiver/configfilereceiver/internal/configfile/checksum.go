package configfile

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// Checksum returns a stable SHA-256 hex digest of sorted key=value lines.
func Checksum(keys map[string]string) string {
	if len(keys) == 0 {
		return sha256Hex("")
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	var b strings.Builder
	for _, k := range sorted {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(keys[k])
		b.WriteByte('\n')
	}
	return sha256Hex(b.String())
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
