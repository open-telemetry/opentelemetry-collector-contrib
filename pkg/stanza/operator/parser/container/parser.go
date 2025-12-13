// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package container // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/container"

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/timeutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const (
	dockerFormat        = "docker"
	crioFormat          = "crio"
	containerdFormat    = "containerd"
	recombineInternalID = "recombine_container_internal"
	dockerPattern       = "^\\{"
	crioPattern         = "^(?P<time>[^ Z]+) (?P<stream>stdout|stderr) (?P<logtag>[^ ]*) ?(?P<log>.*)$"
	containerdPattern   = "^(?P<time>[^ ^Z]+Z) (?P<stream>stdout|stderr) (?P<logtag>[^ ]*) ?(?P<log>.*)$"
	logpathPattern      = "^.*(\\/|\\\\)(?P<namespace>[^_]+)_(?P<pod_name>[^_]+)_(?P<uid>[a-f0-9\\-]+)(\\/|\\\\)(?P<container_name>[^\\._]+)(\\/|\\\\)(?P<restart_count>\\d+)\\.log(\\.\\d{8}-\\d{6})?$"
	logPathField        = attrs.LogFilePath
	crioTimeLayout      = "2006-01-02T15:04:05.999999999Z07:00"
	goTimeLayout        = "2006-01-02T15:04:05.999Z"
)

var (
	dockerMatcher     = regexp.MustCompile(dockerPattern)
	crioMatcher       = regexp.MustCompile(crioPattern)
	containerdMatcher = regexp.MustCompile(containerdPattern)
	pathMatcher       = regexp.MustCompile(logpathPattern)
)

var (
	logFieldsMapping = map[string]string{
		"stream": "log.iostream",
	}
	k8sMetadataMapping = map[string]string{
		"container_name": "k8s.container.name",
		"namespace":      "k8s.namespace.name",
		"pod_name":       "k8s.pod.name",
		"restart_count":  "k8s.container.restart_count",
		"uid":            "k8s.pod.uid",
	}
	// mapPool reuses maps to reduce allocations for parsing
	mapPool = sync.Pool{
		New: func() any {
			return make(map[string]any, 4)
		},
	}
	// pathMapPool reuses maps for path parsing
	pathMapPool = sync.Pool{
		New: func() any {
			return make(map[string]any, 5)
		},
	}
)

// Parser is an operator that parses Container logs.
type Parser struct {
	helper.ParserOperator
	recombineParser         operator.Operator
	format                  string
	addMetadataFromFilepath bool
	criLogEmitter           *helper.BatchingLogEmitter
	asyncConsumerStarted    bool
	criConsumerStartOnce    sync.Once
	criConsumers            *sync.WaitGroup
	timeLayout              string
	pathCache               helper.Cache
}

func (p *Parser) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return p.TransformerOperator.ProcessBatchWith(ctx, entries, p.Process)
}

// Process will parse an entry of Container logs
func (p *Parser) Process(ctx context.Context, entry *entry.Entry) (err error) {
	format := p.format
	if format == "" {
		format, err = p.detectFormat(entry)
		if err != nil {
			return fmt.Errorf("failed to detect a valid container log format: %w", err)
		}
	}

	switch format {
	case dockerFormat:
		p.timeLayout = goTimeLayout
		err = p.ProcessWithCallback(ctx, entry, p.parseDocker, p.handleTimeAndAttributeMappings)
		if err != nil {
			return fmt.Errorf("failed to process the docker log: %w", err)
		}
	case containerdFormat, crioFormat:
		p.criConsumerStartOnce.Do(func() {
			err = p.criLogEmitter.Start(nil)
			if err != nil {
				p.Logger().Error("unable to start the internal LogEmitter", zap.Error(err))
				return
			}
			err = p.recombineParser.Start(nil)
			if err != nil {
				p.Logger().Error("unable to start the internal recombine operator", zap.Error(err))
				return
			}
			p.asyncConsumerStarted = true
		})

		// Short circuit if the "if" condition does not match
		skip, err := p.Skip(ctx, entry)
		if err != nil {
			return p.HandleEntryError(ctx, entry, err)
		}
		if skip {
			return p.Write(ctx, entry)
		}

		if format == containerdFormat {
			// parse the message
			err = p.ParseWith(ctx, entry, p.parseContainerd, p.Write)
			if err != nil {
				return fmt.Errorf("failed to parse containerd log: %w", err)
			}
			p.timeLayout = goTimeLayout
		} else {
			// parse the message
			err = p.ParseWith(ctx, entry, p.parseCRIO, p.Write)
			if err != nil {
				return fmt.Errorf("failed to parse crio log: %w", err)
			}
			p.timeLayout = crioTimeLayout
		}

		err = p.handleTimeAndAttributeMappings(entry)
		if err != nil {
			return fmt.Errorf("failed to handle attribute mappings: %w", err)
		}

		// send it to the recombine operator
		err = p.recombineParser.Process(ctx, entry)
		if err != nil {
			return fmt.Errorf("failed to recombine the crio log: %w", err)
		}
	default:
		return errors.New("failed to detect a valid container log format")
	}

	return nil
}

// Stop ensures that the internal recombineParser, the internal criLogEmitter and
// the crioConsumer are stopped in the proper order without being affected by
// any possible race conditions
func (p *Parser) Stop() error {
	if p.pathCache != nil {
		p.pathCache.Stop()
	}
	if !p.asyncConsumerStarted {
		// nothing is started return
		return nil
	}
	var errs error
	if err := p.recombineParser.Stop(); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("unable to stop the internal recombine operator: %w", err))
	}
	// the recombineParser will call the Process of the criLogEmitter synchronously so the entries will be first
	// written to the channel before the Stop of the recombineParser returns. Then since the criLogEmitter handles
	// the entries synchronously it is safe to call its Stop.
	// After criLogEmitter is stopped the crioConsumer will consume the remaining messages and return.
	if err := p.criLogEmitter.Stop(); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("unable to stop the internal LogEmitter: %w", err))
	}
	p.criConsumers.Wait()
	return errs
}

// detectFormat will detect the container log format
func (p *Parser) detectFormat(e *entry.Entry) (string, error) {
	value, ok := e.Get(p.ParseFrom)
	if !ok {
		return "", errors.New("entry cannot be parsed as container logs")
	}

	raw, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("type '%T' cannot be parsed as container logs", value)
	}

	// Try fast-path format detection first (no regex, no allocations)
	if detectContainerdFormat(raw) {
		return containerdFormat, nil
	}
	if detectCRIOFormat(raw) {
		return crioFormat, nil
	}

	// Fallback to regex for format detection
	switch {
	case dockerMatcher.MatchString(raw):
		return dockerFormat, nil
	case crioMatcher.MatchString(raw):
		return crioFormat, nil
	case containerdMatcher.MatchString(raw):
		return containerdFormat, nil
	}
	return "", fmt.Errorf("entry cannot be parsed as container logs: %v", value)
}

// parseCRIO will parse a crio log value based on a fixed regexp
func (*Parser) parseCRIO(value any) (any, error) {
	raw, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("type '%T' cannot be parsed as cri-o container logs", value)
	}

	// Try fast-path parsing first (no regex)
	if parsed, ok := parseCRIOFields(raw); ok {
		return parsed, nil
	}

	// Fallback to regex
	return helper.MatchValues(raw, crioMatcher)
}

// parseContainerd will parse a containerd log value based on a fixed regexp
func (*Parser) parseContainerd(value any) (any, error) {
	raw, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("type '%T' cannot be parsed as containerd logs", value)
	}

	// Try fast-path parsing first (no regex)
	if parsed, ok := parseContainerdFields(raw); ok {
		return parsed, nil
	}

	// Fallback to regex
	return helper.MatchValues(raw, containerdMatcher)
}

// parseDocker will parse a docker log value as JSON
func (*Parser) parseDocker(value any) (any, error) {
	raw, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("type '%T' cannot be parsed as docker container logs", value)
	}

	parsedValue := make(map[string]any)
	err := json.Unmarshal([]byte(raw), &parsedValue)
	if err != nil {
		return nil, err
	}
	return parsedValue, nil
}

// handleTimeAndAttributeMappings handles fields' mappings and k8s meta extraction
func (p *Parser) handleTimeAndAttributeMappings(e *entry.Entry) error {
	err := parseTime(e, p.timeLayout)
	if err != nil {
		return fmt.Errorf("failed to parse time: %w", err)
	}

	err = p.handleMoveAttributes(e)
	if err != nil {
		return err
	}
	err = p.extractk8sMetaFromFilePath(e)
	if err != nil {
		return err
	}

	return nil
}

// handleMoveAttributes moves fields to final attributes
func (*Parser) handleMoveAttributes(e *entry.Entry) error {
	// move `log` to `body` explicitly first to avoid
	// moving after more attributes have been added under the `log.*` key
	err := moveFieldToBody(e, "log", "body")
	if err != nil {
		return err
	}
	// then move the rest of the fields
	for originalKey, mappedKey := range logFieldsMapping {
		err = moveField(e, originalKey, mappedKey)
		if err != nil {
			return err
		}
	}
	return nil
}

// extractk8sMetaFromFilePath extracts metadata attributes from logfilePath
func (p *Parser) extractk8sMetaFromFilePath(e *entry.Entry) error {
	if !p.addMetadataFromFilepath {
		return nil
	}

	logPath, ok := e.Attributes[logPathField]
	if !ok {
		return fmt.Errorf(
			"operator '%s' has 'add_metadata_from_filepath' enabled, but the log record attribute '%s' is missing. Perhaps enable the 'include_file_path' option?",
			p.OperatorID,
			logPathField)
	}

	rawLogPath, ok := logPath.(string)
	if !ok {
		return fmt.Errorf("type '%T' cannot be parsed as log path field", logPath)
	}

	if p.pathCache != nil {
		if cached := p.pathCache.Get(rawLogPath); cached != nil {
			if parsedValues, ok := cached.(map[string]any); ok {
				return p.setK8sMetadataFromParsedValues(e, parsedValues)
			}
		}
	}

	// Try fast-path parsing first (no regex)
	if parsedValues, ok := parseLogPath(rawLogPath); ok {
		// For cached values, we need a new map (can't reuse pooled map)
		// Copy to a new map for caching
		cachedMap := make(map[string]any, len(parsedValues))
		for k, v := range parsedValues {
			cachedMap[k] = v
		}
		if p.pathCache != nil {
			p.pathCache.Add(rawLogPath, cachedMap)
		}
		err := p.setK8sMetadataFromParsedValues(e, parsedValues)
		// Return map to pool after use (not cached)
		pathMapPool.Put(parsedValues)
		return err
	}

	// Fallback to regex
	parsedValues, err := helper.MatchValues(rawLogPath, pathMatcher)
	if err != nil {
		return errors.New("failed to detect a valid log path")
	}

	if p.pathCache != nil {
		p.pathCache.Add(rawLogPath, parsedValues)
	}

	return p.setK8sMetadataFromParsedValues(e, parsedValues)
}

func (p *Parser) consumeEntries(ctx context.Context, entries []*entry.Entry) {
	for _, e := range entries {
		err := p.Write(ctx, e)
		if err != nil {
			p.Logger().Error("failed to write entry", zap.Error(err))
		}
	}
}

func moveField(e *entry.Entry, originalKey, mappedKey string) error {
	val, exist := entry.NewAttributeField(originalKey).Delete(e)
	if !exist {
		return fmt.Errorf("move: field %v does not exist", originalKey)
	}
	atKey := entry.NewAttributeField(mappedKey)
	if err := atKey.Set(e, val); err != nil {
		return fmt.Errorf("failed to move %v to %v", originalKey, mappedKey)
	}
	return nil
}

func moveFieldToBody(e *entry.Entry, originalKey, mappedKey string) error {
	val, exist := entry.NewAttributeField(originalKey).Delete(e)
	if !exist {
		return fmt.Errorf("move: field %v does not exist", originalKey)
	}
	body, _ := entry.NewField(mappedKey)
	if err := body.Set(e, val); err != nil {
		return fmt.Errorf("failed to move %v to %v", originalKey, mappedKey)
	}
	return nil
}

func parseTime(e *entry.Entry, layout string) error {
	var location *time.Location
	parseFrom := "time"
	value, ok := e.Get(entry.NewAttributeField(parseFrom))
	if !ok {
		return fmt.Errorf("failed to get the time from %v", e)
	}

	if strings.HasSuffix(layout, "Z") {
		// If a timestamp ends with 'Z', it should be interpreted at Zulu (UTC) time
		location = time.UTC
	} else {
		location = time.Local
	}

	timeValue, err := timeutils.ParseGotime(layout, value, location)
	if err != nil {
		return err
	}
	// timeutils.ParseGotime calls timeutils.SetTimestampYear before returning the timeValue
	e.Timestamp = timeValue

	e.Delete(entry.NewAttributeField(parseFrom))

	return nil
}

func (p *Parser) setK8sMetadataFromParsedValues(e *entry.Entry, parsedValues map[string]any) error {
	for originalKey, attributeKey := range k8sMetadataMapping {
		newField := entry.NewResourceField(attributeKey)
		if err := newField.Set(e, parsedValues[originalKey]); err != nil {
			return fmt.Errorf("failed to set %v as metadata at %v", originalKey, attributeKey)
		}
	}
	return nil
}

// parseLogPath parses a Kubernetes log path without regex. Returns ok=false on failure.
// Format: .../namespace_pod-name_uid/container-name/restart-count.log[.rotation-suffix]
// Supports both Unix (/ ) and Windows (\ ) path separators.
func parseLogPath(path string) (map[string]any, bool) {
	// Helper function to find last separator (either / or \)
	findLastSep := func(s string) int {
		for i := len(s) - 1; i >= 0; i-- {
			if s[i] == '/' || s[i] == '\\' {
				return i
			}
		}
		return -1
	}

	lastSep := findLastSep(path)
	if lastSep == -1 {
		return nil, false
	}

	// Extract restart count and verify .log extension
	filename := path[lastSep+1:]

	// Check for rotated log: .log.YYYYMMDD-HHMMSS (or any .log.* suffix like .log.gz)
	// This handles both timestamped rotations and compressed logs
	if idx := strings.LastIndex(filename, ".log."); idx != -1 {
		// Rotated/compressed log, extract just the .log part
		filename = filename[:idx+4] // Keep up to and including ".log"
	} else if !strings.HasSuffix(filename, ".log") {
		return nil, false
	}

	// Remove .log extension and validate restart count is numeric
	restartCountStr := strings.TrimSuffix(filename, ".log")
	if _, err := strconv.Atoi(restartCountStr); err != nil {
		return nil, false
	}

	// Find container name (before restart count)
	pathBeforeFile := path[:lastSep]
	containerSep := findLastSep(pathBeforeFile)
	if containerSep == -1 {
		return nil, false
	}
	containerName := pathBeforeFile[containerSep+1:]

	// Find pod part (namespace_pod-name_uid)
	pathBeforeContainer := pathBeforeFile[:containerSep]
	podSep := findLastSep(pathBeforeContainer)
	if podSep == -1 {
		return nil, false
	}
	podPart := pathBeforeContainer[podSep+1:]

	// Parse pod part: namespace_pod-name_uid
	// Must have at least 2 underscores
	underscore1 := strings.IndexByte(podPart, '_')
	if underscore1 == -1 {
		return nil, false
	}
	namespace := podPart[:underscore1]

	underscore2 := strings.IndexByte(podPart[underscore1+1:], '_')
	if underscore2 == -1 {
		return nil, false
	}
	underscore2 += underscore1 + 1
	podName := podPart[underscore1+1 : underscore2]
	uid := podPart[underscore2+1:]

	// Use pool for path maps (but note: cached maps need to be new allocations)
	m := pathMapPool.Get().(map[string]any)
	// Clear the map in case it was reused
	for k := range m {
		delete(m, k)
	}
	m["namespace"] = namespace
	m["pod_name"] = podName
	m["uid"] = uid
	m["container_name"] = containerName
	m["restart_count"] = restartCountStr
	return m, true
}

// detectCRIOFormat detects CRI-O format without allocating. Returns true if format matches.
func detectCRIOFormat(raw string) bool {
	_, rest, ok := splitOnce(raw, ' ')
	if !ok {
		return false
	}

	stream, rest, ok := splitOnce(rest, ' ')
	if !ok || (stream != "stdout" && stream != "stderr") {
		return false
	}

	// Just check if we can parse the structure, don't need full parsing
	_, _, _ = splitOnce(rest, ' ')
	return true // If we got here, it's valid CRI-O format
}

// parseCRIOFields parses a CRI-O log line without regex. Returns ok=false on failure.
func parseCRIOFields(raw string) (map[string]any, bool) {
	timePart, rest, ok := splitOnce(raw, ' ')
	if !ok {
		return nil, false
	}

	stream, rest, ok := splitOnce(rest, ' ')
	if !ok || (stream != "stdout" && stream != "stderr") {
		return nil, false
	}

	logtag, logPart, ok := splitOnce(rest, ' ')
	if !ok {
		// No log body, treat logtag as rest, log empty.
		logtag = rest
		logPart = ""
	}

	if len(logPart) > 0 && logPart[0] == ' ' {
		logPart = logPart[1:]
	}

	// Use pool to reduce allocations
	m := mapPool.Get().(map[string]any)
	// Clear the map in case it was reused
	for k := range m {
		delete(m, k)
	}
	m["time"] = timePart
	m["stream"] = stream
	m["logtag"] = logtag
	m["log"] = logPart
	return m, true
}

// detectContainerdFormat detects containerd format without allocating. Returns true if format matches.
func detectContainerdFormat(raw string) bool {
	timePart, rest, ok := splitOnce(raw, ' ')
	if !ok || !strings.HasSuffix(timePart, "Z") {
		return false
	}

	stream, rest, ok := splitOnce(rest, ' ')
	if !ok || (stream != "stdout" && stream != "stderr") {
		return false
	}

	// Just check if we can parse the structure, don't need full parsing
	_, _, _ = splitOnce(rest, ' ')
	return true // If we got here, it's valid containerd format
}

// parseContainerdFields parses a containerd log line without regex. Returns ok=false on failure.
func parseContainerdFields(raw string) (map[string]any, bool) {
	timePart, rest, ok := splitOnce(raw, ' ')
	if !ok || !strings.HasSuffix(timePart, "Z") {
		return nil, false
	}

	stream, rest, ok := splitOnce(rest, ' ')
	if !ok || (stream != "stdout" && stream != "stderr") {
		return nil, false
	}

	logtag, logPart, ok := splitOnce(rest, ' ')
	if !ok {
		logtag = rest
		logPart = ""
	}

	if len(logPart) > 0 && logPart[0] == ' ' {
		logPart = logPart[1:]
	}

	// Use pool to reduce allocations
	m := mapPool.Get().(map[string]any)
	// Clear the map in case it was reused
	for k := range m {
		delete(m, k)
	}
	m["time"] = timePart
	m["stream"] = stream
	m["logtag"] = logtag
	m["log"] = logPart
	return m, true
}

// splitOnce splits s on the first occurrence of sep.
func splitOnce(s string, sep byte) (head string, tail string, ok bool) {
	idx := strings.IndexByte(s, sep)
	if idx == -1 {
		return "", "", false
	}
	return s[:idx], s[idx+1:], true
}
