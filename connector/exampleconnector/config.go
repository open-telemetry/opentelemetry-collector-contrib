package exampleconnector

import (
	"fmt"
)

type Config struct {
	Count int `mapstructure:"count"`
}

func (c *Config) Validate() error {
	if c.Count != 0 {
		return fmt.Errorf("our count connector should start counting from zero")
	}
	return nil
}
