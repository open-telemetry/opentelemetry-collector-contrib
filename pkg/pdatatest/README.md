# pdatatest

This module provides a test helpers for comparing metric, log and traces. The main functions are: 
- `pmetrictest.CompareMetrics` 
- `plogtest.CompareLogs` 
- `ptracetest.CompareTraces` 

These functions compare the actual result with the expected result and return an error if they are not equal. 
The error contains a detailed description of the differences. The module also provides several options to customize 
the comparison by ignoring certain fields, attributes, or slices order. The module also provides helper functions 
for comparing other embedded pdata types such as `pmetric.ResourceMetrics`, `pmetric.ScopeMetrics`, `plog.LogRecord`,
`ptrace.Span`, etc.   

## Typical Usage

```go
func TestMetricsScraper(t *testing.T) {
	scraper := newScraper(componenttest.NewNopReceiverCreateSettings(), createDefaultConfig().(*Config))
	require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()))
	actualMetrics, err := require.NoError(t, scraper.scrape(context.Background()))
	require.NoError(t, err)

	expectedFile, err := readMetrics(filepath.Join("testdata", "expected.json"))
	require.NoError(err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreStartTimestamp(), 
		pmetrictest.IgnoreTimestamp()))
}
```

```go
func TestLogsReceiver(t *testing.T) {
	sink := &consumertest.LogsSink{}
	rcvr := newLogsReceiver(createDefaultConfig().(*Config), zap.NewNop(), sink)
	rcvr.client = defaultMockClient()
	require.NoError(t,  rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 2*time.Second, 10*time.Millisecond)
	err = rcvr.Shutdown(context.Background())
	require.NoError(t, err)
	actualLogs := sink.AllLogs()[0]

	expectedLogs, err := readLogs(filepath.Join("testdata", "logs", "expected.json"))
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareLogs(expectedLogs, actualLogs))
}
```

```go
func TestTraceProcessor(t *testing.T) {
	nextTrace := new(consumertest.TracesSink)
	tp, err := newTracesProcessor(NewFactory().CreateDefaultConfig(), nextTrace)
	traces := generateTraces()
	tp.ConsumeTraces(ctx, traces)
	actualTraces := nextTrace.AllTraces()[0]

	expectedTraces, err := readTraces(filepath.Join("testdata", "traces", "expected.json"))
	require.NoError(t, err)
	
	require.NoError(t, ptracetest.CompareTraces(expectedTraces, actualTraces, ptracetest.IgnoreStartTimestamp(), 
		ptracetest.IgnoreEndTimestamp()))
}
```