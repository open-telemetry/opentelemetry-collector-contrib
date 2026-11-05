// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.uber.org/zap"
)

func TestNewUploadManager(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		conf   *Config
		errVal string
	}{
		{
			name: "valid configuration",
			conf: &Config{
				S3Uploader: S3UploaderConfig{
					Region:              "local",
					S3Bucket:            "my-awesome-bucket",
					S3Prefix:            "opentelemetry",
					S3PartitionFormat:   "year=%Y/month=%m/day=%d/hour=%H",
					S3PartitionTimezone: "Europe/London",
					FilePrefix:          "ingested-data-",
					Endpoint:            "localhost",
					RoleArn:             "arn:aws:iam::123456789012:my-awesome-user",
					S3ForcePathStyle:    true,
					DisableSSL:          true,
					Compression:         configcompression.TypeGzip,
				},
			},
			errVal: "",
		},
		{
			name: "valid configuration with static credentials",
			conf: &Config{
				S3Uploader: S3UploaderConfig{
					Region:              "local",
					S3Bucket:            "my-awesome-bucket",
					S3Prefix:            "opentelemetry",
					S3PartitionFormat:   "year=%Y/month=%m/day=%d/hour=%H",
					S3PartitionTimezone: "Europe/London",
					FilePrefix:          "ingested-data-",
					Endpoint:            "localhost",
					AccessKey:           "access",
					SecretKey:           "secret",
					S3ForcePathStyle:    true,
					DisableSSL:          true,
					Compression:         configcompression.TypeGzip,
				},
			},
			errVal: "",
		},
		{
			name: "invalid timezone configuration",
			conf: &Config{
				S3Uploader: S3UploaderConfig{
					Region:              "local",
					S3Bucket:            "my-awesome-bucket",
					S3Prefix:            "opentelemetry",
					S3PartitionFormat:   "year=%Y/month=%m/day=%d/hour=%H",
					S3PartitionTimezone: "non-existing timezone",
					FilePrefix:          "ingested-data-",
					Endpoint:            "localhost",
					RoleArn:             "arn:aws:iam::123456789012:my-awesome-user",
					S3ForcePathStyle:    true,
					DisableSSL:          true,
					Compression:         configcompression.TypeGzip,
				},
			},
			errVal: "invalid S3 partition timezone: unknown time zone non-existing timezone",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sm, err := newUploadManager(
				t.Context(),
				tc.conf,
				zap.NewNop(),
				"metrics",
				"otlp",
				false,
			)

			if tc.errVal != "" {
				assert.Nil(t, sm, "Must not have a valid s3 upload manager")
				assert.EqualError(t, err, tc.errVal, "Must match the expected error")
			} else {
				assert.NotNil(t, sm, "Must have a valid manager")
				assert.NoError(t, err, "Must not error when creating client")
			}
		})
	}
}

func TestNewUploadManager_RoleArnCredentialsCache(t *testing.T) {
	for _, tt := range []struct {
		name         string
		lifetime     time.Duration
		wantSTSCalls int32
		wantKey      string
	}{
		{name: "reuse valid credentials", lifetime: 5 * time.Minute, wantSTSCalls: 1, wantKey: "AKID1"},
		{name: "refresh expired credentials", lifetime: -time.Minute, wantSTSCalls: 2, wantKey: "AKID2"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Given an assumed role backed by a local endpoint serving both STS and S3.
			var stsCalls atomic.Int32
			var lastS3Key atomic.Value
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodPost:
					if err := r.ParseForm(); err != nil {
						t.Error(err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					if r.Form.Get("Action") != "AssumeRole" {
						t.Errorf("unexpected STS action: %s", r.Form.Get("Action"))
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					call := stsCalls.Add(1)
					lifetime := tt.lifetime
					if call > 1 {
						lifetime = time.Hour
					}
					w.Header().Set("Content-Type", "text/xml")
					_, err := fmt.Fprintf(w, `<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><AssumeRoleResult><Credentials><AccessKeyId>AKID%d</AccessKeyId><SecretAccessKey>SECRET</SecretAccessKey><SessionToken>TOKEN</SessionToken><Expiration>%s</Expiration></Credentials></AssumeRoleResult></AssumeRoleResponse>`, call, time.Now().Add(lifetime).UTC().Format(time.RFC3339))
					if err != nil {
						t.Error(err)
					}
				case http.MethodPut:
					_, _ = io.Copy(io.Discard, r.Body)
					// The SigV4 header is "... Credential=<access key>/<date>/...".
					_, credential, _ := strings.Cut(r.Header.Get("Authorization"), "Credential=")
					accessKey, _, _ := strings.Cut(credential, "/")
					lastS3Key.Store(accessKey)
					w.WriteHeader(http.StatusOK)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			t.Cleanup(server.Close)

			dir := t.TempDir()
			for _, name := range []string{"AWS_ACCESS_KEY", "AWS_SECRET_KEY", "AWS_SESSION_TOKEN", "AWS_PROFILE", "AWS_DEFAULT_PROFILE"} {
				t.Setenv(name, "")
			}
			t.Setenv("AWS_ACCESS_KEY_ID", "base-key")
			t.Setenv("AWS_SECRET_ACCESS_KEY", "base-secret")
			t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
			t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
			t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
			t.Setenv("AWS_ENDPOINT_URL_STS", server.URL)
			t.Setenv("AWS_IGNORE_CONFIGURED_ENDPOINT_URLS", "false")

			conf := createDefaultConfig().(*Config)
			conf.S3Uploader.S3Bucket = "my-awesome-bucket"
			conf.S3Uploader.Endpoint = server.URL
			conf.S3Uploader.RoleArn = "arn:aws:iam::123456789012:role/test"
			conf.S3Uploader.S3ForcePathStyle = true
			conf.S3Uploader.DisableSSL = true

			sm, err := newUploadManager(t.Context(), conf, zap.NewNop(), "logs", "otlp", false)
			require.NoError(t, err)
			require.NoError(t, sm.Upload(t.Context(), []byte("data"), nil))

			// When another upload is made without waiting for expiration.
			require.NoError(t, sm.Upload(t.Context(), []byte("data"), nil))

			// Then only expired credentials are refreshed.
			assert.Equal(t, tt.wantKey, lastS3Key.Load())
			assert.Equal(t, tt.wantSTSCalls, stsCalls.Load())
		})
	}
}
