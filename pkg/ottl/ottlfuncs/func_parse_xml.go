// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

// These are metadata keys used in the map returned by the ParseXML function
// so that the parsed XML can be serialized back to XML.
// TODO these should be settable by the user

var (
	xmlAttributesKey     = "xml_attributes"
	xmlValueKey          = "xml_value"
	xmlFlattenedArrayKey = "xml_flattened_array"
	xmlOrderingKey       = "xml_ordering"
)

type ParseXMLArguments[K any] struct {
	Target        ottl.StringGetter[K]
	Version       ottl.Optional[ottl.IntGetter[K]]
	FlattenArrays ottl.Optional[ottl.BoolGetter[K]]
}

func NewParseXMLFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ParseXML", &ParseXMLArguments[K]{}, createParseXMLFunction[K])
}

func createParseXMLFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ParseXMLArguments[K])

	if !ok {
		return nil, fmt.Errorf("ParseXMLFactory args must be of type *ParseXMLArguments[K]")
	}

	return parseXML(args.Target, args.Version, args.FlattenArrays), nil
}

// parseXML returns a `pcommon.Map` struct that is a result of parsing the target string as XML
func parseXML[K any](target ottl.StringGetter[K], version ottl.Optional[ottl.IntGetter[K]], flatten ottl.Optional[ottl.BoolGetter[K]]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		targetVal, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		versionVal := int64(1)
		if !version.IsEmpty() {
			getter := version.Get()
			getterVal, getterErr := getter.Get(ctx, tCtx)
			if getterErr != nil {
				return nil, getterErr
			}
			switch getterVal {
			case 1, 2:
				versionVal = getterVal
			default:
				return nil, fmt.Errorf("version must be 1 or 2, got %d", getterVal)
			}

		}

		flattenArrays := false
		if !flatten.IsEmpty() {
			getter := flatten.Get()
			getterVal, getterErr := getter.Get(ctx, tCtx)
			if getterErr != nil {
				return nil, getterErr
			}
			flattenArrays = getterVal
		}

		parsedXML := xmlElement{}

		decoder := xml.NewDecoder(strings.NewReader(targetVal))
		err = decoder.Decode(&parsedXML)
		if err != nil {
			return nil, fmt.Errorf("unmarshal xml: %w", err)
		}

		if decoder.InputOffset() != int64(len(targetVal)) {
			return nil, errors.New("trailing bytes after parsing xml")
		}

		parsedMap := pcommon.NewMap()
		if versionVal == 1 {
			parsedXML.intoMapLegacy(parsedMap)
		} else {
			parsedXML.intoMap(parsedMap, flattenArrays)
		}
		return parsedMap, nil
	}
}

type xmlElement struct {
	tag        string
	attributes []xml.Attr
	text       string
	children   []xmlElement
	order      int
}

// UnmarshalXML implements xml.Unmarshaler for xmlElement
func (a *xmlElement) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	a.tag = start.Name.Local
	a.attributes = start.Attr

	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("decode next token: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			child := xmlElement{}
			err := d.DecodeElement(&child, &t)
			if err != nil {
				return err
			}

			a.children = append(a.children, child)
		case xml.EndElement:
			// End element means we've reached the end of parsing
			return nil
		case xml.CharData:
			// Strip leading/trailing spaces to ignore newlines and
			// indentation in formatted XML
			a.text += string(bytes.TrimSpace([]byte(t)))
		case xml.Comment: // ignore comments
		case xml.ProcInst: // ignore processing instructions
		case xml.Directive: // ignore directives
		default:
			return fmt.Errorf("unexpected token type %T", t)
		}
	}
}

// intoMap converts and adds the xmlElement into the provided pcommon.Map.
func (a xmlElement) intoMapLegacy(m pcommon.Map) {
	m.EnsureCapacity(4)

	m.PutStr("tag", a.tag)

	if a.text != "" {
		m.PutStr("content", a.text)
	}

	if len(a.attributes) > 0 {
		attrs := m.PutEmptyMap("attributes")
		attrs.EnsureCapacity(len(a.attributes))

		for _, attr := range a.attributes {
			attrs.PutStr(attr.Name.Local, attr.Value)
		}
	}

	if len(a.children) > 0 {
		children := m.PutEmptySlice("children")
		children.EnsureCapacity(len(a.children))

		for _, child := range a.children {
			child.intoMapLegacy(children.AppendEmpty().SetEmptyMap())
		}
	}
}

// intoMap converts and adds the xmlElement into the provided pcommon.Map.
func (a xmlElement) intoMap(m pcommon.Map, flattenArrays bool) {

	// if there are no attributes or children, then we can just put the text
	// as the value of the the tag key

	if len(a.attributes) == 0 && len(a.children) == 0 {
		m.PutStr(a.tag, a.text)
		return
	}

	// we have attributes, children, or both, so we need to
	// create an empty map for the tag key and add the attributes
	node := m.PutEmptyMap(a.tag)
	a.childIntoMap(node, flattenArrays)
}

func (a xmlElement) childIntoMap(m pcommon.Map, flattenArrays bool) {
	// as a child node, the tag will be the key in the map at the level above, and
	// we will ignore it. We also will have either attributes or children, otherwise
	// we would have been a leaf node and we would have been handled by the previous
	// recursion level

	if len(a.attributes) > 0 {
		attrs := m.PutEmptyMap(xmlAttributesKey)
		attrs.EnsureCapacity(len(a.attributes))
		for _, attr := range a.attributes {
			attrs.PutStr(attr.Name.Local, attr.Value)
		}
	}

	if a.text != "" {
		m.PutStr(xmlValueKey, a.text)
	}

	// we need to group the children by tag name so that we can put them in a slice
	// We also need to preserve ordering information

	childrenGrouping := map[string][]*xmlElement{}
	for i := range a.children {
		child := &a.children[i]
		child.order = i

		childrenGrouping[child.tag] = append(childrenGrouping[child.tag], child)
	}

	// these are the ordering keys for the children that don't end up in a flattened array
	orderingKeys := make([]string, len(a.children))
	for tag, children := range childrenGrouping {

		// Only one child with the same tag, no need for slice
		// The child tag will be the key in the current map.
		if len(children) == 1 {
			c := children[0]
			orderingKeys[c.order] = tag

			// If the child has no attributes or children, then we can just put the text
			// as the value of the the tag key
			if len(c.attributes) == 0 && len(c.children) == 0 {
				m.PutStr(tag, c.text)
				continue
			}
			c.childIntoMap(m.PutEmptyMap(tag), flattenArrays)
			continue
		}

		// we're only going to flatten if we have a single grouping
		if len(childrenGrouping) == 1 && arrayCanBeFlattened(children) && flattenArrays {
			arrayMap := m.PutEmptyMap(tag)
			// add a special tag to indicate that this is a flattened array
			commonAttr := children[0].attributes[0].Name.Local
			arrayMap.PutStr(xmlFlattenedArrayKey, commonAttr)
			for _, c := range children {
				// the child has one common attribute, and either children or a value.
				key := c.attributes[0].Value
				orderingKeys[c.order] = key
				if len(c.children) == 0 {
					arrayMap.PutStr(key, c.text)
					continue
				}
				childMap := arrayMap.PutEmptyMap(key)
				c.childIntoMap(childMap, flattenArrays)
			}
			// add the ordering key
			if len(orderingKeys) > 1 {
				arrayMap.PutStr(xmlOrderingKey, strings.Join(orderingKeys, ","))
			}
			return
		}
		// More than one child with the same tag, we need to put them in a slice
		slice := m.PutEmptySlice(tag)
		for i, child := range children {
			// If the child has no attributes or children, then we can just put the text
			// as the value of the tag key
			if len(child.attributes) == 0 && len(child.children) == 0 {
				slice.AppendEmpty().SetStr(child.text)
				continue
			}
			childMap := slice.AppendEmpty().SetEmptyMap()
			child.childIntoMap(childMap, flattenArrays)
			orderingKeys[child.order] = fmt.Sprintf("%s.%d", tag, i)
		}
	}
	// add the ordering key
	if len(orderingKeys) > 1 {
		m.PutStr(xmlOrderingKey, strings.Join(orderingKeys, ","))
	}
}

func arrayCanBeFlattened(arr []*xmlElement) bool {

	// If we have a structure like this:
	//
	// <EventData>
	// 		<Data Name='SubjectUserSid'>S-1-0-0</Data>
	// 		<Data Name='SubjectUserName'>-</Data>
	// 		<Data
	// 			Name='SubjectDomainName'>-</Data>
	// 		<Data Name='SubjectLogonId'>0x0</Data>
	// 		<Data
	// 			Name='TargetUserSid'>S-1-0-0</Data>
	// 		<Data Name='TargetUserName'>Administrator</Data>
	// 		<Data
	// 			Name='TargetDomainName'>domain</Data>
	// 		<Data Name='Status'>0xc000006d</Data>
	// 		<Data
	// 			Name='FailureReason'>%%2313</Data>
	// 		<Data Name='SubStatus'>0xc000006a</Data>
	// </EventData>
	//
	// we can flatten the structure into a map when all the children have:
	//  - the same tag
	//	- one attribute, with the same name
	//	- either children or a value
	//
	// 		EventData["Data"]["SubjectUserSid"]				S-1-0-0
	// 		EventData["Data"]["SubjectDomainName"]			-
	// 		EventData["Data"]["TargetUserSid"]				S-1-0-0
	// 		EventData["Data"]["TargetDomainName"]			domain
	// 		EventData["Data"]["Status"]						0xc000006d
	// 		EventData["Data"]["FailureReason"]				%%2313
	// 		EventData["Data"]["SubStatus"]					0xc000006a
	// 		EventData["Data"][xmlFlattenedArrayKey]		    Name

	for _, elem := range arr {
		if len(elem.attributes) != 1 {
			return false
		}
		if elem.attributes[0].Name != arr[0].attributes[0].Name {
			return false
		}
		if len(elem.children) != 0 && elem.text != "" {
			return false
		}
	}
	return true
}

func (a *xmlElement) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	// Set the start element with the tag name and attributes
	start.Name.Local = a.tag
	start.Attr = a.attributes

	// Start the element
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("encode start element: %w", err)
	}

	// If the element has text, encode it
	if a.text != "" {
		if err := e.EncodeToken(xml.CharData(a.text)); err != nil {
			return fmt.Errorf("encode char data: %w", err)
		}
	}

	// Recursively encode child elements
	for _, child := range a.children {
		child := child
		if err := e.Encode(&child); err != nil {
			return fmt.Errorf("encode child element: %w", err)
		}
	}

	// End the element
	if err := e.EncodeToken(start.End()); err != nil {
		return fmt.Errorf("encode end element: %w", err)
	}

	// Flush to ensure everything is written
	return e.Flush()
}
