package asapclientauthextension

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"path"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)
	factory := NewFactory()
	factories.Extensions[typeStr] = factory

	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NotNil(t, cfg)
	assert.NoError(t, cfg.Validate())

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Ttl = 60
	expected.Audience = []string{"test_service1", "test_service2"}
	expected.Issuer = "test_issuer"
	expected.KeyId = "test_issuer/test_kid"
	expected.PrivateKey = "data:application/pkcs8;kid=test_issuer%2Ftest_keyid;base64,MIIEpQIBAAKCAQEA54wghrw7+3h01MxiXACHFTXa2f4DxPuSljtQaSFs8rxDIemHrHGHrZVBJA5KO9xWo1O1RBFdFhwOOMG0xMeI5hfEXQF8Uz4nn6LJT27NsIPNrQFonChMIYVEA17Gor4N9zF5FIslkyeTKGvzmhs+N90SOpaAYS3vBK7Mx2i7/yB/SG1w/4s/81cET2Z/VB3wZtJjq9Crg+cv515tD/amQamMT1Eh60dRLvzcBZ1aOiztZGlPavbiz2tng19CyBXLwrpbFRgqBX+tPKXbgMXrTopc7YfKjFrtmhR0xaFb0DMrndwp2bMNd2/4IlRwwo7d33ePq0Uacdo+H7Oz5nZWYwIDAQABAoIBAQCCwXXowEmbI5XOSbDNxZqC1svEyJY2Wd6Yqdwp0i9lD/1VHDx6nA4Db0K+6rbvAOmICBBX5PpNLwC0+mZrrUZYsVk5MEqV84aKtnG6QpczM+sk5KO/c14ym8Ahqxa+9laKnkyC1mUcqX+HlxaUkwfaoiPWJAFRX5AXc+K+RR3M3ukHBxOCDt8Omn81QMVYCCT9oWz9mIrUwiIPEHpt1jFU0jIQXMMkEaIdsJ2lj6rwhkj7wdxMeXgwTfX816UCWCSiMlx2QVzLhAqNgCYqEyW9RzgxLSFUUyolPh8gRfK1jBX2XgF+kfdLK1rWrlOHp8e7tZs+8VkCofPCHsRSbuepAoGBAPr+WXYPnrNswjmjEtnJ5w/QPHI83mc85rPm8r59TfKafh/7m/M8tIgwdvdqc+in4MOEA3bcQBzRQgM6VybaFpy5hAzP7v9gZZNpTQWw4kI7G4DWr5cv2wepKNExY2o0fWoJXzN/PUdqPEoiIseKeJrttbekI9AZZO8MrPD7x6mNAoGBAOwqermjN1ZYvKLjDlpBCluZV8J/8frrAL3Yb29AFWlx3mE4kNJmdnb14p+FvnOHGNIBsQr2CAoYNgG61/BBGzny89cjItnN1O9ZmUrWGUxJABrHBHQfz69uMlnxPmL4rdbePzE+I0B/4ppLFfS92a9ARbwwaHLG5GWhhJGr6euvAoGBAJu5NR4XwNoHj1WdRLPVHcPk6avi8gXRdj2F+3OOYM81ZS1IuVAniMa6cwU8id9+UOhdPpz/N9PpTPCdwLa9NqxUOYaNd/YAA+V6vqvaO/bln0HHcTf3HAjbvhRUdR7OpEUmvWdA+W8WjYNdPIDa+8r70vO2JfYV21apYZ8+R3l9AoGAYZdkQ5Yo5euheAYwBifead/CHkPU8QVvtwPbeLOYpYxCgxZm8isZRStyzMzt2Lu5C/9a89abl+BNYQWe+k9NOvMkxIBmhG7EUWxLJlu29IkuG+Kl+n6yyiHVeMlyKF/vJl2M64Jr+tleALiKiCpz7DG5H305jESYWU8Xg0LxVU0CgYEAgFtOY/EwFOe4ivDPqZ81GBLpTy9eJ8fOdGqHEpR8OGVb0g4dGhx1PBWUYyo/Z+fa9V9O315owmUwfMtbrS03be9cYGUxzEIaT7S9cMtEeVi8PAJWVWTP0isHoQaRaCXz92FHNzlVB6tAOwFaNNlA64KYJ4wOKQcFbjC9KjdFMxU="

	ext := cfg.Extensions[config.NewComponentID(typeStr)]
	assert.Equal(t, expected, ext)
}
