package resourceutil

import (
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"testing"
)

func TestHashEmptyResource(t *testing.T) {
	r := pcommon.NewResource()

	assert.EqualValues(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", CalculateResourceAttributesHash(r))
}

func TestHashSimpleResource(t *testing.T) {
	r := pcommon.NewResource()
	r.Attributes().PutStr("k1", "v1")
	r.Attributes().PutStr("k2", "v2")

	assert.EqualValues(t, "3590bbad8f8a328dbbd5d01c35d8a5fab92c3588cf7e468e995c31d45a51cbef", CalculateResourceAttributesHash(r))
}

func TestHashReorderedAttributes(t *testing.T) {
	r1 := pcommon.NewResource()
	r1.Attributes().PutStr("k1", "v1")
	r1.Attributes().PutStr("k2", "v2")

	r2 := pcommon.NewResource()
	r2.Attributes().PutStr("k2", "v2")
	r2.Attributes().PutStr("k1", "v1")

	assert.EqualValues(t, CalculateResourceAttributesHash(r1), CalculateResourceAttributesHash(r2))
}

func TestHashDifferentAttributeValues(t *testing.T) {
	r := pcommon.NewResource()
	r.Attributes().PutBool("k1", false)
	r.Attributes().PutDouble("k2", 1.0)
	r.Attributes().PutEmpty("k3")
	r.Attributes().PutEmptyBytes("k4")
	r.Attributes().PutEmptyMap("k5")
	r.Attributes().PutEmptySlice("k6")
	r.Attributes().PutInt("k7", 1)
	r.Attributes().PutStr("k8", "v8")

	assert.EqualValues(t, "46852adab1751045942d67dace7c88665ec0e68b7f4b81a33bb05e5b954a8e57", CalculateResourceAttributesHash(r))
}
