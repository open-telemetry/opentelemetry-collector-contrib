package ottlfuncs

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type ParseXMLArguments[K any] struct {
	Target ottl.StringGetter[K]
}

func NewParseXMLFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ParseXML", &ParseXMLArguments[K]{}, createParseXMLFunction[K])
}

func createParseXMLFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ParseXMLArguments[K])

	if !ok {
		return nil, fmt.Errorf("ParseXMLFactory args must be of type *ParseXMLArguments[K]")
	}

	return parseXML(args.Target, true), nil
}

// parseXML returns a `pcommon.Map` struct that is a result of parsing the target string as XML
// parseXML unmarshals an XML node to a map by:
//   - Placing the tag name in the #tag field
//   - Place any attributes in a field `@${attributeName}`, where attributeName is the name of the attribute
//   - Placing any trimmed text in the node into the #text field (only if there was any text)
//   - Placing any children into a key `${tag}#${position}`, where tag is the tag of the child,
//     and position is the 0-based index of the element.
func parseXML[K any](target ottl.StringGetter[K], strict bool) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		targetVal, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		parsedXML := anyXML{}

		decoder := xml.NewDecoder(strings.NewReader(targetVal))
		decoder.Strict = strict
		err = decoder.Decode(&parsedXML)
		if err != nil {
			return nil, fmt.Errorf("unmarshal xml: %w", err)
		}

		parsedMap := parsedXML.toMap()

		e := json.NewEncoder(os.Stdout)
		e.SetIndent("", "\t")
		_ = e.Encode(parsedMap)

		m := pcommon.NewMap()
		err = m.FromRaw(parsedMap)
		if err != nil {
			return nil, fmt.Errorf("create pcommon.Map: %w", err)
		}

		return m, nil
	}
}

const (
	// attributePrefix is deliberitely chosen to be a character that cannot start a name or token in XML
	// See: https://www.w3.org/TR/REC-xml/#d0e804
	attributePrefix = "@"
	// These fields are also chosen to start with a character that cannot start a name or token in XML (#)
	textField = "#chardata"
	tagField  = "#tag"
)

type anyXML struct {
	tag        string
	attributes []xml.Attr
	text       string
	children   []anyXML
}

func (a *anyXML) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	a.tag = start.Name.Local
	a.attributes = start.Attr

	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("decode next token: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			child := anyXML{}
			err := d.DecodeElement(&child, &t)
			if err != nil {
				return fmt.Errorf("decode start element: %w", err)
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

func (a anyXML) toMap() map[string]any {
	// 1 tag filed + 1 text field + 1 per attribute + 1 per child
	mapSize := 2 + len(a.attributes) + len(a.children)
	m := make(map[string]any, mapSize)

	m[tagField] = a.tag

	if a.text != "" {
		m[textField] = a.text
	}

	for _, attr := range a.attributes {
		m[attributePrefix+attr.Name.Local] = attr.Value
	}

	hasCollisions := xmlChildrenHaveNameCollisions(a.children)
	xmlAddChildrenToMap(m, a.children, hasCollisions)

	return m
}

func xmlAddChildrenToMap(m map[string]any, children []anyXML, includeSequenceNumber bool) {
	// Pad the number out so that the keys will sort properly for the same elements
	seqNumberDigits := int(math.Log10(float64(len(children)))) + 1

	for i, child := range children {
		mapKey := child.tag
		if includeSequenceNumber {
			mapKey = fmt.Sprintf("%s#%0*d", mapKey, seqNumberDigits, i)
		}
		m[mapKey] = child.toMap()
	}
}

// xmlChildrenHaveNameCollisions determines if children has any name collisions.
// if it does, we must add a sequence number to the name to avoid the collisions.
func xmlChildrenHaveNameCollisions(children []anyXML) bool {
	tagSet := make(map[string]struct{}, len(children))

	for _, child := range children {
		if _, ok := tagSet[child.tag]; ok {
			return true
		}

		tagSet[child.tag] = struct{}{}
	}

	return false
}
