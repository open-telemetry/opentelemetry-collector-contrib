// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/docker"

import (
	"errors"

	"github.com/distribution/reference"
	"go.uber.org/zap"
)

type ImageRef struct {
	Repository      string
	Tag             string
	Digest          string
	DigestAlgorithm string
}

// ParseImageName extracts image repository, tag, digest and digest algorithm from a combined image reference
// e.g. example.com:5000/alpine/alpine:test --> `example.com:5000/alpine/alpine` and `test`
func ParseImageName(image string) (ImageRef, error) {
	ref, err := reference.Parse(image)
	if err != nil {
		return ImageRef{}, err
	}

	namedRef, imgOk := ref.(reference.Named)
	if !imgOk {
		return ImageRef{}, errors.New("cannot retrieve image name")
	}

	// Basic image reference
	img := ImageRef{
		Repository: namedRef.Name(),
		// If no tag provided in image reference - default to "latest"
		Tag: "latest",
	}

	if taggedRef, ok := namedRef.(reference.Tagged); ok {
		img.Tag = taggedRef.Tag()
	}

	digestedRef, digestOk := namedRef.(reference.Digested)
	// No image digest provided
	if !digestOk {
		return img, nil
	}

	// Image digest is incorrect or no digest
	if err := digestedRef.Digest().Validate(); err != nil {
		return img, nil
	}
	img.Digest = digestedRef.Digest().Encoded()
	img.DigestAlgorithm = digestedRef.Digest().Algorithm().String()

	return img, nil
}

// CanonicalImageRef tries to parse provided image reference
// and return canonical image reference instead, i.e. image reference
// that have qualified repository path and image name
// If it's not possible to normalize image reference - empty string and error returned
// Extracted from k8sattributesprocessor for future reuse
func CanonicalImageRef(image string) (string, error) {
	parsed, err := reference.ParseAnyReference(image)
	if err != nil {
		return "", err
	}

	switch parsed.(type) {
	case reference.Canonical:
		return parsed.String(), nil
	default:
		return "", errors.New("not canonical image reference")
	}
}

func LogParseError(err error, image string, logger *zap.Logger) {
	logger.Debug(err.Error(),
		zap.String("image", image),
	)
}
