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

package s3mapprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/s3mapprovider"

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.opentelemetry.io/collector/confmap"
	"gopkg.in/yaml.v2"
)

const (
	schemeName = "s3"
)

type s3Client interface {
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

type provider struct {
	client s3Client
}

// New returns a new confmap.Provider that reads the configuration from a file.
//
// This Provider supports "s3" scheme, and can be called with a "uri" that follows:
//
//	s3-uri : s3://[BUCKET].s3.[REGION].amazonaws.com/[KEY]
//
// One example for s3-uri be like: s3://DOC-EXAMPLE-BUCKET.s3.us-west-2.amazonaws.com/photos/puppy.jpg
//
// Examples:
// `s3://DOC-EXAMPLE-BUCKET.s3.us-west-2.amazonaws.com/photos/puppy.jpg` - (unix, windows)
func New() confmap.Provider {
	return &provider{client: nil}
}

func (fmp *provider) Retrieve(ctx context.Context, uri string, _ confmap.WatcherFunc) (*confmap.Retrieved, error) {
	if !strings.HasPrefix(uri, schemeName+":") {
		return nil, fmt.Errorf("%q uri is not supported by %q provider", uri, schemeName)
	}

	// initialize the s3 client in the first call of Retrieve
	if fmp.client == nil {
		cfg, err := config.LoadDefaultConfig(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to load configurations to initialize an AWS SDK client, error: %w", err)
		}
		fmp.client = s3.NewFromConfig(cfg)
	}

	// Split the uri and get [BUCKET], [REGION], [KEY]
	bucket, region, key, err := s3URISplit(uri)
	if err != nil {
		return nil, fmt.Errorf("%q uri is not valid s3-url: %w", uri, err)
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
	var conf map[string]interface{}
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

// S3URISplit splits the s3 uri and get the [BUCKET], [REGION], [KEY] in it
// INPUT : s3 uri (like s3://[BUCKET].s3.[REGION].amazonaws.com/[KEY])
// OUTPUT :
//   - [BUCKET] : The name of a bucket in Amazon S3.
//   - [REGION] : Where are servers from, e.g. us-west-2.
//   - [KEY]    : The key exists in a given bucket, can be used to retrieve a file.
func s3URISplit(uri string) (string, string, string, error) {
	// check whether the pattern of s3-uri is correct
	matched, err := regexp.MatchString(`s3:\/\/(.*)\.s3\.(.*).amazonaws\.com\/(.*)`, uri)
	if !matched || err != nil {
		return "", "", "", fmt.Errorf("invalid s3-uri using a wrong pattern")
	}
	// parse the uri as [scheme:][//[userinfo@]host][/]path[?query][#fragment], then extract components from
	u, err := url.Parse(uri)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to change the s3-uri to url.URL: %w", err)
	}
	// extract components
	key := strings.TrimPrefix(u.Path, "/")
	host := u.Host
	hostSplitted := strings.Split(host, ".")
	if len(hostSplitted) < 5 {
		return "", "", "", fmt.Errorf("invalid host in the s3-uri")
	}
	bucket := hostSplitted[0]
	region := hostSplitted[2]
	// check empty fields
	if bucket == "" || region == "" || key == "" {
		return "", "", "", fmt.Errorf("invalid s3-uri with empty fields")
	}
	return bucket, region, key, nil
}
