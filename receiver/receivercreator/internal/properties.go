// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator/internal"

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

const (
	ConfigType             string = "config"
	RuleType               string = "rule"
	ResourceAttributesType string = "resource_attributes"
	FullMappingType        string = ""
)

var doubleUnderscoreRE = regexp.MustCompile("__")

// Receiver creator endpoint properties are the method of configuring individual receivers from a given observer endpoint environment.
// (io.opentelemetry.collector.receiver-creator.|receiver-creator.collector.opentelemetry.io/)<receiver-type(/name)>: <full mapping value>
// (io.opentelemetry.collector.receiver-creator.|receiver-creator.collector.opentelemetry.io/)<receiver-type(/name)>.config.<field>(<::subfield>)*: value
// (io.opentelemetry.collector.receiver-creator.|receiver-creator.collector.opentelemetry.io/)<receiver-type(/name)>.rule: value
// (io.opentelemetry.collector.receiver-creator.|receiver-creator.collector.opentelemetry.io/)<receiver-type(/name)>.resource_attributes.<resource_attribute>: value
// (io.opentelemetry.collector.receiver-creator.|receiver-creator.collector.opentelemetry.io/)<receiver-type(/name)>.resource_attributes: "attribute: value"
// Parsing properties requires lookaheads (backtracking) so we use participle instead of re2.

var lex = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Dot", Pattern: `\.`},
	{Name: "ForwardSlash", Pattern: `/|__`},
	{Name: "Whitespace", Pattern: `\s+`},
	// Anything that isn't a Dot or ForwardSlash (doesn't support terminating '_').
	{Name: "String", Pattern: `([^./_]*(_[^./_])*)*`},
})

var parser = participle.MustBuild[property](
	participle.Lexer(lex),
	participle.Elide("Whitespace"),
	participle.UseLookahead(participle.MaxLookahead),
)

// property is the ast for a parsed property.
type property struct {
	// map storing the parsed contents for confmap.Conf form
	stringMap map[string]map[string]any
	// Namespace support `io.opentelemetry.collector.receiver-creator.` reverse-DNS (docker) and `receiver-creator.collector.opentelemetry.io/` prefixes
	Namespace string `parser:"@(('io' Dot 'opentelemetry' Dot 'collector' Dot 'receiver-creator' Dot)|('receiver-creator' Dot 'collector' Dot 'opentelemetry' Dot 'io' ForwardSlash))"`
	Receiver  CID    `parser:"@@"`
	Type      string `parser:"( ( Dot ( @('config'|'resource_attributes') Dot? ) | Dot @'rule' ) | @'' )"`
	Field     string `parser:"@(String|Dot|ForwardSlash)*"`
	Val       string
	Raw       string
}

type CID struct {
	// CID.Type is anything that isn't the ForwardSlash or Type identifier
	Type string `parser:"@~(ForwardSlash | (Dot (?= ('config'|'rule'|'resource_attributes'))))+"`
	// CID.Name is anything after a ForwardSlash that isn't the Type identifier
	Name string `parser:"(ForwardSlash @(~(Dot (?= ('config'|'rule'|'resource_attributes')))+)*)?"`
}

func newProperty(key, value string) (*property, bool, error) {
	p, err := parser.ParseString("receiver_creator", key)
	if err != nil {
		return nil, false, fmt.Errorf("invalid property %q (parsing error): %w", key, err)
	}
	p.Raw = key
	p.Val = value
	p.Receiver.Name = doubleUnderscoreRE.ReplaceAllString(p.Receiver.Name, "/")
	switch p.Type {
	case FullMappingType:
		var dst map[string]any
		if err = yaml.Unmarshal([]byte(value), &dst); err != nil {
			return nil, true, fmt.Errorf("failed unmarshaling full mapping property (%q) -> %w", key, err)
		}
		sm := confmap.NewFromStringMap(dst).ToStringMap()
		var invalidKeys []string
		var invalidVals []string
		propStringMap := map[string]map[string]any{}
		for k, v := range sm {
			switch k {
			case RuleType, ConfigType, ResourceAttributesType:
				var vMap map[string]any
				var ok bool
				if k == RuleType {
					vMap = map[string]any{RuleType: v}
				} else {
					if vMap, ok = v.(map[string]any); !ok {
						invalidVals = append(invalidVals, fmt.Sprintf("%s: %v", k, v))
						continue
					}
				}
				propStringMap[k] = vMap
			default:
				invalidKeys = append(invalidKeys, k)
			}
		}
		if len(invalidKeys) != 0 {
			return nil, true, fmt.Errorf(`invalid endpoint property keys: %v. Must be in ["config", "resource_attributes", "rule"]`, invalidKeys)
		}
		if len(invalidVals) != 0 {
			return nil, true, fmt.Errorf(`invalid endpoint property vals: %v. Must be valid yaml mapping`, invalidVals)
		}
		p.stringMap = propStringMap
	case RuleType:
		p.stringMap = map[string]map[string]any{p.Type: {p.Type: p.Val}}
	case ConfigType, ResourceAttributesType:
		var dst map[string]any
		var cfgItem []byte

		if strings.Contains(value, confmap.KeyDelimiter) {
			var innerDst map[string]any
			innerItem := []byte(value)
			if e := yaml.Unmarshal(innerItem, &innerDst); e != nil {
				return nil, true, fmt.Errorf("failed unmarshaling inner property item %q: %w", innerItem, err)
			}
			innerYaml := confmap.NewFromStringMap(innerDst).ToStringMap()
			if p.Field != "" {
				innerYaml = map[string]any{
					p.Field: innerYaml,
				}
				p.Field = ""
			}
			if v, e := yaml.Marshal(innerYaml); e == nil {
				value = string(v)
			}
		}

		if p.Field == "" {
			cfgItem = []byte(value)
		} else {
			cfgItem = []byte(fmt.Sprintf("%s: %s", p.Field, value))
		}
		if err = yaml.Unmarshal(cfgItem, &dst); err != nil {
			return nil, true, fmt.Errorf("failed unmarshaling property (%q) %q -> %w", key, cfgItem, err)
		}
		p.stringMap = map[string]map[string]any{
			p.Type: confmap.NewFromStringMap(dst).ToStringMap(),
		}
	default:
		return nil, true, fmt.Errorf("invalid type %q from %q (%q)", p.Type, key, p.componentID().String())
	}
	return p, true, nil
}

// toStringMap returns a map[string]any equivalent to the sub-level confmap.ToStringMap()
func (p *property) toStringMap() map[string]any {
	if p != nil {
		if p.Type == FullMappingType {
			sm := map[string]any{}
			for k, v := range p.stringMap {
				var val any = v
				if k == RuleType {
					val = p.stringMap[RuleType][RuleType]
				}
				sm[k] = val
			}
			return sm
		}
		return p.stringMap[p.Type]
	}
	return nil
}

// componentID returns the property's receiver's component.ID
func (p *property) componentID() component.ID {
	if p != nil {
		return component.NewIDWithName(component.Type(p.Receiver.Type), p.Receiver.Name)
	}
	return component.ID{}
}

func PropertyConfFromEndpointEnv(details observer.EndpointDetails, logger *zap.Logger) *confmap.Conf {
	var properties []property

	endpointType := details.Type()
	env := details.Env()

	var candidates map[string]string
	switch endpointType {
	case observer.ContainerType:
		if l, hasLabels := env["labels"]; hasLabels {
			if labels, isMap := l.(map[string]string); isMap {
				candidates = labels
			}
		}
	case observer.PodType, observer.PortType, observer.K8sNodeType:
		if endpointType == observer.PortType {
			// port endpoints nest pod env
			if pod, hasPod := env["pod"]; hasPod {
				if e, isEnv := pod.(observer.EndpointEnv); isEnv {
					env = e
				}
			}
		}
		if a, hasAnnotations := env["annotations"]; hasAnnotations {
			if annotations, isMap := a.(map[string]string); isMap {
				candidates = annotations
			}
		}
	default:
		panic(endpointType)
	}

	for key, val := range candidates {
		prop, isProp, err := newProperty(key, val)
		if !isProp {
			continue
		}
		if err != nil {
			logger.Info("error creating property", zap.String("parsed", key), zap.Error(err))
			continue
		}
		if prop == nil {
			// programming error - shouldn't be reachable
			logger.Warn("unexpectedly nil parsed property", zap.String("parsed", key))
			continue
		}
		properties = append(properties, *prop)
	}

	// for best effort determinism in confmap.Merge()
	sortProperties(properties)

	confFromProperties := confmap.New()
	for _, prop := range properties {
		receiverID := prop.componentID().String()
		rMap, err := confFromProperties.Sub(receiverID)
		if err != nil {
			logger.Info(fmt.Sprintf("error accessing %q config", receiverID), zap.Error(err))
			continue
		}
		switch prop.Type {
		case FullMappingType:
			if mergeErr := rMap.Merge(confmap.NewFromStringMap(prop.toStringMap())); mergeErr != nil {
				logger.Info(fmt.Sprintf("%q rMap merge error", receiverID), zap.Error(mergeErr))
				continue
			}
		case RuleType:
			if mergeErr := rMap.Merge(confmap.NewFromStringMap(prop.toStringMap())); mergeErr != nil {
				logger.Info(fmt.Sprintf("%q rMap merge error", receiverID), zap.Error(mergeErr))
				continue
			}
		case ConfigType, ResourceAttributesType:
			subMap, err := rMap.Sub(prop.Type)
			if err != nil {
				logger.Info(fmt.Sprintf("error accessing %q sub-config", receiverID), zap.Error(err))
				continue
			}
			propConfigMap := confmap.NewFromStringMap(prop.toStringMap())
			if mergeErr := subMap.Merge(propConfigMap); mergeErr != nil {
				logger.Info(fmt.Sprintf("%q %v subMap merge error", receiverID, prop.Type), zap.Error(mergeErr))
				continue
			}
			if mergeErr := rMap.Merge(confmap.NewFromStringMap(map[string]any{prop.Type: subMap.ToStringMap()})); mergeErr != nil {
				logger.Info(fmt.Sprintf("%q %v subMap merge error", prop.Type, receiverID), zap.Error(mergeErr))
				continue
			}
		}
		if mergeErr := confFromProperties.Merge(confmap.NewFromStringMap(map[string]any{receiverID: rMap.ToStringMap()})); mergeErr != nil {
			logger.Info(fmt.Sprintf("%q confFromProperties merge error", receiverID), zap.Error(mergeErr))
			continue
		}
	}
	return confFromProperties
}

func sortProperties(properties []property) {
	sort.SliceStable(properties, func(i, j int) bool {
		propI := properties[i]
		propJ := properties[j]
		if propI.Type != propJ.Type {
			// Full mapping will always be first (and in practice can only one to exist)
			return propI.Type < propJ.Type
		}

		var mapI, mapJ map[string]any

		switch propI.Type {
		case ConfigType, ResourceAttributesType:
			mapI = propI.toStringMap()
			mapJ = propJ.toStringMap()
		default:
			// RuleType (shouldn't be reached otherwise)
			return propI.Val < propJ.Val
		}
		if len(mapI) == 0 || len(mapJ) == 0 {
			return len(mapI) < len(mapJ)
		}
		var mapIKeys, mapJKeys []string
		for k := range mapI {
			mapIKeys = append(mapIKeys, k)
		}
		for k := range mapJ {
			mapJKeys = append(mapJKeys, k)
		}
		sort.Strings(mapIKeys)
		sort.Strings(mapJKeys)
		for ik, iKey := range mapIKeys {
			if len(mapJKeys) <= ik {
				return false
			}
			jKey := mapJKeys[ik]
			if jKey != iKey {
				return iKey < jKey
			}
			iVal := fmt.Sprintf("%v", mapI[iKey])
			jVal := fmt.Sprintf("%v", mapJ[jKey])
			if iVal != jVal {
				return iVal < jVal
			}
		}
		return len(mapIKeys) < len(mapJKeys)
	})
}
