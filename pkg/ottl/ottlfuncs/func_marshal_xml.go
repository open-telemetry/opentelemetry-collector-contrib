// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type MarshalXMLArguments[K any] struct {
	Target  ottl.PMapGetter[K]
	Version ottl.Optional[ottl.IntGetter[K]]
}

func NewMarshalXMLFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("MarshalXML", &MarshalXMLArguments[K]{}, createMarshalXMLFunction[K])
}

func createMarshalXMLFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*MarshalXMLArguments[K])

	if !ok {
		return nil, fmt.Errorf("MarshalXMLFactory args must be of type *MarshalXMLArguments[K]")
	}

	return marshalXML(args.Target, args.Version), nil
}

// marshalXML returns a string that is a result of marshaling the target `pcommon.Map` as XML
func marshalXML[K any](target ottl.PMapGetter[K], version ottl.Optional[ottl.IntGetter[K]]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
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

		targetVal, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		var root xmlElement
		if versionVal == 1 {
			root, err = mapToLegacyXMLElement(targetVal)

			if err != nil {
				return nil, err
			}

			bytes, marshalErr := xml.MarshalIndent(&root, "", "\t")
			if marshalErr != nil {
				return nil, fmt.Errorf("marshal xml: %w", marshalErr)
			}
			return string(bytes), nil
		}

		// this map should have a single key, which is the root element
		if targetVal.Len() != 1 {
			return nil, errors.New("target map must have a single key")
		}

		targetVal.Range(func(k string, v pcommon.Value) bool {

			switch v.Type() {
			case pcommon.ValueTypeStr:
				// special case for
				// <root>
				//  	value
				// </root>
				root = xmlElement{tag: k, text: v.Str()}
			case pcommon.ValueTypeMap:
				root = mapToXMLElement(v.Map(), k)
			default:
				err = fmt.Errorf("unsupported value type: %v", v.Type())
			}
			return false
		})
		if err != nil {
			return nil, err
		}

		bytes, err := xml.MarshalIndent(&root, "", "\t")
		if err != nil {
			return nil, fmt.Errorf("marshal xml: %w", err)
		}
		return string(bytes), nil
	}
}

// mapToXMLElement takes a pcommon.Map and converts it to an xmlElement
func mapToLegacyXMLElement(m pcommon.Map) (xmlElement, error) {
	var root xmlElement
	var err error
	m.Range(func(k string, v pcommon.Value) bool {

		switch k {
		case "tag":
			root.tag = v.Str()
		case "children":
			children := v.Slice()
			for i := 0; i < children.Len(); i++ {
				child := children.At(i)
				if child.Type() != pcommon.ValueTypeMap {
					// error
					err = errors.New("child must be a map")
					return false
				}
				childMap := child.Map()
				xmlChild, xmlErr := mapToLegacyXMLElement(childMap)
				if xmlErr != nil {
					err = xmlErr
					return false
				}
				root.children = append(root.children, xmlChild)
			}
		case "attributes":
			if v.Type() != pcommon.ValueTypeMap {
				// error
				err = errors.New("attributes must be a map")
				return false
			}
			v.Map().Range(func(k string, v pcommon.Value) bool {
				root.attributes = append(root.attributes, newXMLAttribute(k, v.Str()))
				return true
			})

		case "content":
			if v.Type() != pcommon.ValueTypeStr {
				// error
				err = errors.New("content must be a string")
				return false
			}
			root.text = v.Str()
		}
		return true
	})
	return root, err
}

func newXMLAttribute(name, value string) xml.Attr {
	return xml.Attr{Name: xml.Name{Local: name}, Value: value}
}

// mapToXMLElement takes a pcommon.Map and converts it to an xmlElement
func mapToXMLElement(m pcommon.Map, tag string) xmlElement {
	elem := xmlElement{tag: tag}
	var orderKey string
	var flattened bool
	m.Range(func(k string, v pcommon.Value) bool {
		switch k {
		case xmlAttributesKey:
			v.Map().Range(func(k string, v pcommon.Value) bool {
				elem.attributes = append(elem.attributes, newXMLAttribute(k, v.Str()))
				return true
			})
			return true

		case xmlValueKey:
			elem.text = v.Str()
			return true
		case xmlOrderingKey:
			orderKey = v.Str()
			return true
		}

		switch v.Type() {
		case pcommon.ValueTypeMap:
			children, childFlattened, childOrderKey := handleChildMap(v.Map(), k)
			elem.children = append(elem.children, children...)
			if childOrderKey != "" {
				orderKey = childOrderKey
			}
			flattened = childFlattened
		case pcommon.ValueTypeSlice:
			slice := v.Slice()
			for i := 0; i < slice.Len(); i++ {
				val := slice.At(i)
				if val.Type() == pcommon.ValueTypeStr {
					// If the nodes in the slice have no attributes or children,
					// the slice values will be the text for the node
					elem.children = append(elem.children, xmlElement{tag: k, text: val.Str()})
				}
				if val.Type() == pcommon.ValueTypeMap {
					elem.children = append(elem.children, mapToXMLElement(val.Map(), k))
				}
			}

		default:
			// if the value is not a map or a slice, then it is a leaf node
			elem.children = append(elem.children, xmlElement{tag: k, text: v.Str()})
		}
		return true
	})
	// order the children by the xmlOrderKey
	// if the key is not present, then the order is not guaranteed
	sortByOrderKey(elem.children, orderKey, flattened)

	return elem
}

func handleChildMap(childMap pcommon.Map, arrayTag string) ([]xmlElement, bool, string) {

	orderKey := ""
	var children []xmlElement
	// it the child is a flattened array, we need to handle it here
	flattenedAttr, flattened := childMap.Get(xmlFlattenedArrayKey)
	if !flattened {
		// typical case
		return []xmlElement{mapToXMLElement(childMap, arrayTag)}, flattened, orderKey
	}

	// handle the flattened array
	childMap.Range(func(k string, v pcommon.Value) bool {
		switch k {
		case xmlFlattenedArrayKey:
			return true
		case xmlOrderingKey:
			orderKey = v.Str()
			return true
		}
		child := xmlElement{tag: arrayTag, attributes: []xml.Attr{newXMLAttribute(flattenedAttr.Str(), k)}}

		switch v.Type() {
		case pcommon.ValueTypeStr:
			child.text = v.Str()
		case pcommon.ValueTypeMap:
			// TODO test this case
			child.children = append(child.children, mapToXMLElement(v.Map(), k))
		default:
			// TODO -- handle this case
		}

		children = append(children, child)

		return true
	})

	return children, flattened, orderKey
}

func sortByOrderKey(xml []xmlElement, orderKey string, flattenedArray bool) {
	if orderKey == "" {
		return
	}
	keys := strings.Split(orderKey, ",")
	// create a map of the keys to the index
	keyIndex := make(map[string]int)
	for i, key := range keys {
		keyIndex[key] = i
	}
	// sort the children by the order key

	slices.SortFunc(xml, func(i, j xmlElement) int {
		var iKey, jKey string
		if flattenedArray {
			iKey = i.attributes[0].Value
			jKey = j.attributes[0].Value
		} else {
			iKey = i.tag
			jKey = j.tag
		}
		iOrder, iPresent := keyIndex[iKey]
		jOrder, jPresent := keyIndex[jKey]
		if iPresent && jPresent {
			return iOrder - jOrder
		}
		if iPresent {
			return -1
		}
		if jPresent {
			return 1
		}
		return 0
	})
}
