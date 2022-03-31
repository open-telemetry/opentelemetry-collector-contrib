// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package basicauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component/componenttest"
)

var (
	credentials = [][]string{
		{"htpasswd-md5", "$apr1$FVVioVP7$ZdIWPG1p4E/ErujO7kA2n0"},
		{"openssl-apr1", "$apr1$peiE49Vv$lo.z77Z.6.a.Lm7GMjzQh0"},
		{"openssl-md5", "$1$mvmz31IB$U9KpHBLegga2doA0e3s3N0"},
		{"htpasswd-sha", "{SHA}vFznddje0Ht4+pmO0FaxwrUKN/M="},
		{"htpasswd-bcrypt", "$2y$10$Q6GeMFPd0dAxhQULPDdAn.DFy6NDmLaU0A7e2XoJz7PFYAEADFKbC"},
		{"", "$2a$06$DCq7YPn5Rq63x1Lad4cll.TV4S6ytwfsfvkgY8jIucDrjc8deX1s."},
		{"", "$2a$08$HqWuK6/Ng6sg9gQzbLrgb.Tl.ZHfXLhvt/SgVyWhQqgqcZ7ZuUtye"},
		{"", "$2a$10$k1wbIrmNyFAPwPVPSVa/zecw2BCEnBwVS2GbrmgzxFUOqW9dk4TCW"},
		{"", "$2a$12$k42ZFHFWqBp3vWli.nIn8uYyIkbvYRvodzbfbK18SSsY.CsIQPlxO"},
		{"a", "$2a$06$m0CrhHm10qJ3lXRY.5zDGO3rS2KdeeWLuGmsfGlMfOxih58VYVfxe"},
		{"a", "$2a$08$cfcvVd2aQ8CMvoMpP2EBfeodLEkkFJ9umNEfPD18.hUF62qqlC/V."},
		{"a", "$2a$10$k87L/MF28Q673VKh8/cPi.SUl7MU/rWuSiIDDFayrKk/1tBsSQu4u"},
		{"a", "$2a$12$8NJH3LsPrANStV6XtBakCez0cKHXVxmvxIlcz785vxAIZrihHZpeS"},
		{"abc", "$2a$06$If6bvum7DFjUnE9p2uDeDu0YHzrHM6tf.iqN8.yx.jNN1ILEf7h0i"},
		{"abcdefghijklmnopqrstuvwxyz", "$2a$06$.rCVZVOThsIa97pEDOxvGuRRgzG64bvtJ0938xuqzv18d3ZpQhstC"},
		{"abcdefghijklmnopqrstuvwxyz", "$2a$08$aTsUwsyowQuzRrDqFflhgekJ8d9/7Z3GV3UcgvzQW3J5zMyrTvlz."},
		{"abcdefghijklmnopqrstuvwxyz", "$2a$10$fVH8e28OQRj9tqiDXs1e1uxpsjN0c7II7YPKXua2NAKYvM6iQk7dq"},
		{"abcdefghijklmnopqrstuvwxyz", "$2a$12$D4G5f18o7aMMfwasBL7GpuQWuP3pkrZrOAnqP.bmezbMng.QwJ/pG"},
		{"~!@#$%^&*()      ~!@#$%^&*()PNBFRD", "$2a$06$fPIsBO8qRqkjj273rfaOI.HtSV9jLDpTbZn782DC6/t7qT67P6FfO"},
		{"~!@#$%^&*()      ~!@#$%^&*()PNBFRD", "$2a$08$Eq2r4G/76Wv39MzSX262huzPz612MZiYHVUJe/OcOql2jo4.9UxTW"},
		{"~!@#$%^&*()      ~!@#$%^&*()PNBFRD", "$2a$10$LgfYWkbzEvQ4JakH7rOvHe0y8pHKF9OaFgwUZ2q7W2FFZmZzJYlfS"},
		{"~!@#$%^&*()      ~!@#$%^&*()PNBFRD", "$2a$12$WApznUOJfkEGSmYRfnkrPOr466oFDCaj4b6HY3EXGvfxm43seyhgC"},
		{"ππππππππ", "$2a$10$.TtQJ4Jr6isd4Hp.mVfZeuh6Gws4rOQ/vdBczhDx.19NFK0Y84Dle"},
	}
)

func TestBasicAuth_Valid(t *testing.T) {
	t.Parallel()
	f, err := ioutil.TempFile("", ".htpasswd")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	for _, c := range credentials {
		_, err = fmt.Fprintf(f, "%s:%s\n", c[0], c[1])
		require.NoError(t, err)
	}

	ctx := context.Background()

	ext, err := newServerAuthExtension(&Config{
		Htpasswd: &HtpasswdSettings{
			File: f.Name(),
		},
	})
	require.NoError(t, err)

	require.NoError(t, ext.Start(ctx, componenttest.NewNopHost()))

	for _, c := range credentials {
		t.Run(c[0], func(t *testing.T) {
			t.Parallel()
			auth := fmt.Sprintf("%s:%s", c[0], c[0])
			auth = base64.StdEncoding.EncodeToString([]byte(auth))

			authCtx, err := ext.Authenticate(ctx, map[string][]string{"authorization": {"Basic " + auth}})
			assert.NoError(t, err)
			cl := client.FromContext(authCtx)
			assert.Equal(t, c[0], cl.Auth.GetAttribute("username"))
			assert.Equal(t, auth, cl.Auth.GetAttribute("raw"))
		})
	}
}

func TestBasicAuth_InvalidCredentials(t *testing.T) {
	ext, err := newServerAuthExtension(&Config{
		Htpasswd: &HtpasswdSettings{
			Inline: "username:password",
		},
	})
	require.NoError(t, err)
	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))
	_, err = ext.Authenticate(context.Background(), map[string][]string{"authorization": {"Basic dXNlcm5hbWU6cGFzc3dvcmR4eHg="}})
	assert.Equal(t, errInvalidCredentials, err)
}

func TestBasicAuth_NoHeader(t *testing.T) {
	ext, err := newServerAuthExtension(&Config{
		Htpasswd: &HtpasswdSettings{
			Inline: "username:password",
		},
	})
	require.NoError(t, err)
	_, err = ext.Authenticate(context.Background(), map[string][]string{})
	assert.Equal(t, errNoAuth, err)
}

func TestBasicAuth_InvalidPrefix(t *testing.T) {
	ext, err := newServerAuthExtension(&Config{
		Htpasswd: &HtpasswdSettings{
			Inline: "username:password",
		},
	})
	require.NoError(t, err)
	_, err = ext.Authenticate(context.Background(), map[string][]string{"authorization": {"Bearer token"}})
	assert.Equal(t, errInvalidSchemePrefix, err)
}

func TestBasicAuth_NoFile(t *testing.T) {
	ext, err := newServerAuthExtension(&Config{
		Htpasswd: &HtpasswdSettings{
			File: "/non/existing/file",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, ext)

	require.Error(t, ext.Start(context.Background(), componenttest.NewNopHost()))
}

func TestBasicAuth_InvalidFormat(t *testing.T) {
	ext, err := newServerAuthExtension(&Config{
		Htpasswd: &HtpasswdSettings{
			Inline: "username:password",
		},
	})
	require.NoError(t, err)
	for _, auth := range [][]string{
		{"non decodable", "invalid"},
		{"missing separator", "aW52YWxpZAo="},
	} {
		t.Run(auth[0], func(t *testing.T) {
			_, err = ext.Authenticate(context.Background(), map[string][]string{"authorization": {"Basic " + auth[1]}})
			assert.Equal(t, errInvalidFormat, err)
		})
	}
}

func TestBasicAuth_HtpasswdInlinePrecedence(t *testing.T) {
	t.Parallel()
	f, err := ioutil.TempFile("", ".htpasswd")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	f.WriteString("username:fromfile")

	ext, err := newServerAuthExtension(&Config{
		Htpasswd: &HtpasswdSettings{
			File:   f.Name(),
			Inline: "username:frominline",
		},
	})
	require.NoError(t, err)
	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

	auth := base64.StdEncoding.EncodeToString([]byte("username:frominline"))

	_, err = ext.Authenticate(context.Background(), map[string][]string{"authorization": {"Basic " + auth}})
	assert.NoError(t, err)

	auth = base64.StdEncoding.EncodeToString([]byte("username:fromfile"))

	_, err = ext.Authenticate(context.Background(), map[string][]string{"authorization": {"Basic " + auth}})
	assert.Error(t, errInvalidCredentials, err)
}

func TestBasicAuth_SupportedHeaders(t *testing.T) {
	ext, err := newServerAuthExtension(&Config{
		Htpasswd: &HtpasswdSettings{
			Inline: "username:password",
		},
	})
	require.NoError(t, err)
	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

	auth := base64.StdEncoding.EncodeToString([]byte("username:password"))

	for _, k := range []string{
		"Authorization",
		"authorization",
		"aUtHoRiZaTiOn",
	} {
		_, err = ext.Authenticate(context.Background(), map[string][]string{k: {"Basic " + auth}})
		assert.NoError(t, err)
	}
}

func TestBasicAuth_ServerInvalid(t *testing.T) {
	_, err := newServerAuthExtension(&Config{
		Htpasswd: &HtpasswdSettings{},
	})
	assert.Error(t, err)
}

type mockRoundTripper struct{}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{StatusCode: http.StatusOK, Header: map[string][]string{}}
	for k, v := range req.Header {
		resp.Header[k] = v
	}
	return resp, nil
}

func TestBasicAuth_ClientValid(t *testing.T) {
	ext, err := newClientAuthExtension(&Config{
		ClientAuth: &ClientAuthSettings{
			Username: "username",
			Password: "password",
		},
	})
	require.NotNil(t, ext)
	require.NoError(t, err)

	require.Nil(t, ext.Start(context.Background(), componenttest.NewNopHost()))

	base := &mockRoundTripper{}
	c, err := ext.RoundTripper(base)
	require.NoError(t, err)
	require.NotNil(t, c)

	authCreds := base64.StdEncoding.EncodeToString([]byte("username:password"))
	orgHeaders := http.Header{
		"Test-Header-1": []string{"test-value-1"},
	}
	expectedHeaders := http.Header{
		"Test-Header-1": []string{"test-value-1"},
		"Authorization": {fmt.Sprintf("Basic %s", authCreds)},
	}

	resp, err := c.RoundTrip(&http.Request{Header: orgHeaders})
	assert.NoError(t, err)
	assert.Equal(t, expectedHeaders, resp.Header)
	assert.Nil(t, ext.Shutdown(context.Background()))
}

func TestBasicAuth_ClientInValid(t *testing.T) {
	t.Run("no username", func(t *testing.T) {
		_, err := newClientAuthExtension(&Config{
			ClientAuth: &ClientAuthSettings{
				Username: "",
			},
		})
		assert.Error(t, err)
	})

	t.Run("invalid username format", func(t *testing.T) {
		ext, err := newClientAuthExtension(&Config{
			ClientAuth: &ClientAuthSettings{
				Username: "user:name",
				Password: "password",
			},
		})
		require.NotNil(t, ext)
		require.NoError(t, err)

		require.Nil(t, ext.Start(context.Background(), componenttest.NewNopHost()))

		base := &mockRoundTripper{}
		_, err = ext.RoundTripper(base)
		assert.Error(t, err)
	})

}
