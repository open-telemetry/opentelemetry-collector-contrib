package starlarktransform

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type Config struct {

	// in-line code
	Code string `mapstructure:"code"`

	// source of startlark script
	// accepts both file path and url
	Script string `mapstructure:"script"`
}

func (c *Config) validate() error {
	if c.Code == "" && c.Script == "" {
		return errors.New("a value must be given for altest one, [code] or [script]")
	}

	return nil
}

func (c *Config) GetCode() (string, error) {

	if c.Code != "" {
		return c.Code, nil
	}

	var code string
	switch {
	case strings.HasPrefix(c.Script, "http"):
		resp, err := http.Get(c.Script)
		if err != nil {
			return "", fmt.Errorf("failed to fetch script from %s -%w", c.Script, err)
		}

		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return code, fmt.Errorf("failed to read http response body after fetching script from: %w", err)
		}

		code = string(b)

	default:
		f, err := os.Open(c.Script)
		if err != nil {
			return code, fmt.Errorf("failed to read script from path %s - %w", c.Script, err)
		}

		defer f.Close()

		b, err := io.ReadAll(f)
		if err != nil {
			return code, fmt.Errorf("failed while reading script file: %w", err)
		}

		code = string(b)
	}

	return code, nil
}
