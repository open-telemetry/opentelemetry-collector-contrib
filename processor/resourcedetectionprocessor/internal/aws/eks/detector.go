// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eks // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks"

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
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

	clusterNameAwsEksTag = "aws:eks:cluster-name"
	clusterNameEksTag    = "eks:cluster-name"
)

type detectorUtils interface {
	getConfigMap(ctx context.Context, namespace string, name string) (map[string]string, error)
	getClusterName() (string, error)
	getClusterNameTagFromReservations([]types.Reservation) string
}

type eksDetectorUtils struct {
	clientset *kubernetes.Clientset
}

// detector for EKS
type detector struct {
	utils  detectorUtils
	logger *zap.Logger
	err    error
	rb     *metadata.ResourceBuilder
}

var _ internal.Detector = (*detector)(nil)

var _ detectorUtils = (*eksDetectorUtils)(nil)

// NewDetector returns a resource detector that will detect AWS EKS resources.
func NewDetector(set processor.CreateSettings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	utils, err := newK8sDetectorUtils()

	return &detector{
		utils:  utils,
		logger: set.Logger,
		err:    err,
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

	// The error is unhandled because we want to return successfully detected resources
	// regardless of an error. The caller will properly handle any error hit while getting
	// the cluster name.
	clusterName, err := d.utils.getClusterName()
	d.rb.SetK8sClusterName(clusterName)
	return d.rb.Emit(), conventions.SchemaURL, err
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
		return nil, fmt.Errorf("failed to create clientset for Kubernetes client")
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

func (e eksDetectorUtils) getClusterName() (string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", err
	}

	ec2IMS := imds.NewFromConfig(cfg)
	regionOutput, err := ec2IMS.GetRegion(context.TODO(), &imds.GetRegionInput{})
	if err != nil {
		return "", err
	}
	cfg.Region = regionOutput.Region

	client := ec2.NewFromConfig(cfg)
	describeInstances, err := client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{})
	if err != nil {
		return "", err
	}

	clusterName := e.getClusterNameTagFromReservations(describeInstances.Reservations)

	if len(clusterName) == 0 {
		return clusterName, fmt.Errorf("Failed to detect EKS cluster name. No tag for cluster name found on EC2 instance")
	}
	return clusterName, nil
}

func (e eksDetectorUtils) getClusterNameTagFromReservations(reservations []types.Reservation) string {
	for _, reservation := range reservations {
		for _, instance := range reservation.Instances {
			for _, tag := range instance.Tags {
				if *tag.Key == clusterNameAwsEksTag || *tag.Key == clusterNameEksTag {
					return *tag.Value
				}
			}
		}
	}

	return ""
}
