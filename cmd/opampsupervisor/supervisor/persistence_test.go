package supervisor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateNewULID(t *testing.T) {
	// Test generating a new ULID twice returns 2 different results
	id1, err := generateNewULID()
	require.NoError(t, err)

	id2, err := generateNewULID()
	require.NoError(t, err)

	require.NotEqual(t, id1, id2)
}
