package awscloudwatchreceiver

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const (
	awsCredentialModeProfile        = "profiling"
	awsCredentialModeRoleDelegation = "role_delegation"
	awsCredentialModeAccessKeys     = "access_keys"
)

func (l *logsReceiver) configureAWSClient(ctx context.Context) error {
	if l.client != nil {
		return nil
	}

	var cfg aws.Config
	var err error

	switch l.pollingApproach {
	case awsCredentialModeProfile:
		cfg, err = l.configureProfiling(ctx)
		//creds, _ := cfg.Credentials.Retrieve(ctx)
		//fmt.Println("AccessKeyID: ", creds.AccessKeyID)
	case awsCredentialModeRoleDelegation:
		cfg, err = l.configureRoleDelegation(ctx)
	case awsCredentialModeAccessKeys:
		cfg, err = l.configureAccessKeys(ctx)
	default:
		return fmt.Errorf("incomplete AWS configuration: must define credential mode as %s | %s | %s",
			awsCredentialModeProfile, awsCredentialModeRoleDelegation, awsCredentialModeAccessKeys)
	}

	if err != nil {
		return err
	}

	l.client = cloudwatchlogs.NewFromConfig(cfg)
	return nil
}

func (l *logsReceiver) configureProfiling(ctx context.Context) (aws.Config, error) {
	return config.LoadDefaultConfig(ctx,
		config.WithRegion(l.region),
		config.WithSharedConfigProfile(l.profile),
		config.WithEC2IMDSEndpoint(l.imdsEndpoint),
	)
}

func (l *logsReceiver) configureRoleDelegation(ctx context.Context) (aws.Config, error) {
	if l.externalId == "" {
		return aws.Config{}, errors.New("ExternalId is missing")
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(l.region),
		config.WithEC2IMDSEndpoint(l.imdsEndpoint),
	)
	if err != nil {
		return cfg, err
	}

	stsClient := sts.NewFromConfig(cfg)
	stsCredsProvider := stscreds.NewAssumeRoleProvider(stsClient, l.awsRoleArn, func(aro *stscreds.AssumeRoleOptions) {
		aro.ExternalID = &l.externalId
	})
	cfg.Credentials = aws.NewCredentialsCache(stsCredsProvider)
	return cfg, nil
}

func (l *logsReceiver) configureAccessKeys(ctx context.Context) (aws.Config, error) {
	return config.LoadDefaultConfig(ctx,
		config.WithRegion(l.region),
		config.WithEC2IMDSEndpoint(l.imdsEndpoint),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				l.awsAccessKey,
				l.awsSecretKey,
				"",
			),
		),
	)
}
