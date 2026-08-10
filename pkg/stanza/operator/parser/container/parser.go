// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package container // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/container"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-json"
	lru "github.com/hashicorp/golang-lru/v2"
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
	logPathField        = attrs.LogFilePath
	crioTimeLayout      = "2006-01-02T15:04:05.999999999Z07:00"
	goTimeLayout        = "2006-01-02T15:04:05.999Z"
)

// Parser is an operator that parses Container logs.
type Parser struct {
	helper.ParserOperator
	recombineParser         operator.Operator
	format                  string
	addMetadataFromFilepath bool
	criLogEmitter           helper.LogEmitter
	recombineStarted        bool
	recombineStartOnce      sync.Once
	timeLayout              string
	cache                   *lru.Cache[string, map[string]any]
}

var (
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

func (p *Parser) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	processedEntries := make([]*entry.Entry, 0, len(entries))
	write := func(_ context.Context, ent *entry.Entry) error {
		processedEntries = append(processedEntries, ent)
		return nil
	}
	var errs []error
	var criEntries []*entry.Entry

	for _, ent := range entries {
		skip, err := p.Skip(ctx, ent)
		if err != nil {
			errs = append(errs, p.HandleEntryErrorWithWrite(ctx, ent, err, write))
			continue
		}
		if skip {
			_ = write(ctx, ent)
			continue
		}

		fields, format, err := p.parseEntry(ent)
		if err != nil {
			errs = append(errs, p.HandleEntryErrorWithWrite(ctx, ent, fmt.Errorf("failed to parse entry: %w", err), write))
			continue
		}

		switch format {
		case dockerFormat:
			p.timeLayout = goTimeLayout
			if err = p.ParseWith(ctx, ent, p.parseDocker, write); err != nil {
				if !errors.Is(err, helper.ErrEntryHandled) {
					errs = append(errs, fmt.Errorf("failed to process the docker log: %w", err))
				}
				continue
			}
			if err = p.handleTimeAndAttributeMappings(ent); err != nil {
				errs = append(errs, p.HandleEntryErrorWithWrite(ctx, ent, err, write))
				continue
			}
			_ = write(ctx, ent)

		case containerdFormat, crioFormat:
			p.recombineStartOnce.Do(func() {
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
				p.recombineStarted = true
			})

			capturedFields := fields
			if err = p.ParseWith(ctx, ent, func(_ any) (any, error) { return capturedFields, nil }, write); err != nil {
				if !errors.Is(err, helper.ErrEntryHandled) {
					errs = append(errs, fmt.Errorf("failed to parse cri log: %w", err))
				}
				continue
			}
			if format == containerdFormat {
				p.timeLayout = goTimeLayout
			} else {
				p.timeLayout = crioTimeLayout
			}

			if err = p.handleTimeAndAttributeMappings(ent); err != nil {
				errs = append(errs, p.HandleEntryErrorWithWrite(ctx, ent, err, write))
				continue
			}
			criEntries = append(criEntries, ent)

		default:
			errs = append(errs, p.HandleEntryErrorWithWrite(ctx, ent, errors.New("failed to detect a valid container log format"), write))
		}
	}

	// Send CRI entries as a batch to recombine
	if len(criEntries) > 0 {
		if err := p.recombineParser.ProcessBatch(ctx, criEntries); err != nil {
			errs = append(errs, fmt.Errorf("failed to recombine cri logs: %w", err))
		}
	}

	// Write all docker/skipped entries as a batch
	if len(processedEntries) > 0 {
		errs = append(errs, p.WriteBatch(ctx, processedEntries))
	}

	return errors.Join(errs...)
}

// Process will parse an entry of Container logs
func (p *Parser) Process(ctx context.Context, entry *entry.Entry) (err error) {
	// Short circuit if the "if" condition does not match
	skip, err := p.Skip(ctx, entry)
	if err != nil {
		return p.HandleEntryError(ctx, entry, err)
	}
	if skip {
		return p.Write(ctx, entry)
	}

	fields, format, err := p.parseEntry(entry)
	if err != nil {
		return p.HandleEntryError(ctx, entry, fmt.Errorf("failed to parse entry: %w", err))
	}

	switch format {
	case dockerFormat:
		p.timeLayout = goTimeLayout
		err = p.ProcessWithCallback(ctx, entry, p.parseDocker, p.handleTimeAndAttributeMappings)
		if err != nil {
			return fmt.Errorf("failed to process the docker log: %w", err)
		}
	case containerdFormat, crioFormat:
		p.recombineStartOnce.Do(func() {
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
			p.recombineStarted = true
		})

		capturedFields := fields
		err = p.ParseWith(ctx, entry, func(_ any) (any, error) { return capturedFields, nil }, p.Write)
		if err != nil {
			if errors.Is(err, helper.ErrEntryHandled) {
				return nil
			}
			return fmt.Errorf("failed to parse cri log: %w", err)
		}
		if format == containerdFormat {
			p.timeLayout = goTimeLayout
		} else {
			p.timeLayout = crioTimeLayout
		}

		err = p.handleTimeAndAttributeMappings(entry)
		if err != nil {
			err = fmt.Errorf("failed to handle attribute mappings: %w", err)

			switch p.OnError {
			case helper.DropOnErrorQuiet:
				return nil
			case helper.SendOnErrorQuiet:
				if writeErr := p.Write(ctx, entry); writeErr != nil {
					return fmt.Errorf("failed to send entry after error: %w", writeErr)
				}
				return nil
			case helper.SendOnError:
				if writeErr := p.Write(ctx, entry); writeErr != nil {
					return fmt.Errorf("failed to send entry after error: %w", writeErr)
				}
				return err
			default:
				return err
			}
		}

		// send it to the recombine operator
		err = p.recombineParser.Process(ctx, entry)
		if err != nil {
			return p.HandleEntryError(ctx, entry, fmt.Errorf("failed to recombine the crio log: %w", err))
		}
	default:
		return p.HandleEntryError(ctx, entry, errors.New("failed to detect a valid container log format"))
	}

	return nil
}

// Stop ensures that the internal recombineParser and criLogEmitter are stopped
// in the proper order without being affected by any possible race conditions.
func (p *Parser) Stop() error {
	if !p.recombineStarted {
		// nothing is started return
		return nil
	}
	var errs error
	if err := p.recombineParser.Stop(); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("unable to stop the internal recombine operator: %w", err))
	}
	if err := p.criLogEmitter.Stop(); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("unable to stop the internal LogEmitter: %w", err))
	}
	return errs
}

// parseEntry reads the raw value from the entry and detects its format via splitCRI.
// If p.format is pinned in config, the detected format must match — a mismatch is an error.
func (p *Parser) parseEntry(e *entry.Entry) (map[string]any, string, error) {
	value, ok := e.Get(p.ParseFrom)
	if !ok {
		return nil, "", errors.New("entry cannot be parsed as container logs")
	}

	raw, ok := value.(string)
	if !ok {
		return nil, "", fmt.Errorf("type '%T' cannot be parsed as container logs", value)
	}

	var detected string
	var fields map[string]any

	if raw != "" && raw[0] == '{' {
		detected = dockerFormat
	} else {
		var containerd bool
		fields, containerd, ok = parseCRI(raw)
		if !ok {
			return nil, "", errors.New("entry cannot be split to CRI fields")
		}
		if containerd {
			detected = containerdFormat
		} else {
			detected = crioFormat
		}
	}

	if p.format != "" && p.format != detected {
		return nil, "", fmt.Errorf("entry detected as %s but format is configured as %s", detected, p.format)
	}

	return fields, detected, nil
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

	return p.extractk8sMetaFromFilePath(e)
}

// handleMoveAttributes moves fields to final attributes
func (*Parser) handleMoveAttributes(e *entry.Entry) error {
	// move `log` to `body` explicitly first to avoid
	// moving after more attributes have been added under the `log.*` key
	err := moveFieldToBody(e, "log", "body")
	if err != nil {
		return err
	}

	return moveField(e, "stream", "log.iostream")
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
			logPathField,
		)
	}

	rawLogPath, ok := logPath.(string)
	if !ok {
		return fmt.Errorf("type '%T' cannot be parsed as log path field", logPath)
	}

	var parsedValues map[string]any
	if p.cache != nil {
		if parsedValues, ok = p.cache.Get(rawLogPath); ok {
			return p.setK8sMetadataFromParsedValues(e, parsedValues)
		}
	}

	parsedValues, ok = parseLogPath(rawLogPath)
	if !ok {
		return errors.New("failed to detect a valid log path")
	}

	if p.cache != nil {
		p.cache.Add(rawLogPath, parsedValues)
	}

	pathMapPool.Put(parsedValues)
	return p.setK8sMetadataFromParsedValues(e, parsedValues)
}

func (*Parser) setK8sMetadataFromParsedValues(e *entry.Entry, parsedValues map[string]any) error {
	for attributeKey, value := range parsedValues {
		newField := entry.NewResourceField(attributeKey)
		if err := newField.Set(e, value); err != nil {
			return fmt.Errorf("failed to set %v as metadata at %v", value, attributeKey)
		}
	}
	return nil
}

func (p *Parser) consumeEntries(ctx context.Context, entries []*entry.Entry) {
	if err := p.WriteBatch(ctx, entries); err != nil {
		p.Logger().Error("failed to write batch of entries", zap.Error(err))
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

// parseCRI parses a raw CRI log line (containerd or crio format) without regex.
// It returns the parsed fields as a map, whether the format is containerd (vs crio), and whether parsing succeeded.
func parseCRI(raw string) (map[string]any, bool, bool) {
	i := strings.IndexByte(raw, ' ')
	if i <= 0 {
		return nil, false, false
	}
	timeVal := raw[:i]
	containerd := false
	if raw[i-1] == 'Z' {
		// containerd: [^ ^Z]+Z — excludes space, caret, and any Z before the trailing one
		if strings.ContainsAny(timeVal[:len(timeVal)-1], " ^Z") {
			return nil, false, false
		}
		containerd = true
	} else if strings.IndexByte(timeVal, 'Z') >= 0 { // crio: [^ Z]+
		return nil, false, false
	}

	rest := raw[i+1:]
	const n = len("stdout")
	if len(rest) < n+1 || rest[n] != ' ' {
		return nil, false, false
	}
	stream := rest[:n]
	if stream != "stdout" && stream != "stderr" {
		return nil, false, false
	}
	rest = rest[n+1:]

	var logtag, log string
	if before, after, ok := strings.Cut(rest, " "); ok {
		logtag, log = before, after
	} else {
		logtag = rest
	}

	// Use pool to reduce allocations
	m := mapPool.Get().(map[string]any)

	for k := range m {
		delete(m, k)
	}

	m["time"] = timeVal
	m["stream"] = stream
	m["logtag"] = logtag
	m["log"] = log
	return m, containerd, true
}

// stripLogSuffix validates and strips the log file suffix from a path.
// Returns the path without the suffix and true on success, empty string and false otherwise.
// Accepted: ".log"  or  ".log.YYYYMMDD-HHMMSS"  (mirrors \.log(\.\d{8}-\d{6})?$)
func stripLogSuffix(raw string) (string, bool) {
	const logExt = ".log"
	idx := strings.LastIndex(raw, logExt)
	if idx < 0 {
		return "", false
	}
	after := raw[idx+len(logExt):]
	switch {
	case after == "":
		// exactly ".log"
		return raw[:idx], true
	case len(after) == 16 && after[0] == '.' && after[9] == '-':
		// ".log.YYYYMMDD-HHMMSS" — validate digits
		rotation := after[1:] // "YYYYMMDD-HHMMSS"
		for i, c := range rotation {
			if i == 8 {
				if c != '-' {
					return "", false
				}
			} else if c < '0' || c > '9' {
				return "", false
			}
		}
		return raw[:idx], true
	default:
		return "", false
	}
}

// parseLogPath parses a Kubernetes pod log file path without regex.
// The expected format is: .../<namespace>_<pod_name>_<uid>/<container_name>/<restart_count>.log[.<rotation>]
// It returns the parsed fields keyed by OTel resource attribute names, and whether parsing succeeded.
//
// Validation rules (mirrors the previous regex):
//   - namespace/pod_name: any non-empty string not containing '_' (triplet is split by '_')
//   - uid: lowercase hex and '-' (no length enforcement)
//   - container_name: any non-empty string not containing '\\', '.', or '_'
//   - restart_count: digits only
//   - suffix: exactly ".log" or ".log." followed by 8 digits, a hyphen, and 6 digits
func parseLogPath(raw string) (map[string]any, bool) {
	// Validate and strip the suffix before touching any other path components.
	base, ok := stripLogSuffix(raw)
	if !ok {
		return nil, false
	}

	sep2 := strings.LastIndexAny(base, "/\\")
	if sep2 < 0 {
		return nil, false
	}
	restartCount := base[sep2+1:]
	if !isDigits(restartCount) {
		return nil, false
	}
	base = base[:sep2]

	sep1 := strings.LastIndexAny(base, "/\\")
	if sep1 < 0 {
		return nil, false
	}
	containerName := base[sep1+1:]
	if !isContainerName(containerName) {
		return nil, false
	}
	base = base[:sep1]

	sep0 := strings.LastIndexAny(base, "/\\")
	if sep0 < 0 {
		return nil, false
	}
	triplet := base[sep0+1:]
	if triplet == "" {
		return nil, false
	}

	lastUnd := strings.LastIndex(triplet, "_")
	if lastUnd < 0 {
		return nil, false
	}
	uid := triplet[lastUnd+1:]
	if !isUID(uid) {
		return nil, false
	}
	triplet = triplet[:lastUnd]

	lastUnd = strings.LastIndex(triplet, "_")
	if lastUnd < 0 {
		return nil, false
	}
	ns := triplet[:lastUnd]
	pod := triplet[lastUnd+1:]

	if !isNamespace(ns) {
		return nil, false
	}
	if !isPodName(pod) {
		return nil, false
	}

	m := pathMapPool.Get().(map[string]any)
	// Clear the map in case it was reused
	for k := range m {
		delete(m, k)
	}

	m["k8s.namespace.name"] = ns
	m["k8s.pod.name"] = pod
	m["k8s.pod.uid"] = uid
	m["k8s.container.name"] = containerName
	m["k8s.container.restart_count"] = restartCount

	return m, true
}

// isNamespace matches [^_]+ from the regex — any char except underscore, one or more.
func isNamespace(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c == '_' {
			return false
		}
	}
	return true
}

// isPodName matches [^_]+ from the regex — any char except underscore, one or more.
func isPodName(s string) bool {
	return isNamespace(s)
}

// isContainerName matches [^\._]+ from the regex — any char except backslash, dot, and underscore.
func isContainerName(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c == '\\' || c == '.' || c == '_' {
			return false
		}
	}
	return true
}

// isUID matches [a-f0-9\-]+ from the regex — lowercase hex and hyphens, one or more chars.
func isUID(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if (c < 'a' || c > 'f') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

// isDigits returns true if s is a non-empty string of ASCII digits.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
