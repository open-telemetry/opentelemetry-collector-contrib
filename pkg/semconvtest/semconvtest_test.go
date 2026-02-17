package semconvtest_test

import (
	"os"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/semconvtest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestWeaver(t *testing.T) {
	outputDir := t.TempDir()

	opts := &semconvtest.WeaverOptions{
		OutputDir: outputDir,
	}

	weaver, err := semconvtest.NewWeaverContext(t.Context(), opts)
	require.NoError(t, err)

	logs := plog.NewLogs()
	res := logs.ResourceLogs().AppendEmpty()
	res.Resource().Attributes().PutStr("something", "value")
	scope := res.ScopeLogs().AppendEmpty()
	record := scope.LogRecords().AppendEmpty()
	record.Body().SetStr("hi I am a log")

	time.Sleep(10 * time.Second)

	err = weaver.TestLogs(logs)
	require.NoError(t, err)

	err = weaver.Stop()
	require.NoError(t, err)

	outputFile, err := weaver.WaitForOutput(30 * time.Second)
	require.NoError(t, err)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	require.Contains(t, string(content), "something")
	require.Contains(t, string(content), "missing_attribute")
}
