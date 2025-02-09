## Summary
This package provides a `ConfigMapProvider` implementation for Amazon SSM Parameter Store (`ssm`) that allows the Collector to read data stored in AWS SSM Parameter Store.

## How it works
- Use placeholders with the following pattern `${ssm:<parameter-name>}`
- To extract a value from a JSON parameter, use `${ssm:<parameter-name>#json-key}`
- Ensure the OTEL Collector Role has the `ssm:GetParameter` permission
- Parameters are automatically decrypted if they are stored as SecureString

## Prerequisites
- Set up access keys from the IAM console (`aws_access_key_id` and `aws_secret_access_key`) with permission to access Amazon SSM Parameter Store
- For details, refer to [AWS SDK for Go V2 Configuration](https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/)

