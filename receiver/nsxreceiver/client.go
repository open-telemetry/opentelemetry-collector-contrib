package nsxreceiver

import (
	"context"
	"fmt"

	nsxt "github.com/vmware/go-vmware-nsxt"
)

type client struct {
	nsxtConf *nsxt.Configuration
	driver   *nsxt.APIClient
}

const defaultMaxRetries = 3
const defaultRetryMinDelay = 5
const defaultRetryMaxDelay = 10

func newClient(c *Config) *client {
	retriesConfiguration := nsxt.ClientRetriesConfiguration{
		MaxRetries:    defaultMaxRetries,
		RetryMinDelay: defaultRetryMinDelay,
		RetryMaxDelay: defaultMaxRetries,
	}
	return &client{
		nsxtConf: &nsxt.Configuration{
			BasePath:             "/api/v1",
			Host:                 c.Host,
			UserName:             c.Username,
			Password:             c.Password,
			Insecure:             c.Insecure,
			CAFile:               c.CAFile,
			RetriesConfiguration: retriesConfiguration,
		},
	}
}

func (c *client) Connect(ctx context.Context) error {
	driver, err := nsxt.NewAPIClient(c.nsxtConf)
	if err != nil {
		return fmt.Errorf("unable to connect to NSXT API: %w", err)
	}
	c.driver = driver
	return nil
}

func (c *client) Disconnect(ctx context.Context) error {
	if c.driver != nil {
	}
}
