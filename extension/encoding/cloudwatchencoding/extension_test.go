package cloudwatchencoding

import (
	"context"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"testing"
)

func TestExtension_Start_Shutdown(t *testing.T) {
	extension := &cloudwatchExtension{}

	err := extension.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	err = extension.Shutdown(context.Background())
	require.NoError(t, err)
}
