// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

package s3provider // import "github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/s3provider"

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.opentelemetry.io/collector/confmap"
	"gopkg.in/yaml.v3"
)

const (
	schemeName = "s3"
	// Pattern for an AWS S3 virtual-hosted-style uri
	s3AWSPattern = `^s3:\/\/([a-z0-9\.\-]{3,63})\.s3(?:-fips)?(?:\.dualstack)?\.([a-z0-9\-]+)\.(api\.amazonwebservices\.com\.cn|api\.amazonwebservices\.eu|api\.cloud-aws\.adc-e\.uk|api\.aws\.hci\.ic\.gov|amazonaws\.com\.cn|cloud\.adc-e\.uk|api\.aws\.scloud|api\.aws\.ic\.gov|csp\.hci\.ic\.gov|sc2s\.sgov\.gov|amazonaws\.com|amazonaws\.eu|c2s\.ic\.gov|api\.aws)\/.`
)

var s3AWSRegexp = regexp.MustCompile(s3AWSPattern)

type s3Client interface {
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

type provider struct {
	client s3Client
}

// NewFactory returns a new confmap.ProviderFactory that creates a confmap.Provider
// which reads configuration from a file obtained from an s3 bucket.
//
// This Provider supports the "s3" scheme with two URI formats:
//
// AWS virtual-hosted-style (standard Amazon S3):
//
//	s3://[BUCKET].s3.[REGION].amazonaws.com/[KEY]
//	s3://doc-example-bucket.s3.us-west-2.amazonaws.com/config.yaml
//
// S3-compatible path-style (for MinIO, DigitalOcean Spaces, and other S3-compatible services):
//
//	s3://[ENDPOINT_HOST]/[BUCKET]/[KEY]?region=[REGION]
//	s3://minio.example.com/my-bucket/config.yaml
func NewFactory() confmap.ProviderFactory {
	return confmap.NewProviderFactory(newWithSettings)
}

func newWithSettings(confmap.ProviderSettings) confmap.Provider {
	return &provider{client: nil}
}

func (fmp *provider) Retrieve(ctx context.Context, uri string, _ confmap.WatcherFunc) (*confmap.Retrieved, error) {
	// Split the uri and get [BUCKET], [REGION], [KEY], and optional [ENDPOINT]
	bucket, region, key, endpoint, err := s3URISplit(uri)
	if err != nil {
		return nil, fmt.Errorf("%q uri is not valid s3-url: %w", uri, err)
	}

	if fmp.client == nil {
		cfg, loadErr := config.LoadDefaultConfig(context.Background())
		if loadErr != nil {
			return nil, fmt.Errorf("failed to load configurations to initialize an AWS SDK client, error: %w", loadErr)
		}
		var clientOpts []func(*s3.Options)
		if endpoint != "" {
			clientOpts = append(clientOpts, func(o *s3.Options) {
				o.BaseEndpoint = aws.String(endpoint)
				o.UsePathStyle = true
			})
		}
		fmp.client = s3.NewFromConfig(cfg, clientOpts...)
	}

	// s3 downloading
	resp, err := fmp.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, func(o *s3.Options) {
		o.Region = region
	})
	if err != nil {
		return nil, fmt.Errorf("file in S3 failed to fetch uri %q: %w", uri, err)
	}

	// read config from response body
	dec := yaml.NewDecoder(resp.Body)
	defer resp.Body.Close()
	var conf map[string]any
	err = dec.Decode(&conf)
	if err != nil {
		return nil, err
	}
	return confmap.NewRetrieved(conf)
}

func (*provider) Scheme() string {
	return schemeName
}

func (*provider) Shutdown(context.Context) error {
	return nil
}

// s3URISplit splits the s3 uri and returns [BUCKET], [REGION], [KEY], and optional [ENDPOINT].
//
// Two URI formats are supported:
//
//  1. AWS virtual-hosted-style (host contains "amazonaws.com"):
//     s3://[BUCKET].s3.[REGION].amazonaws.com/[KEY]
//
//  2. S3-compatible path-style (any other host):
//     s3://[ENDPOINT_HOST]/[BUCKET]/[KEY]?region=[REGION]
//     The host is used as the endpoint; region is optional.
func s3URISplit(uri string) (bucket, region, key, endpoint string, err error) {
	// parse the uri as [scheme:][//[userinfo@]host][/]path[?query][#fragment]
	u, err := url.Parse(uri)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to parse s3 uri: %w", err)
	}

	if u.Scheme != schemeName {
		return "", "", "", "", fmt.Errorf("uri scheme must be %q, got %q", schemeName, u.Scheme)
	}

	if matches := s3AWSRegexp.FindStringSubmatch(uri); matches != nil {
		bucket, region = matches[1], matches[2]
		key = strings.TrimPrefix(u.Path, "/")
		return bucket, region, key, "", nil
	}

	// S3-compatible path-style: s3://endpoint-host/bucket/key?region=...
	// The host is the endpoint; the path provides bucket and key.
	parts := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", "", errors.New("invalid s3-uri: for S3-compatible services use s3://[ENDPOINT_HOST]/[BUCKET]/[KEY]")
	}
	bucket = parts[0]
	key = parts[1]
	q := u.Query()
	region = q.Get("region")
	scheme := "https"
	if q.Get("insecure") == "true" {
		scheme = "http"
	}
	endpoint = scheme + "://" + u.Host

	return bucket, region, key, endpoint, nil
}
