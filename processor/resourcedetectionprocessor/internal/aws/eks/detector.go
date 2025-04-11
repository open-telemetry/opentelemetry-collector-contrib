// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eks // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "eks"

	// Environment variable that is set when running on Kubernetes.
	kubernetesServiceHostEnvVar = "KUBERNETES_SERVICE_HOST"
	authConfigmapNS             = "kube-system"
	authConfigmapName           = "aws-auth"

	clusterNameAwsEksTag     = "aws:eks:cluster-name"
	clusterNameEksTag        = "eks:cluster-name"
	kubernetesClusterNameTag = "kubernetes.io/cluster/"
)

type detectorUtils interface {
	getConfigMap(ctx context.Context, namespace string, name string) (map[string]string, error)
	getClusterName(ctx context.Context, logger *zap.Logger) string
	getClusterNameTagFromReservations([]*ec2.Reservation) string
	getCloudAccountID(ctx context.Context, logger *zap.Logger) string
}

type eksDetectorUtils struct {
	clientset *kubernetes.Clientset
}

// detector for EKS
type detector struct {
	utils  detectorUtils
	logger *zap.Logger
	err    error
	ra     metadata.ResourceAttributesConfig
	rb     *metadata.ResourceBuilder
}

var _ internal.Detector = (*detector)(nil)

var _ detectorUtils = (*eksDetectorUtils)(nil)

// NewDetector returns a resource detector that will detect AWS EKS resources.
func NewDetector(set processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	utils, err := newK8sDetectorUtils()

	return &detector{
		utils:  utils,
		logger: set.Logger,
		err:    err,
		ra:     cfg.ResourceAttributes,
		rb:     metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}, nil
}

// Detect returns a Resource describing the Amazon EKS environment being run in.
func (d *detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	// Check if running on EKS.
	isEKS, err := isEKS(ctx, d.utils)
	if !isEKS {
		d.logger.Debug("Unable to identify EKS environment", zap.Error(err))
		return pcommon.NewResource(), "", err
	}

	d.rb.SetCloudProvider(conventions.AttributeCloudProviderAWS)
	d.rb.SetCloudPlatform(conventions.AttributeCloudPlatformAWSEKS)
	if d.ra.CloudAccountID.Enabled {
		accountID := d.utils.getCloudAccountID(ctx, d.logger)
		d.rb.SetCloudAccountID(accountID)
	}

	if d.ra.K8sClusterName.Enabled {
		clusterName := d.utils.getClusterName(ctx, d.logger)
		d.rb.SetK8sClusterName(clusterName)
	}

	return d.rb.Emit(), conventions.SchemaURL, nil
}

func isEKS(ctx context.Context, utils detectorUtils) (bool, error) {
	if os.Getenv(kubernetesServiceHostEnvVar) == "" {
		return false, nil
	}

	// Make HTTP GET request
	awsAuth, err := utils.getConfigMap(ctx, authConfigmapNS, authConfigmapName)
	if err != nil {
		return false, fmt.Errorf("isEks() error retrieving auth configmap: %w", err)
	}

	return awsAuth != nil, nil
}

func newK8sDetectorUtils() (*eksDetectorUtils, error) {
	// Get cluster configuration
	confs, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	// Create clientset using generated configuration
	clientset, err := kubernetes.NewForConfig(confs)
	if err != nil {
		return nil, errors.New("failed to create clientset for Kubernetes client")
	}

	return &eksDetectorUtils{clientset: clientset}, nil
}

func (e eksDetectorUtils) getConfigMap(ctx context.Context, namespace string, name string) (map[string]string, error) {
	cm, err := e.clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve ConfigMap %s/%s: %w", namespace, name, err)
	}
	return cm.Data, nil
}

func (e eksDetectorUtils) getClusterName(ctx context.Context, logger *zap.Logger) string {
	defaultErrorMessage := "Unable to get EKS cluster name"
	sess, err := session.NewSession()
	if err != nil {
		logger.Warn(defaultErrorMessage, zap.Error(err))
		return ""
	}

	ec2Svc := ec2metadata.New(sess)
	region, err := ec2Svc.Region()
	if err != nil {
		logger.Warn(defaultErrorMessage, zap.Error(err))
		return ""
	}

	svc := ec2.New(sess, aws.NewConfig().WithRegion(region))
	instanceIdentityDocument, err := ec2Svc.GetInstanceIdentityDocumentWithContext(ctx)
	if err != nil {
		logger.Warn(defaultErrorMessage, zap.Error(err))
		return ""
	}

	instances, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			aws.String(instanceIdentityDocument.InstanceID),
		},
	})
	if err != nil {
		logger.Warn(defaultErrorMessage, zap.Error(err))
		return ""
	}

	clusterName := e.getClusterNameTagFromReservations(instances.Reservations)
	if len(clusterName) == 0 {
		logger.Warn("Failed to detect EKS cluster name. No tag for cluster name found on EC2 instance")
		return ""
	}

	return clusterName
}

func (e eksDetectorUtils) getClusterNameTagFromReservations(reservations []*ec2.Reservation) string {
	for _, reservation := range reservations {
		for _, instance := range reservation.Instances {
			for _, tag := range instance.Tags {
				if tag.Key == nil {
					continue
				}

				if *tag.Key == clusterNameAwsEksTag || *tag.Key == clusterNameEksTag {
					return *tag.Value
				} else if strings.HasPrefix(*tag.Key, kubernetesClusterNameTag) {
					return strings.TrimPrefix(*tag.Key, kubernetesClusterNameTag)
				}
			}
		}
	}

	return ""
}

func (e eksDetectorUtils) getCloudAccountID(ctx context.Context, logger *zap.Logger) string {
	defaultErrorMessage := "Unable to get EKS cluster account ID"
	sess, err := session.NewSession()
	if err != nil {
		logger.Warn(defaultErrorMessage, zap.Error(err))
		return ""
	}

	ec2Svc := ec2metadata.New(sess)
	instanceIdentityDocument, err := ec2Svc.GetInstanceIdentityDocumentWithContext(ctx)
	if err != nil {
		logger.Warn(defaultErrorMessage, zap.Error(err))
		return ""
	}

	return instanceIdentityDocument.AccountID
}
