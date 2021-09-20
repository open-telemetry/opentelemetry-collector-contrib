package instrumentationlibrary

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestTagsFromInstrumentationLibraryMetadata(t *testing.T) {
	tests := []struct {
		name         string
		version      string
		expectedTags []string
	}{
		{"test-il", "1.0.0", []string{fmt.Sprintf("%s:%s", instrumentationLibraryTag, "test-il"), fmt.Sprintf("%s:%s", instrumentationLibraryVersionTag, "1.0.0")}},
		{"test-il", "", []string{fmt.Sprintf("%s:%s", instrumentationLibraryTag, "test-il"), fmt.Sprintf("%s:%s", instrumentationLibraryVersionTag, "")}},
		{"", "1.0.0", []string{fmt.Sprintf("%s:%s", instrumentationLibraryTag, ""), fmt.Sprintf("%s:%s", instrumentationLibraryVersionTag, "1.0.0")}},
	}

	for _, testInstance := range tests {
		fmt.Println(testInstance)
		il := pdata.NewInstrumentationLibrary()
		il.SetName(testInstance.name)
		il.SetVersion(testInstance.version)
		tags := TagsFromInstrumentationLibraryMetadata(il)

		assert.ElementsMatch(t, testInstance.expectedTags, tags)
	}
}
