package attributes

import (
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"

	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTagsFromAttributes(t *testing.T) {
	attributeMap := map[string]pdata.AttributeValue{
		conventions.AttributeProcessExecutableName: pdata.NewAttributeValueString("otelcol"),
		conventions.AttributeProcessExecutablePath: pdata.NewAttributeValueString("/usr/bin/cmd/otelcol"),
		conventions.AttributeProcessCommand:        pdata.NewAttributeValueString("cmd/otelcol"),
		conventions.AttributeProcessCommandLine:    pdata.NewAttributeValueString("cmd/otelcol --config=\"/path/to/config.yaml\""),
		conventions.AttributeProcessID:             pdata.NewAttributeValueInt(1),
		conventions.AttributeProcessOwner:          pdata.NewAttributeValueString("root"),
	}
	attrs := pdata.NewAttributeMap().InitFromMap(attributeMap)


	assert.Equal(t, []string{
		fmt.Sprintf("%s:%s", conventions.AttributeProcessExecutableName, "otelcol"),
	}, TagsFromAttributes(attrs))
}

func TestTagsFromAttributesEmpty(t *testing.T) {
	attrs := pdata.NewAttributeMap()


	assert.Equal(t, []string{}, TagsFromAttributes(attrs))
}