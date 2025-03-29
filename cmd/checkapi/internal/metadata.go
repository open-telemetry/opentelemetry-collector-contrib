package internal

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type status struct {
	Class string `yaml:"class"`
}

type metadata struct {
	Status status `yaml:"status"`
}

func ReadComponentType(folder string) (string, error) {
	var componentType string
	if _, err := os.Stat(filepath.Join(folder, "metadata.yaml")); errors.Is(err, os.ErrNotExist) {
		componentType = "pkg"
	} else {
		m, err := os.ReadFile(filepath.Join(folder, "metadata.yaml"))
		if err != nil {
			return "", err
		}
		var componentInfo metadata
		if err = yaml.Unmarshal(m, &componentInfo); err != nil {
			return "", err
		}
		componentType = componentInfo.Status.Class
	}
	return componentType, nil
}
