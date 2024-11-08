// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/docker"

import (
	"errors"

	"go.uber.org/zap"
	"k8s.io/kubernetes/pkg/util/parsers"
)

type ImageRef struct {
	Repository string
	Tag        string
	Digest     string
}

// ParseImageName extracts image repository and tag from a combined image reference
// e.g. example.com:5000/alpine/alpine:test --> `example.com:5000/alpine/alpine` and `test`
func ParseImageName(image string) (ImageRef, error) {
	if image == "" {
		return ImageRef{}, errors.New("empty image")
	}

	repository, tag, digest, err := parsers.ParseImageName(image)
	if err != nil {
		return ImageRef{}, errors.New("failed to parse image")
	}

	return ImageRef{
		Repository: repository,
		Tag:        tag,
		Digest:     digest,
	}, nil
}

func LogParseError(err error, image string, logger *zap.Logger) {
	logger.Debug(err.Error(),
		zap.String("image", image),
	)
}
