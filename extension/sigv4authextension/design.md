# **Sigv4 Authenticator Extension Design Document**


This document outlines a proposed implementation of a Sigv4 authenticator extension for the OpenTelemetry (OTEL) Collector. The design is based on the OpenTelemetry Collector [configauth](https://github.com/open-telemetry/opentelemetry-collector/tree/main/config/configauth) module and also existing authenticator extensions like the [oauth2](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/oauth2clientauthextension) authenticator.


## **Use Cases**


The Sigv4 authenticator extension provides a way for the OTEL Collector’s Prometheus Remote Write (PRW) Exporter to sign http requests with Sigv4. This would effectively replace the AWS PRW Exporter as the preferred way to send metrics in remote write format to [Amazon Managed Service for Prometheus (AMP)](https://aws.amazon.com/prometheus/). The Sigv4 authenticator extension could be used to sign any http requests that require Sigv4. The Sigv4 authenticator extension will not be able to sign gRPC requests.


<p align="center">
        <img src=images/E2EDiagram.png />
</p>

<p style="text-align: center;"><i>Diagram 1: E2E data path</i></p>


## **Sigv4 Authenticator Implementation Details**


### **config.go**

We first introduce the `config.go` module, which defines the configuration options a user can make and also does some data validation to catch errors.


`Config` is a struct of the AWS Sigv4 authentication configurations.


```go
// Config for the Sigv4 Authenticator

type Config struct {
    Region string `mapstructure:"region,omitempty"`
    Service string `mapstructure:"service,omitempty"`
    AssumeRole AssumeRole `mapstructure:"assume_role"`
	credsProvider *aws.CredentialsProvider
}
```


* `Region` is the AWS region for AWS Sigv4. This is an optional field.
    * Note that an attempt will be made to obtain a valid region from the endpoint of the service you are exporting to
* `Service` is the AWS service for AWS Sigv4. This is an optional field.
    * Note that an attempt will be made to obtain a valid service from the endpoint of the service you are exporting to
* `AssumeRole` is the AssumeRole struct that holds the configuration needed to assume a role, which includes the ARN and SessionName
* `credsProvider` holds the necessary AWS CredentialsProvider. This is a private field and will not be configured by the user. We store the provider instead of the credentials themselves so we can ensure we have refreshed credentials when we sign a request.

`AssumeRole` is a struct that holds the configuration needed to assume a role. 


```go
type AssumeRole struct {
	ARN                  string `mapstructure:"arn,omitempty"`
	SessionName          string `mapstructure:"session_name,omitempty"`
}
```


* `ARN` is the Amazon Resource Name (ARN) of a role to assume. This is an optional field.
* `SessionName`: The name of a role session. This is an optional field.


`Validate()` is a method that checks if the Extension configuration is valid. We aim to catch most errors here, so we can ensure that we fail early and to avoid revalidating static data. We also set our AWS credentials here after we check them.


```go
func (cfg *Config) Validate() error {
	credsProvider, err := getCredsFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("could not retrieve credential provider: %w", err)
	}
	if credsProvider == nil {
		return fmt.Errorf("credsProvider cannot be nil")
	}
	return nil
}
```


### **extension.go**


Next, we introduce `extension.go`, which contains the bulk of the implementation of the Sigv4 authenticator. This includes implementing the `ClientAuthenticator` interface and its necessary methods.


`sigv4Auth` is a struct that implements the `ClientAuthenticator` interface. It must implement two methods, `RoundTripper()` and `PerRPCCredentials()`. Additionally, it must also implement a `Start()` and a `Shutdown()` method. For our Sigv4 authenticator, both of these methods won’t do anything, so we instead embed `componenthelper.StartFunc` and `componenthelper.ShutdownFunc` to get that default behavior.


```go
type sigv4Auth struct {
    cfg *Config
    logger *zap.logger
    awsSDKInfo string
    componenthelper.StartFunc
    componenthelper.ShutdownFunc
}
```


`RoundTripper()` is a method that returns a custom `signingRoundTripper`. We use the `sigv4Auth` struct here instead of the `signingRoundTripper` struct for `RoundTripper()` because `sigv4Auth` is the struct that implements the `ClientAuthenticator` interface. Additionally, it makes it so that `RoundTripper()` creates a new round tripper every time for concurrency purposes. 


```go
func (sa *sigv4Auth) RoundTripper(base http.RoundTripper) (http.RoundTripper, error)
    // Create a signingRoundTripper struct
    rt := signingRoundTripper {
        transport: base
        signer:
        region:
        service:
        credsProvider:
        awsSDKInfo:
        logger:
    }

    // return the address of the signingRoundTripper and no error if
    return &rt, nil
}
```


`PerRPCCredentials()` will be implemented to satisfy the `ClientAuthenticator` interface, but AWS API requests don’t use gRPC, so the Sigv4 authenticator will not implement this method. Instead, it will return nil and an error stating that it is not implemented.


```go
func (sa *sigv4Auth) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
    return nil, errors.New("Not Implemented")
}
```


`newSigv4Extension()` is a function called by `createExtension()` in `factory.go`. It returns a new `sigv4Auth` struct.


```go
func newSigv4Extension(cfg *Config, awsSDKInfo string, logger *zap.logger) (*sigv4Auth, error) {
    return &sigv4Auth{
        cfg: cfg,
        logger: logger,
        awsSDKInfo: awsSDKInfo,
    }, nil
}
```


`getCredsFromConfig()` is a function that gets AWS credentials from the Config. It returns `*aws.Credentials`.


```go
func getCredsFromConfig(cfg *Config) (*aws.Credentials, error) {
	awscfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, err
	}
	if cfg.RoleARN != "" {
		stsSvc := sts.NewFromConfig(awscfg)

		identifier := cfg.RoleSessionName
		if identifier == "" {
			b := make([]byte, 5)
			rand.Read(b)
			identifier = base32.StdEncoding.EncodeToString(b)
		}

		provider := stscreds.NewAssumeRoleProvider(stsSvc, cfg.RoleARN, func(o *stscreds.AssumeRoleOptions) {
			o.RoleSessionName = "otel-" + identifier
		})
		awscfg.Credentials = aws.NewCredentialsCache(provider)
	}

	_, err = awscfg.Credentials.Retrieve(context.Background())
	if err != nil {
		return nil, err
	}

	return &awscfg.Credentials, nil
}
```


`cloneRequest()` is a function that clones an `http.request` for thread safety purposes.


```go
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	return r2
}
```


### **signingroundtripper.go**
`signingroundtripper.go` contains the rest of the implementation of the Sigv4 authenticator. We implement the `RoundTripper` interface and its necessary methods.


`signingRoundTripper` is a custom `RoundTripper` struct (i.e. a custom `http.RoundTripper`). This struct implements the [RoundTripper interface](https://pkg.go.dev/net/http#RoundTripper.RoundTrip) and will implement the method `RoundTrip()`.


```go
type signingRoundTripper struct {
    transport http.RoundTripper
    signer *v4.Signer
    region string
    service string
    credsProvider *aws.CredentialsProvider
    awsSDKInfo string
	logger *zap.Logger
}
```


`RoundTrip()` executes a single HTTP transaction and returns an HTTP response. It will sign the request with Sigv4. This method is implemented to satisfy the `RoundTripper` interface. Here, we will have multiple error checks throughout the method to ensure a proper `RoundTrip()` will succeed.


```go
func (si *signingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
    reqBody, err := req.GetBody()
    if err != nil {
        return nil, err
    }

    content, err := io.ReadAll(reqBody)
    reqBody.Close()
    if err != nil {
        return nil, err
    }
    body := bytes.NewReader(content)

    // Clone request to ensure thread safety.
    // Impl. as helper function
    req2 := cloneRequest(req)

    // Add the runtime information to the User-Agent header of the request
    ua := req2.Header.Get("User-Agent")
    if len(ua) > 0 {
        ua = ua + " " + si.awsSDKInfo
    } else {
        ua = si.awsSDKInfo
    }
    req2.Header.Set("User-Agent", ua)

	// Hash the request
	h := sha256.New()
	_, _ = io.Copy(h, body)
	payloadHash := hex.EncodeToString(h.Sum(nil))

	// Use user provided service/region if specified, use inferred service/region if not, then sign the request
	service, region := si.inferServiceAndRegion(req)
	creds, err := (*si.credsProvider).Retrieve(req.Context())
	if err != nil {
		return nil, fmt.Errorf("error retrieving credentials: %w", err)
	}
	err = si.signer.SignHTTP(req.Context(), creds, req2, payloadHash, service, region, time.Now())
	if err != nil {
		return nil, fmt.Errorf("error signing the request: %w", err)
	}

	// Send the request
	return si.transport.RoundTrip(req2)
} 
```


**Performance Considerations of `RoundTrip()`**


We take a closer look at the performance of `RoundTrip()`, since it will be heavily used i.e. called by every HTTP transaction, and can also be used for exporting to any AWS service. Our goal here is to outline both the memory and runtime performance of this method, as well as explain design choices.


```go
    reqBody, err := req.GetBody()
    .
    .
    .
    content, err := io.ReadAll(reqBody)
    reqBody.Close()
    .
    .
    .
    body := bytes.NewReader(content)
```


First, we obtain the body. This should not have an impact on runtime performance or memory performance as we do not expect to handle http requests that are exponentially large in size*. `body` is necessary as it is passed into the `Sign()` method later.


**Note that OTEL does not limit the size of HTTP requests, except if the exporters themselves set a limit. No information could be found on the size of AWS API requests either. If the request body is exponentially large it could affect performance but it is a reasonably safe assumption that that will not be the case.*


```go
    req2 := cloneRequest(req)
    .
    .
    .
    ua := req2.Header.Get("User-Agent")
    ua = ua + " " + si.awsSDKInfo // or ua = si.awsSDKInfo
    req2.Header.Set("User-Agent", ua)
```


Next, we clone the request and also add runtime information to the User-Agent header. `cloneRequest()` is a helper function that makes a shallow copy of the request and a deep copy of the header. While this may slow the performance of `RoundTrip()`, it is necessary to ensure thread safety*. Overall, this should not have a large impact on either runtime or memory performance, as again we do not expect to handle http requests that are exponentially large in size. Adding runtime information to the header also will not impact memory or runtime performance either. `req2` will be passed into `Sign()`.


**Design Decision: The necessity of `cloneRequest()` is due to the necessity to read the body. If we want to avoid using `cloneRequest()`, that can only be done by avoiding reading the body.*




```go
    h := sha256.New()
	_, _ = io.Copy(h, body)
	payloadHash := hex.EncodeToString(h.Sum(nil))

    err = si.signer.SignHTTP(req.Context(), creds, req2, payloadHash, service, region, time.Now())
    resp, err := si.transport.RoundTrip(req2)
    return resp, nil
```


Lastly, we compute the hash, sign the request, send the request, and return the response. These are all necessary for the authenticator, and while this is the main point of consideration for the performance of `RoundTrip()`, it cannot be optimized further.


`inferServiceAndRegionFromRequestURL()` attempts to infer a region and a service from the URL in the `http.request`.


```go
func (si *signingRoundTripper) inferServiceAndRegionFromRequestURL(r *http.Request) (service string, region string) {
    // check for service and region from URL

	return service, region
}
```


### **factory.go**


Lastly, we introduce `factory.go`, which provides the logic for creating the Sigv4 authenticator extension.


`NewFactory()` creates a factory for the Sigv4 Authenticator Extension. 


```go
func NewFactory() extension.Factory {
    return extensionhelper.NewFactory(
        "sigv4auth",
        createDefaultConfig,
        createExtension)
}
```


`createDefaultConfig()` usually creates a config struct with default values. In our case, there are no sensible defaults to provide so it won’t do much except set the ID.


```go
func createDefaultConfig() component.Config {
    return &Config{}
}
```


`createExtension()` calls `newSigv4Extension()` in `extension.go` to create the extension. Here, we add `awsSDKInfo` that will be used later to add that info to our `signingRoundTripper` in our `RoundTripper()` method.


```go
func createExtension(_ context.Context, set extension.CreateSettings, cfg component.Config) (extension.Extension, error) {
    awsSDKInfo := fmt.Sprintf("%s/%s", aws.SDKName, aws.SDKVersion)
    return newSigv4Extension(cfg.(*Config), awsSDKInfo, set.Logger)
}
```


### **Sigv4 Component Diagram**


Below is a diagram of the Sigv4 authenticator’s components and how the interact with each other. This is a detailed look at the “Sigv4 Authenticator” box in diagram 1.


<p align="center">
        <img src=images/Sigv4ComponentDiagram.png />
</p>

<p style="text-align: center;"><i>Diagram 2: Sigv4 component diagram</i></p>


## **Test Design**


### **config_test.go**


`TestLoadConfig()`


```go
func TestLoadConfig(t *testing.T) {
   //Load config yaml from test data and assert against expected values
}
```


`TestLoadConfigError()`


```go
func TestLoadConfigError(t *testing.T) {
   // Load bad config yaml from test data and assert right errors thrown
}
```


### **extension_test.go**


`TestNewSigv4Extension()`


```go
func TestNewSigv4Extension(t *testing.T) {
    // assert that newSigv4Extension() returns a non nil, correct sigv4Auth 
    // struct
}
```


`TestRoundTripper()`


```go
func TestRoundTripper(t *testing.T) {
    // assert that a signingRoundTripper created with region/service 
    // provided is valid
    // assert that a signingRoundTripper created with one/both of
    // region/service not provided throws an error
}
```


```go
func TestPerRPCCredentials(t *testing.T) {
    // assert that an error is thrown
}
```


```go
func TestGetCredsFromConfig(t *testing.T) {
    // assert that credentials can be fetched properly and no error is thrown
    // will not test for fetching credentials when a ARN is provided, due to
    // it needing to fetch a token from AWS.
}
```


```go
func TestCloneRequest(t *testing.T) {
    // assert that a request and a clone have equal headers and body
}
```


### **signingroundtripper_test.go**


`TestRoundTrip()`


```go
func TestRoundTrip(t *testing.T) {
    // assert http req are signed by sigv4
    // assert status code 200 on successful round trip
    // assert nil/err or no nil/err on a bad/good round trip
}
```


```go
func TestInferServiceAndRegionFromRequestURL(t *testing.T) {
    // assert empty service/region string if invalid URL
    // assert service/region found with proper URL
}
```


### **factory_test.go**


`TestNewFactory()`


```go
func TestNewFactory(t *testing.T) {
    // assert that the new factory is not nil
}
```


`TestCreateDefaultConfig()`


```go
func TestCreateDefaultConfig(t *testing.T) {
    // assert createDefaultConfig() creates config struct that only sets ID
}
```


`TestCreateExtension()`


```go
func TestCreateExtension(t *testing.T) {
    // assert no error/nil
}
```


## **Package File Structure**


File structure will mimic that of existing authentication extension implementations.


* In `opentelemetry-collector-contrib/extension`, the `sigv4authextension` directory will be added, containing the `sigv4authextension` package. Within this directory will be:
    * `config.go`
    * `extension.go`
    * `signingroundtripper.go`
    * `factory.go`
    * `config_test.go`
    * `extension_test.go`
    * `signingroundtripper_test.go`
    * `factory_test.go`
    * `go.mod`
    * `go.sum`
    * `doc.go` for godoc purposes
    * `Makefile` for make purposes
    * `README.md` to provide information for the user on how to set up the authenticator
    * `design.md` for insight into the design of the extension
    * an `images` directory for images used by `design.md`
    * and a `testdata` directory for data needed for tests (mostly `config_test.go`)
        * `config.yaml` to load a good config and test against expected values
        * `config_bad.yaml`to load a bad config and test that the right errors are thrown


Additionally, a few other files will be modified to reflect changes made, for example:


* `CHANGELOG.md` 
* `go.mod`
* `go.sum`
* `versions.yaml`
* `.github/CODEOWNERS`
* `cmd/configschema/go.mod`
* `internal/components/components.go`
* `internal/components/extensions_test.go`
