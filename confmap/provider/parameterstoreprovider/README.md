## Summary
This package provides a `ConfigMapProvider` implementation for AWS SSM ParameterStore (`parameterstore`) that gives 
the Collector the ability to read data stored in AWS SSM ParameterStore.
## How it works
- Just use the placeholders with the following pattern `${parameterstore:<arn or name>}`
- Make sure you have the `ssm:GetParameter` in the OTEL Collector Role
- If your parameter is a json string, you can get the value for a json key using the following pattern `${parameterstore:<arn or name>#json-key}`
- If your parameter is a SecureString, you can enable decryption of the value using following pattern `${parameterstore:<arn or name>?withDecryption=true}`

Prerequisites:
- Need to set up access keys from IAM console (aws_access_key_id and aws_secret_access_key) with permission to access AWS SSM ParameterStore
- For details, can take a look at https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/
