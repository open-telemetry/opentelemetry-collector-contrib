// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
