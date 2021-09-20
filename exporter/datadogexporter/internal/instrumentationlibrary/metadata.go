package instrumentationlibrary

import (
	"fmt"

	"go.opentelemetry.io/collector/model/pdata"
)

const (
	instrumentationLibraryTag        = "instrumentation_library"
	instrumentationLibraryVersionTag = "instrumentation_library_version"
)

// TagsFromInstrumentationLibraryMetadata takes the name and version of
// the instrumentation library and converts them to Datadog tags.
func TagsFromInstrumentationLibraryMetadata(il pdata.InstrumentationLibrary) []string {
	return []string{
		fmt.Sprintf("%s:%s", instrumentationLibraryTag, il.Name()),
		fmt.Sprintf("%s:%s", instrumentationLibraryVersionTag, il.Version()),
	}
}
