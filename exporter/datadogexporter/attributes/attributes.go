package attributes

import (
	"go.opentelemetry.io/collector/translator/conventions"
	"go.opentelemetry.io/collector/consumer/pdata"
)


// TagsFromAttributes converts a selected list of attributes
// to a tag list that can be added to metrics.
func TagsFromAttributes(attrs pdata.AttributeMap) []string {
	tags := make([]string, 0, attrs.Len())

	var processAttributes processAttributes

	attrs.ForEach(func(key string, value pdata.AttributeValue) {
		switch key {
		case conventions.AttributeProcessExecutableName:
			processAttributes.ExecutableName = value.StringVal()
		case conventions.AttributeProcessExecutablePath:
			processAttributes.ExecutablePath = value.StringVal()
		case conventions.AttributeProcessCommand:
			processAttributes.Command = value.StringVal()
		case conventions.AttributeProcessCommandLine:
			processAttributes.CommandLine = value.StringVal()
		case conventions.AttributeProcessID:
			processAttributes.PID = value.IntVal()
		case conventions.AttributeProcessOwner:
			processAttributes.Owner = value.StringVal()
		}
	})

	tags = append(tags, processAttributes.extractProcessTags()...)

	return tags
}