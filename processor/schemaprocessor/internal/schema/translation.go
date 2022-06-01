package schema

import (
	"sync"

	"go.opentelemetry.io/otel/schema/v1.0/ast"
)

type translation struct {
	rw sync.RWMutex

	min, max *Version
}

func newTranslation(max *Version) *translation {
	return &translation{
		min: &Version{0, 0, 0},
		max: &Version{max.Major, max.Minor, max.Patch},
	}
}

var _ Schema = (*translation)(nil)

func (t *translation) SupportedVersion(version *Version) bool {
	t.rw.RLock()
	defer t.rw.RUnlock()

	// This implementation will change to check if the version is known
	// for the time being
	return version.Compare(t.min) >= 0 && version.Compare(t.max) <= 0
}

func (t *translation) Merge(content *ast.Schema) error {
	// Implementation to come
	return nil
}
