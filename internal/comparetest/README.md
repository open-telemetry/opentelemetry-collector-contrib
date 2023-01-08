# comparetest

This module provides a mechanism for capturing and comparing expected metric and log results.

## Typical Usage

A scraper test typically looks something like this:

```go
func TestScraper(t *testing.T) {
  cfg := createDefaultConfig().(*Config)
  require.NoError(t, component.ValidateConfig(cfg))

  scraper := newScraper(componenttest.NewNopReceiverCreateSettings(), cfg)

  err := scraper.start(context.Background(), componenttest.NewNopHost())
  require.NoError(t, err)

  actualMetrics, err := scraper.scrape(context.Background())
  require.NoError(t, err)

  expectedFile := filepath.Join("testdata", "scraper", "expected.json")
  expectedMetrics, err := golden.ReadMetrics(expectedFile)
  require.NoError(t, err)

  require.NoError(t, comparetest.CompareMetrics(expectedMetrics, actualMetrics))
}
```

```go
func TestLogsSink(t *testing.T) {
  cfg := createDefaultConfig().(*Config)
  require.NoError(t, component.ValidateConfig(cfg))

  sink := &consumertest.LogsSink{}
  alertRcvr := newLogsReceiver(cfg, zap.NewNop(), sink)
  alertRcvr.client = defaultMockClient()

  err := alertRcvr.Start(context.Background(), componenttest.NewNopHost())
  require.NoError(t, err)

  require.Eventually(t, func() bool {
    return sink.LogRecordCount() > 0
  }, 2*time.Second, 10*time.Millisecond)

  err = alertRcvr.Shutdown(context.Background())
  require.NoError(t, err)

  logs := sink.AllLogs()[0]
  expected, err := readLogs(filepath.Join("testdata", "logs", "expected.json"))
  require.NoError(t, err)
  require.NoError(t, comparetest.CompareLogs(expected, logs))
}
```

## Generating an expected result file

The easiest way to capture the expected result in a file is `golden.WriteMetrics` or `golden.WriteLogs`.

When writing a new test:
1. Write the test as if the expected file exists.
2. Follow the steps below for updating an existing test.

When updating an existing test:
1. Add a call to `golden.WriteMetrics` or `golden.WriteLogs` or in the appropriate place.
2. Run the test once.
3. Remove the call to `golden.WriteMetrics` or `golden.WriteLogs`.

NOTE: `golden.WriteMetrics` will always mark the test as failed. This behavior is
necessary to ensure the function is removed after the golden file is written.

```go
func TestScraper(t *testing.T) {
  cfg := createDefaultConfig().(*Config)
  require.NoError(t, component.ValidateConfig(cfg))

  scraper := newScraper(componenttest.NewNopReceiverCreateSettings(), cfg)

  err := scraper.start(context.Background(), componenttest.NewNopHost())
  require.NoError(t, err)

  actualMetrics, err := scraper.scrape(context.Background())
  require.NoError(t, err)

  expectedFile := filepath.Join("testdata", "scraper", "expected.json")

  golden.WriteMetrics(t, expectedFile, actualMetrics) // This line is temporary! TODO remove this!!

  expectedMetrics, err := golden.ReadMetrics(expectedFile)
  require.NoError(t, err)

  require.NoError(t, comparetest.CompareMetrics(expectedMetrics, actualMetrics))
}
```
