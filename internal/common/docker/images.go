// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/docker"

import (
	"errors"
	"regexp"

	"go.uber.org/zap"
)

var (
	extractImageRegexp = regexp.MustCompile(`^(?P<repository>([^/\s]+/)?([^:\s]+))(:(?P<tag>[^@\s]+))?(@sha256:(?P<sha256>\d+))?$`)
)

type ImageRef struct {
	Repository string
	Tag        string
	SHA256     string
}

// ParseImageName extracts image repository and tag from a combined image reference
// e.g. example.com:5000/alpine/alpine:test --> `example.com:5000/alpine/alpine` and `test`
func ParseImageName(image string) (ImageRef, error) {
	if image == "" {
		return ImageRef{}, errors.New("empty image")
	}

	match := extractImageRegexp.FindStringSubmatch(image)
	if len(match) == 0 {
		return ImageRef{}, errors.New("failed to match regex against image")
	}

	tag := "latest"
	if foundTag := match[extractImageRegexp.SubexpIndex("tag")]; foundTag != "" {
		tag = foundTag
	}

	repository := match[extractImageRegexp.SubexpIndex("repository")]

	hash := match[extractImageRegexp.SubexpIndex("sha256")]

	return ImageRef{
		Repository: repository,
		Tag:        tag,
		SHA256:     hash,
	}, nil
}

func LogParseError(err error, image string, logger *zap.Logger) {
	logger.Debug(err.Error(),
		zap.String("image", image),
	)
}
