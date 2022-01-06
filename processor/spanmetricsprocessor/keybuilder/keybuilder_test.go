package keybuilder

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNew(t *testing.T) {
	t.Run("should create new keybuilder", func(t *testing.T) {
		assert.NotPanics(t, func() {
			assert.NotNil(t, New())
		})
	})
}

func Test_metricKeyBuilder_Append(t *testing.T) {
	tests := []struct {
		name   string
		args   [][]string
		result string
	}{
		{
			name:   "should skip empty string",
			args:   [][]string{{"", "abc", "", "def"}},
			result: fmt.Sprintf("abc%sdef", separator),
		},
		{
			name:   "should concat multiple append",
			args:   [][]string{{"abc", "def"}, {"", "hij"}},
			result: fmt.Sprintf("abc%sdef%shij", separator, separator),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kb := New()
			for _, arg := range tt.args {
				kb.Append(arg...)
			}
			assert.Equal(t, tt.result, kb.String())
		})
	}
}
