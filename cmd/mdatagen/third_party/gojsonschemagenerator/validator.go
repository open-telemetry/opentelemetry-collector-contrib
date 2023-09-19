package gojsonschemagenerator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/sanity-io/litter"

	"github.com/atombender/go-jsonschema/pkg/codegen"
)

type validator interface {
	generate(out *codegen.Emitter)
	desc() *validatorDesc
}

type validatorDesc struct {
	hasError            bool
	beforeJSONUnmarshal bool
}

var (
	_ validator = new(requiredValidator)
	_ validator = new(nullTypeValidator)
	_ validator = new(defaultValidator)
	_ validator = new(arrayValidator)
)

type requiredValidator struct {
	jsonName string
	declName string
}

func (v *requiredValidator) generate(out *codegen.Emitter) {
	out.Printlnf(`if v, ok := %s["%s"]; !ok || v == nil {`, varNameRawMap, v.jsonName)
	out.Indent(1)
	out.Printlnf(`return fmt.Errorf("field %s in %s: required")`, v.jsonName, v.declName)
	out.Indent(-1)
	out.Printlnf("}")
}

func (v *requiredValidator) desc() *validatorDesc {
	return &validatorDesc{
		hasError:            true,
		beforeJSONUnmarshal: true,
	}
}

type nullTypeValidator struct {
	jsonName   string
	fieldName  string
	arrayDepth int
}

func (v *nullTypeValidator) generate(out *codegen.Emitter) {
	value := fmt.Sprintf("%s.%s", varNamePlainStruct, v.fieldName)
	fieldName := v.jsonName

	var indexes []string

	for i := 0; i < v.arrayDepth; i++ {
		index := fmt.Sprintf("i%d", i)
		indexes = append(indexes, index)
		out.Printlnf(`for %s := range %s {`, index, value)
		value += fmt.Sprintf("[%s]", index)
		fieldName += "[%d]"

		out.Indent(1)
	}

	fieldName = fmt.Sprintf(`"%s"`, fieldName)
	if len(indexes) > 0 {
		fieldName = fmt.Sprintf(`fmt.Sprintf(%s, %s)`, fieldName, strings.Join(indexes, ", "))
	}

	out.Printlnf(`if %s != nil {`, value)
	out.Indent(1)
	out.Printlnf(`return fmt.Errorf("field %%s: must be null", %s)`, fieldName)
	out.Indent(-1)
	out.Printlnf("}")

	for i := 0; i < v.arrayDepth; i++ {
		out.Indent(-1)
		out.Printlnf("}")
	}
}

func (v *nullTypeValidator) desc() *validatorDesc {
	return &validatorDesc{
		hasError:            true,
		beforeJSONUnmarshal: false,
	}
}

type defaultValidator struct {
	jsonName         string
	fieldName        string
	defaultValueType codegen.Type
	defaultValue     interface{}
}

func (v *defaultValidator) generate(out *codegen.Emitter) {
	defaultValue, err := v.tryDumpDefaultSlice(out.MaxLineLength())
	if err != nil {
		// Fallback to sdump in case we couldn't dump it properly.
		defaultValue = litter.Sdump(v.defaultValue)
	}

	out.Printlnf(`if v, ok := %s["%s"]; !ok || v == nil {`, varNameRawMap, v.jsonName)
	out.Indent(1)
	out.Printlnf(`%s.%s = %s`, varNamePlainStruct, v.fieldName, defaultValue)
	out.Indent(-1)
	out.Printlnf("}")
}

func (v *defaultValidator) tryDumpDefaultSlice(maxLineLen uint) (string, error) {
	tmpEmitter := codegen.NewEmitter(maxLineLen)
	v.defaultValueType.Generate(tmpEmitter)
	tmpEmitter.Printlnf("{")

	kind := reflect.ValueOf(v.defaultValue).Kind()
	switch kind {
	case reflect.Slice:
		df, ok := v.defaultValue.([]interface{})
		if !ok {
			return "", errors.New("invalid default value")
		}

		for _, value := range df {
			tmpEmitter.Printlnf("%s,", litter.Sdump(value))
		}

	default:
		return "", errors.New("didn't find a slice to dump")
	}

	tmpEmitter.Printf("}")

	return tmpEmitter.String(), nil
}

func (v *defaultValidator) desc() *validatorDesc {
	return &validatorDesc{
		hasError:            false,
		beforeJSONUnmarshal: false,
	}
}

type arrayValidator struct {
	jsonName   string
	fieldName  string
	arrayDepth int
	minItems   int
	maxItems   int
}

func (v *arrayValidator) generate(out *codegen.Emitter) {
	if v.minItems == 0 && v.maxItems == 0 {
		return
	}

	value := fmt.Sprintf("%s.%s", varNamePlainStruct, v.fieldName)
	fieldName := v.jsonName

	var indexes []string

	for i := 1; i < v.arrayDepth; i++ {
		index := fmt.Sprintf("i%d", i)
		indexes = append(indexes, index)
		out.Printlnf(`for %s := range %s {`, index, value)
		value += fmt.Sprintf("[%s]", index)
		fieldName += "[%d]"

		out.Indent(1)
	}

	fieldName = fmt.Sprintf(`"%s"`, fieldName)
	if len(indexes) > 0 {
		fieldName = fmt.Sprintf(`fmt.Sprintf(%s, %s)`, fieldName, strings.Join(indexes, ", "))
	}

	if v.minItems != 0 {
		out.Printlnf(`if len(%s) < %d {`, value, v.minItems)
		out.Indent(1)
		out.Printlnf(`return fmt.Errorf("field %%s length: must be >= %%d", %s, %d)`, fieldName, v.minItems)
		out.Indent(-1)
		out.Printlnf("}")
	}

	if v.maxItems != 0 {
		out.Printlnf(`if len(%s) > %d {`, value, v.maxItems)
		out.Indent(1)
		out.Printlnf(`return fmt.Errorf("field %%s length: must be <= %%d", %s, %d)`, fieldName, v.maxItems)
		out.Indent(-1)
		out.Printlnf("}")
	}

	for i := 1; i < v.arrayDepth; i++ {
		out.Indent(-1)
		out.Printlnf("}")
	}
}

func (v *arrayValidator) desc() *validatorDesc {
	return &validatorDesc{
		hasError:            true,
		beforeJSONUnmarshal: false,
	}
}
