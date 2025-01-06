## Summary
This package provides a `ConfigMapProvider` implementation for Amazon Secrets Manager (`secretsmanager`) that allows 
the  
Collector the ability to read data stored in AWS Secrets Manager.
## How it works
- Just use the placeholders with the following pattern `${secretsmanager:<arn or name>}`
- Make sure you have the `secretsmanager:GetSecretValue` in the OTEL Collector Role
- If your secret is a json string, you can get the value for a json key using the following pattern `${secretsmanager:<arn or name>#json-key}`

Prerequisites:
- Need to set up access keys from IAM console (aws_access_key_id and aws_secret_access_key) with permission to access Amazon Secrets Manager
- For details, can take a look at https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/
