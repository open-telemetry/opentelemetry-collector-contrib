package subst

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubst(t *testing.T) {
	t.Parallel()

	vars := map[string]string{
		"attr.Val_1": "v1",
		"attr.Val_2": "v2",
	}

	s := Subst("foo/${attr.Val_1}/bar/${attr.Val_2}/baz", func(key string) string {
		return vars[key]
	})
	assert.Equal(t, "foo/v1/bar/v2/baz", s)
}

func TestVars(t *testing.T) {
	v := Vars("foo/${var1}/bar/${var2}/baz/${var1}")
	assert.Equal(t, []string{"var1", "var2"}, v)
}
