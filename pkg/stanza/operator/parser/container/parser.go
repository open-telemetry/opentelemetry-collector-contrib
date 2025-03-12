// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package container // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/container"

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-json"
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
	logpathPattern      = "^.*(\\/|\\\\)(?P<namespace>[^_]+)_(?P<pod_name>[^_]+)_(?P<uid>[a-f0-9\\-]+)(\\/|\\\\)(?P<container_name>[^\\._]+)(\\/|\\\\)(?P<restart_count>\\d+)\\.log$"
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
}

func (p *Parser) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return p.ProcessBatchWith(ctx, entries, p.Process)
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
		err = p.ParserOperator.ProcessWithCallback(ctx, entry, p.parseDocker, p.handleTimeAndAttributeMappings)
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
			err = p.ParserOperator.ParseWith(ctx, entry, p.parseContainerd)
			if err != nil {
				return fmt.Errorf("failed to parse containerd log: %w", err)
			}
			p.timeLayout = goTimeLayout
		} else {
			// parse the message
			err = p.ParserOperator.ParseWith(ctx, entry, p.parseCRIO)
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
		return fmt.Errorf("failed to detect a valid container log format")
	}

	return nil
}

// Stop ensures that the internal recombineParser, the internal criLogEmitter and
// the crioConsumer are stopped in the proper order without being affected by
// any possible race conditions
func (p *Parser) Stop() error {
	if !p.asyncConsumerStarted {
		// nothing is started return
		return nil
	}
	var stopErrs []error
	err := p.recombineParser.Stop()
	if err != nil {
		stopErrs = append(stopErrs, fmt.Errorf("unable to stop the internal recombine operator: %w", err))
	}
	// the recombineParser will call the Process of the criLogEmitter synchronously so the entries will be first
	// written to the channel before the Stop of the recombineParser returns. Then since the criLogEmitter handles
	// the entries synchronously it is safe to call its Stop.
	// After criLogEmitter is stopped the crioConsumer will consume the remaining messages and return.
	err = p.criLogEmitter.Stop()
	if err != nil {
		stopErrs = append(stopErrs, fmt.Errorf("unable to stop the internal LogEmitter: %w", err))
	}
	p.criConsumers.Wait()
	return errors.Join(stopErrs...)
}

// detectFormat will detect the container log format
func (p *Parser) detectFormat(e *entry.Entry) (string, error) {
	value, ok := e.Get(p.ParseFrom)
	if !ok {
		return "", fmt.Errorf("entry cannot be parsed as container logs")
	}

	raw, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("type '%T' cannot be parsed as container logs", value)
	}

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
func (p *Parser) parseCRIO(value any) (any, error) {
	raw, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("type '%T' cannot be parsed as cri-o container logs", value)
	}

	return helper.MatchValues(raw, crioMatcher)
}

// parseContainerd will parse a containerd log value based on a fixed regexp
func (p *Parser) parseContainerd(value any) (any, error) {
	raw, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("type '%T' cannot be parsed as containerd logs", value)
	}

	return helper.MatchValues(raw, containerdMatcher)
}

// parseDocker will parse a docker log value as JSON
func (p *Parser) parseDocker(value any) (any, error) {
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
func (p *Parser) handleMoveAttributes(e *entry.Entry) error {
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

	logPath := e.Attributes[logPathField]
	rawLogPath, ok := logPath.(string)
	if !ok {
		return fmt.Errorf("type '%T' cannot be parsed as log path field", logPath)
	}

	parsedValues, err := helper.MatchValues(rawLogPath, pathMatcher)
	if err != nil {
		return fmt.Errorf("failed to detect a valid log path")
	}

	for originalKey, attributeKey := range k8sMetadataMapping {
		newField := entry.NewResourceField(attributeKey)
		if err := newField.Set(e, parsedValues[originalKey]); err != nil {
			return fmt.Errorf("failed to set %v as metadata at %v", originalKey, attributeKey)
		}
	}
	return nil
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
