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

package s3provider

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/confmap"
	"gopkg.in/yaml.v2"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	schemeName = "s3"
)

type provider struct {
	client *s3.Client
}

// New returns a new confmap.Provider that reads the configuration from a file.
//
// This Provider supports "s3" scheme, and can be called with a "uri" that follows:
//   s3-uri : s3://[BUCKET].s3.[REGION].amazonaws.com/[KEY]
//
// One example for s3-uri be like: s3://DOC-EXAMPLE-BUCKET.s3.us-west-2.amazonaws.com/photos/puppy.jpg
//
// Examples:
// `s3://DOC-EXAMPLE-BUCKET.s3.us-west-2.amazonaws.com/photos/puppy.jpg` - (unix, windows)
func New() confmap.Provider {
	// initialize the client
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	return &provider{client: s3.NewFromConfig(cfg)}
}

func (fmp *provider) Retrieve(ctx context.Context, uri string, _ confmap.WatcherFunc) (confmap.Retrieved, error) {
	if !strings.HasPrefix(uri, schemeName+":") {
		return confmap.Retrieved{}, fmt.Errorf("%q uri is not supported by %q provider", uri, schemeName)
	}

	// Split the uri and get [BUCKET], [REGION], [KEY]
	bucket, region, key, err := s3URISplit(uri)
	if err != nil {
		return confmap.Retrieved{}, fmt.Errorf("%q uri is not valid s3-url", uri)
	}

	// s3 downloading
	resp, err := fmp.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, func(o *s3.Options) {
		o.Region = region
	})
	if err != nil {
		return confmap.Retrieved{}, fmt.Errorf("file in S3 failed to fetch : uri %q", uri)
	}

	// create a buffer and read content from the response body
	buffer := make([]byte, int(resp.ContentLength))
	defer resp.Body.Close()
	_, err = resp.Body.Read(buffer)
	if err != io.EOF && err != nil {
		return confmap.Retrieved{}, fmt.Errorf("failed to read content from the downloaded config file via uri %q", uri)
	}

	// unmarshalling the yaml to map[string]interface{}, then construct a Retrieved object
	var rawConf map[string]interface{}
	if err := yaml.Unmarshal(buffer, &rawConf); err != nil {
		return confmap.Retrieved{}, err
	}
	return confmap.NewRetrieved(rawConf)
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
//		-  [BUCKET] : The name of a bucket in Amazon S3.
//		-  [REGION] : Where are servers from, e.g. us-west-2.
//		-  [KEY]    : The key exists in a given bucket, can be used to retrieve a file.
func s3URISplit(uri string) (string, string, string, error) {
	// check whether the pattern of s3-uri is correct
	matched, err := regexp.MatchString(`s3:\/\/(.*)\.s3\.(.*).amazonaws\.com\/(.*)`, uri)
	if !matched || err != nil {
		return "", "", "", fmt.Errorf("invalid s3-uri using a wrong pattern")
	}
	// parse the uri as [scheme:][//[userinfo@]host][/]path[?query][#fragment], then extract components from
	u, err := url.Parse(uri)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to change the s3-uri to url.URL")
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
