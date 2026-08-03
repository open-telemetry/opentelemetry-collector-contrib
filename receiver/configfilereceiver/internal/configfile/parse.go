package configfile

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultMaxKeysPerFile = 500
	MaxConfigFileBytes    = 1 << 20 // 1 MiB per-file read cap
)

var DefaultExcludeKeyGlobs = []string{
	"*password*",
	"*secret*",
	"*token*",
	"*_key",
	"*credential*",
}

// Pair is a flattened config key and value.
type Pair struct {
	Key   string
	Value any
}

// Options controls parsing and filtering.
type Options struct {
	ExcludeKeys    []string
	MaxKeysPerFile int
}

func (o Options) maxKeys() int {
	if o.MaxKeysPerFile <= 0 {
		return DefaultMaxKeysPerFile
	}
	return o.MaxKeysPerFile
}

func (o Options) excludeGlobs() []string {
	if len(o.ExcludeKeys) == 0 {
		return DefaultExcludeKeyGlobs
	}
	return o.ExcludeKeys
}

// ReadFile reads up to MaxConfigFileBytes from path.
func ReadFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	limited := io.LimitReader(f, MaxConfigFileBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(data) > MaxConfigFileBytes {
		return data[:MaxConfigFileBytes], nil
	}
	return data, nil
}

// ResolveFormat maps auto or explicit format to a parser name.
func ResolveFormat(path, format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "generic", "ini", "yaml", "json":
		return format
	case "auto", "":
		switch strings.ToLower(filepath.Ext(path)) {
		case ".yaml", ".yml":
			return "yaml"
		case ".json":
			return "json"
		case ".ini", ".conf":
			if strings.HasSuffix(strings.ToLower(path), ".ini") {
				return "ini"
			}
			return "generic"
		default:
			return "generic"
		}
	default:
		return "generic"
	}
}

// ParseFile reads and parses a config file into filtered string key/value pairs.
func ParseFile(path, format string, opts Options) (resolvedFormat string, pairs map[string]string, err error) {
	resolvedFormat = ResolveFormat(path, format)
	data, err := ReadFile(path)
	if err != nil {
		return resolvedFormat, nil, err
	}
	raw, err := parseConfigFile(resolvedFormat, data)
	if err != nil {
		return resolvedFormat, nil, err
	}
	return resolvedFormat, filterPairs(raw, opts), nil
}

func filterPairs(raw []Pair, opts Options) map[string]string {
	exclude := opts.excludeGlobs()
	maxKeys := opts.maxKeys()
	seen := make(map[string]struct{})
	out := make(map[string]string)

	for _, pair := range raw {
		if pair.Key == "" {
			continue
		}
		if matchAnyGlob(pair.Key, exclude) {
			continue
		}
		if _, exists := seen[pair.Key]; exists {
			continue
		}
		seen[pair.Key] = struct{}{}
		if len(out) >= maxKeys {
			break
		}
		out[pair.Key] = StringifyValue(pair.Value)
	}
	return out
}

func parseConfigFile(format string, data []byte) ([]Pair, error) {
	switch format {
	case "yaml":
		return parseYAML(data)
	case "json":
		return parseJSON(data)
	case "ini":
		return parseINI(data), nil
	default:
		return parseGeneric(data), nil
	}
}

func parseYAML(data []byte) ([]Pair, error) {
	var root any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	var out []Pair
	flattenValue("", root, &out)
	return out, nil
}

func parseJSON(data []byte) ([]Pair, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	var out []Pair
	flattenValue("", root, &out)
	return out, nil
}

func flattenValue(prefix string, v any, out *[]Pair) {
	switch typed := v.(type) {
	case map[string]any:
		for k, child := range typed {
			pathKey := k
			if prefix != "" {
				pathKey = prefix + "." + k
			}
			flattenValue(pathKey, child, out)
		}
	case map[any]any:
		for k, child := range typed {
			key := fmt.Sprint(k)
			pathKey := key
			if prefix != "" {
				pathKey = prefix + "." + key
			}
			flattenValue(pathKey, child, out)
		}
	case []any:
		for i, child := range typed {
			pathKey := fmt.Sprintf("%s[%d]", prefix, i)
			if prefix == "" {
				pathKey = fmt.Sprintf("[%d]", i)
			}
			flattenValue(pathKey, child, out)
		}
	default:
		if prefix == "" {
			return
		}
		*out = append(*out, Pair{Key: prefix, Value: typed})
	}
}

func parseGeneric(data []byte) []Pair {
	var out []Pair
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if idx := strings.IndexAny(line, "#;"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
			if line == "" {
				continue
			}
		}

		key, value, ok := splitKeyValue(line)
		if !ok {
			continue
		}
		out = append(out, Pair{Key: key, Value: value})
	}
	return out
}

func parseINI(data []byte) []Pair {
	var out []Pair
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	section := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if idx := strings.IndexAny(line, "#;"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
			if line == "" {
				continue
			}
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}

		key, value, ok := splitKeyValue(line)
		if !ok {
			continue
		}
		if section != "" {
			key = section + "." + key
		}
		out = append(out, Pair{Key: key, Value: value})
	}
	return out
}

func splitKeyValue(line string) (string, string, bool) {
	if strings.Contains(line, "=") {
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"'")
		if key == "" {
			return "", "", false
		}
		return key, value, true
	}

	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", "", false
	}
	key := fields[0]
	value := strings.Join(fields[1:], " ")
	value = strings.Trim(value, "\"'")
	return key, value, true
}

// StringifyValue canonicalizes parsed values for checksum and log attributes.
func StringifyValue(v any) string {
	if v == nil {
		return ""
	}
	switch typed := v.(type) {
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'g', -1, 64)
	case float32:
		f := float64(typed)
		if f == float64(int64(f)) {
			return strconv.FormatInt(int64(f), 10)
		}
		return strconv.FormatFloat(f, 'g', -1, 32)
	default:
		return fmt.Sprint(typed)
	}
}

func matchAnyGlob(key string, globs []string) bool {
	lowerKey := strings.ToLower(key)
	for _, pattern := range globs {
		ok, err := path.Match(strings.ToLower(pattern), lowerKey)
		if err == nil && ok {
			return true
		}
		base := lowerKey
		if i := strings.LastIndex(lowerKey, "."); i >= 0 {
			base = lowerKey[i+1:]
		}
		ok, err = path.Match(strings.ToLower(pattern), base)
		if err == nil && ok {
			return true
		}
	}
	return false
}
