# scraperinttest

This package is intended to be a temporary workspace for extracting common functionality
from scraper integration tests. Once stabilized, this code can likely move to `pdatatest`
or its sub-packages `pmetrictest`, etc.

If you're having issue with your `testcontainers.ContainerRequest` starting up, you can use the provided `PrintfLogConsumer` in a [`LogConsumerCfg`](https://pkg.go.dev/github.com/testcontainers/testcontainers-go#LogConsumerConfig) to display the startup logs

```golang
    scraperinttest.NewIntegrationTest(
            // ...
            scraperinttest.WithContainerRequest(
            testcontainers.ContainerRequest{
            // ...
                LogConsumerCfg: &testcontainers.LogConsumerConfig{
                    Consumers: []testcontainers.LogConsumer{
                        &scraperinttest.PrintfLogConsumer{},
                    },
                },
            // ...
            }
        )
    )
```
