# golden

The package golden provides utilities for reading and writing files with metrics, traces and logs in YAML format. 
The package is expected to be used with pkg/pdatatest module.

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

	expectedFile := filepath.Join("testdata", "scraper", "expected.yaml")

	golden.WriteMetrics(t, expectedFile, actualMetrics) // This line is temporary! TODO remove this!!

	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics))
}
```
