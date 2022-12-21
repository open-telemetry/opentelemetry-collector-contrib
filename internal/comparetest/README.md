# comparetest

This module provides a mechanism for capturing and comparing expected metric results.

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

## Generating an expected result file

The easiest way to capture the expected result in a file is `golden.WriteMetrics`.

When writing a new test:
1. Write the test as if the expected file exists.
2. Follow the steps below for updating an existing test.

When updating an existing test:
1. Add a call to `golden.WriteMetrics` in the appropriate place.
2. Run the test once.
3. Remove the call to `golden.WriteMetrics`.

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

  golden.WriteMetrics(expectedFile, actualMetrics)   // This line is temporary! TODO remove this!!

  expectedMetrics, err := golden.ReadMetrics(expectedFile)
  require.NoError(t, err)

  require.NoError(t, comparetest.CompareMetrics(expectedMetrics, actualMetrics))
}
```
