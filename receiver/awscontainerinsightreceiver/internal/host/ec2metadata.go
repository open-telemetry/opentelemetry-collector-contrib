// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package host // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	awsec2metadata "github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type metadataClient interface {
	GetInstanceIdentityDocumentWithContext(ctx context.Context) (awsec2metadata.EC2InstanceIdentityDocument, error)
}

type ec2MetadataProvider interface {
	getInstanceID() string
	getInstanceType() string
	getRegion() string
	getInstanceIP() string
}

type ec2Metadata struct {
	logger           *zap.Logger
	client           metadataClient
	refreshInterval  time.Duration
	instanceID       string
	instanceType     string
	instanceIP       string
	region           string
	instanceIDReadyC chan bool
	instanceIPReadyC chan bool
}

type ec2MetadataOption func(*ec2Metadata)

func newEC2Metadata(ctx context.Context, session *session.Session, refreshInterval time.Duration,
	instanceIDReadyC chan bool, instanceIPReadyC chan bool, logger *zap.Logger, options ...ec2MetadataOption) ec2MetadataProvider {
	emd := &ec2Metadata{
		client:           awsec2metadata.New(session),
		refreshInterval:  refreshInterval,
		instanceIDReadyC: instanceIDReadyC,
		instanceIPReadyC: instanceIPReadyC,
		logger:           logger,
	}

	for _, opt := range options {
		opt(emd)
	}

	shouldRefresh := func() bool {
		// stop the refresh once we get instance ID and type successfully
		return emd.instanceID == "" || emd.instanceType == "" || emd.instanceIP == ""
	}

	go RefreshUntil(ctx, emd.refresh, emd.refreshInterval, shouldRefresh, 0)

	return emd
}

func (emd *ec2Metadata) refreshFromIMDS(ctx context.Context) error {
	doc, err := emd.client.GetInstanceIdentityDocumentWithContext(ctx)
	if err != nil {
		emd.logger.Error("Failed to get ec2 metadata", zap.Error(err))
		return err
	}

	emd.instanceID = doc.InstanceID
	emd.instanceType = doc.InstanceType
	emd.region = doc.Region
	emd.instanceIP = doc.PrivateIP
	return nil
}

const (
	NODE_NAME_ENV       = "K8S_NODE_NAME"
	INSTANCE_TYPE_LABEL = "node.kubernetes.io/instance-type"
)

func (emd *ec2Metadata) refreshFromKubernetes(ctx context.Context) error {
	nodeName := os.Getenv(NODE_NAME_ENV)
	if nodeName == "" {
		return fmt.Errorf("%s environment variable not set", NODE_NAME_ENV)
	}

	k8sClient := k8sclient.Get(emd.logger)
	node, err := k8sClient.GetClientSet().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil || node == nil {
		return fmt.Errorf("failed to get node %s: %v", nodeName, err)
	}

	splits := strings.Split(node.Spec.ProviderID, "/")
	if len(splits) < 5 {
		return fmt.Errorf("invalid providerID format %s", node.Spec.ProviderID)
	}
	instanceID := splits[4]
	zone := splits[3]
	region := zone[:len(zone)-1]
	emd.instanceID = instanceID
	emd.region = region

	instanceType, ok := node.ObjectMeta.Labels[INSTANCE_TYPE_LABEL]
	if ok {
		emd.instanceType = instanceType
	}

	privateIP := ""
	for _, addr := range node.Status.Addresses {
		if addr.Type == "InternalIP" {
			privateIP = addr.Address
			break
		}
	}
	emd.instanceIP = privateIP

	return nil
}

func (emd *ec2Metadata) refresh(ctx context.Context) {
	emd.logger.Info("Fetch instance id and type from ec2 metadata")

	err := emd.refreshFromIMDS(ctx)
	if err != nil {
		emd.logger.Error("Failed to get ec2 metadata, falling back to Kubernetes API", zap.Error(err))
		err = emd.refreshFromKubernetes(ctx)
		if err != nil {
			emd.logger.Error("Failed to get ec2 metadata from Kubernetes API", zap.Error(err))
			return
		}
	}

	// notify ec2tags and ebsvolume that the instance id is ready
	if emd.instanceID != "" {
		close(emd.instanceIDReadyC)
	}
	// notify ecsinfo that the instance id is ready
	if emd.instanceIP != "" {
		close(emd.instanceIPReadyC)
	}
}

func (emd *ec2Metadata) getInstanceID() string {
	return emd.instanceID
}

func (emd *ec2Metadata) getInstanceType() string {
	return emd.instanceType
}

func (emd *ec2Metadata) getRegion() string {
	return emd.region
}

func (emd *ec2Metadata) getInstanceIP() string {
	return emd.instanceIP
}