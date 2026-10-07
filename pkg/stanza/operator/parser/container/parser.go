// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package container // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/container"

import (
	"context"
	"errors"
	"fmt"
	"maps"
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
	criTimeLayout       = "2006-01-02T15:04:05.999999999Z07:00"
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
	// mapPool reuses maps to reduce allocations for CRI log line parsing.
	mapPool = sync.Pool{
		New: func() any {
			return make(map[string]any, 4)
		},
	}
	// pathMapPool reuses maps for log path parsing.
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

		format := p.format
		if format == "" {
			format, err = p.detectFormat(ent)
			if err != nil {
				errs = append(errs, p.HandleEntryErrorWithWrite(ctx, ent, fmt.Errorf("failed to detect a valid container log format: %w", err), write))
				continue
			}
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

			if format == containerdFormat {
				m := mapPool.Get().(map[string]any)
				for k := range m {
					delete(m, k)
				}
				if err = p.ParseWith(ctx, ent, func(v any) (any, error) {
					return m, parseContainerdInto(m, v)
				}, write); err != nil {
					mapPool.Put(m)
					if !errors.Is(err, helper.ErrEntryHandled) {
						errs = append(errs, fmt.Errorf("failed to parse containerd log: %w", err))
					}
					continue
				}
				mapPool.Put(m)
				p.timeLayout = criTimeLayout
			} else {
				m := mapPool.Get().(map[string]any)
				for k := range m {
					delete(m, k)
				}
				if err = p.ParseWith(ctx, ent, func(v any) (any, error) {
					return m, parseCRIOInto(m, v)
				}, write); err != nil {
					mapPool.Put(m)
					if !errors.Is(err, helper.ErrEntryHandled) {
						errs = append(errs, fmt.Errorf("failed to parse crio log: %w", err))
					}
					continue
				}
				mapPool.Put(m)
				p.timeLayout = criTimeLayout
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

	format := p.format
	if format == "" {
		format, err = p.detectFormat(entry)
		if err != nil {
			return p.HandleEntryError(ctx, entry, fmt.Errorf("failed to detect a valid container log format: %w", err))
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

		if format == containerdFormat {
			m := mapPool.Get().(map[string]any)
			for k := range m {
				delete(m, k)
			}
			if err = p.ParseWith(ctx, entry, func(v any) (any, error) {
				return m, parseContainerdInto(m, v)
			}, p.Write); err != nil {
				mapPool.Put(m)
				if errors.Is(err, helper.ErrEntryHandled) {
					return nil
				}
				return fmt.Errorf("failed to parse containerd log: %w", err)
			}
			mapPool.Put(m)
			p.timeLayout = criTimeLayout
		} else {
			m := mapPool.Get().(map[string]any)
			for k := range m {
				delete(m, k)
			}
			if err = p.ParseWith(ctx, entry, func(v any) (any, error) {
				return m, parseCRIOInto(m, v)
			}, p.Write); err != nil {
				mapPool.Put(m)
				if errors.Is(err, helper.ErrEntryHandled) {
					return nil
				}
				return fmt.Errorf("failed to parse crio log: %w", err)
			}
			mapPool.Put(m)
			p.timeLayout = criTimeLayout
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

	if raw != "" && raw[0] == '{' {
		return dockerFormat, nil
	}

	timePart, rest, ok := strings.Cut(raw, " ")
	if !ok {
		return "", errors.New("could not split timestamp from log to detect format")
	}

	stream, _, ok := strings.Cut(rest, " ")
	if !ok || (stream != "stdout" && stream != "stderr") {
		return "", errors.New("could not split stream from log to detect format")
	}

	// Use isContainerdTimestamp instead of HasSuffix("Z") so that timezone-offset
	// timestamps (e.g. +02:00, -05:00) are correctly classified as containerd rather
	// than falling through to CRIO.
	if isContainerdTimestamp(timePart) {
		return containerdFormat, nil
	}

	return crioFormat, nil
}

// isContainerdTimestamp reports whether s matches [^ Z]+(?:Z|[+-]\d{2}:\d{2}).
func isContainerdTimestamp(s string) bool {
	if s == "" {
		return false
	}
	var prefix string
	if strings.HasSuffix(s, "Z") {
		prefix = s[:len(s)-1]
	} else {
		// timezone offset [+-]\d{2}:\d{2} — last 6 chars
		if len(s) < 7 {
			return false
		}
		off := s[len(s)-6:]
		if (off[0] != '+' && off[0] != '-') || off[3] != ':' {
			return false
		}
		for _, i := range []int{1, 2, 4, 5} {
			if off[i] < '0' || off[i] > '9' {
				return false
			}
		}
		prefix = s[:len(s)-6]
	}
	// prefix must be [^ Z]+ — at least one char, no spaces or Z
	return len(prefix) >= 1 && !strings.ContainsAny(prefix, " Z")
}

// parseContainerdInto parses a raw containerd CRI log line into m without allocating.
func parseContainerdInto(m map[string]any, value any) error {
	raw, ok := value.(string)
	if !ok {
		return fmt.Errorf("type '%T' cannot be parsed as containerd logs", value)
	}
	timePart, rest, ok := strings.Cut(raw, " ")
	if !ok || !isContainerdTimestamp(timePart) {
		return errors.New("could not parse containerd fields")
	}

	stream, rest, ok := strings.Cut(rest, " ")
	if !ok || (stream != "stdout" && stream != "stderr") {
		return errors.New("could not parse containerd fields")
	}

	logtag, logPart, ok := strings.Cut(rest, " ")
	if !ok {
		logtag = rest
		logPart = ""
	}

	m["time"] = timePart
	m["stream"] = stream
	m["logtag"] = logtag
	m["log"] = logPart
	return nil
}

// parseCRIOInto parses a raw CRI-O log line into m without allocating.
func parseCRIOInto(m map[string]any, value any) error {
	raw, ok := value.(string)
	if !ok {
		return fmt.Errorf("type '%T' cannot be parsed as cri-o container logs", value)
	}
	timePart, rest, ok := strings.Cut(raw, " ")
	if !ok {
		return errors.New("could not parse CRIO fields")
	}

	stream, rest, ok := strings.Cut(rest, " ")
	if !ok || (stream != "stdout" && stream != "stderr") {
		return errors.New("could not parse CRIO fields")
	}

	logtag, logPart, ok := strings.Cut(rest, " ")
	if !ok {
		logtag = rest
		logPart = ""
	}

	m["time"] = timePart
	m["stream"] = stream
	m["logtag"] = logtag
	m["log"] = logPart
	return nil
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

	if p.cache != nil {
		if cached, hit := p.cache.Get(rawLogPath); hit {
			return p.setK8sMetadataFromParsedValues(e, cached)
		}
	}

	m := pathMapPool.Get().(map[string]any)
	for k := range m {
		delete(m, k)
	}
	if !parseLogPathInto(m, rawLogPath) {
		pathMapPool.Put(m)
		return errors.New("failed to detect a valid log path")
	}

	if p.cache != nil {
		cachedMap := make(map[string]any, len(m))
		maps.Copy(cachedMap, m)
		p.cache.Add(rawLogPath, cachedMap)
	}

	err := p.setK8sMetadataFromParsedValues(e, m)
	pathMapPool.Put(m)
	return err
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

// parseLogPathInto parses a Kubernetes pod log file path into m without allocating.
func parseLogPathInto(m map[string]any, raw string) bool {
	base, ok := stripLogSuffix(raw)
	if !ok {
		return false
	}

	sep2 := strings.LastIndexAny(base, "/\\")
	if sep2 < 0 {
		return false
	}
	restartCount := base[sep2+1:]
	if !isDigits(restartCount) {
		return false
	}
	base = base[:sep2]

	sep1 := strings.LastIndexAny(base, "/\\")
	if sep1 < 0 {
		return false
	}
	containerName := base[sep1+1:]
	if !isValidContainerName(containerName) {
		return false
	}
	base = base[:sep1]

	sep0 := strings.LastIndexAny(base, "/\\")
	if sep0 < 0 {
		return false
	}
	triplet := base[sep0+1:]
	if triplet == "" {
		return false
	}

	lastUnd := strings.LastIndex(triplet, "_")
	if lastUnd < 0 {
		return false
	}
	uid := triplet[lastUnd+1:]
	if !isValidUID(uid) {
		return false
	}
	triplet = triplet[:lastUnd]

	lastUnd = strings.LastIndex(triplet, "_")
	if lastUnd < 0 {
		return false
	}
	ns := triplet[:lastUnd]
	pod := triplet[lastUnd+1:]

	if !isValidPodOrNamespace(ns) {
		return false
	}
	if !isValidPodOrNamespace(pod) {
		return false
	}

	m["k8s.namespace.name"] = ns
	m["k8s.pod.name"] = pod
	m["k8s.pod.uid"] = uid
	m["k8s.container.name"] = containerName
	m["k8s.container.restart_count"] = restartCount

	return true
}

// isValidPodOrNamespace matches [^_]+ from the regex — any char except underscore, one or more.
func isValidPodOrNamespace(s string) bool {
	if s == "" {
		return false
	}

	if strings.Contains(s, "_") {
		return false
	}

	return true
}

// isValidContainerName matches [^\._]+ from the regex — any char except dot and underscore, one or more.
func isValidContainerName(s string) bool {
	if s == "" {
		return false
	}

	if strings.ContainsAny(s, "._") {
		return false
	}
	return true
}

// isValidUID matches [a-f0-9\-]+ from the regex — lowercase hex and hyphens, one or more chars.
func isValidUID(s string) bool {
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
