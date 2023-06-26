package simpleconnector

import "fmt"

// Config for the connector
type Config struct {
	count int
}

// validates any configuration settings put onto the connector via the config file
func (c *Config) Validate() error {
	if c.count != 0 {
		return fmt.Errorf("We should start counting from 0")
	}
	return nil
}
