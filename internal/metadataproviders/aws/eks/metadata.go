// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eks // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/aws/eks"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/smithy-go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

const (
	// clusterNameAwsEksTag is applied by EKS and lives in the reserved "aws:"
	// tag namespace, which users cannot write to. It is the only cluster name
	// source on an EC2 node that EKS itself owns.
	clusterNameAwsEksTag = "aws:eks:cluster-name"
	// clusterNameEksTag is applied by EKS managed node groups but sits in a
	// user-writable namespace.
	clusterNameEksTag = "eks:cluster-name"
	// kubernetesClusterNameTag is applied by users and by provisioning tools.
	// A node can carry several of them, and the cluster name is in the key.
	kubernetesClusterNameTag = "kubernetes.io/cluster/"
	// kubernetesClusterOwnedValue marks a resource as belonging to that
	// cluster rather than being shared between clusters.
	kubernetesClusterOwnedValue = "owned"
)

// ErrClusterIdentityMismatch reports that EKS described the named cluster and
// it is not the cluster this node is running in. It is the one ARN failure that
// says something about the cluster name itself: the name is wrong, rather than
// unproven, so a caller reporting the name separately can stop reporting it.
var ErrClusterIdentityMismatch = errors.New("the named cluster is not the cluster this node runs in")

// errAmbiguousClusterName reports that a node's EC2 tags name more than one
// cluster at the same precedence, so there is no single cluster the node can be
// said to belong to. Callers see this as an empty cluster name with
// InstanceMetadata.ClusterNameAmbiguous set, not as an error.
var errAmbiguousClusterName = errors.New("node EC2 tags name more than one cluster at the same precedence")

// The remaining errors reported by ClusterARN. None of them carry the cluster
// name, the issuer URLs, or any other value read from the node or returned by
// the service. Two of the three tags the cluster name can come from sit in
// namespaces a user can write to, so the name can name a cluster this node has
// nothing to do with, and copying it into a log line or into an error that
// reaches the collector's own output would repeat it as if it were this node's.
var (
	errNoClusterName              = errors.New("no cluster name for this node")
	errNoRegion                   = errors.New("no region for this node")
	errLiveIssuerUnavailable      = errors.New("the OIDC issuer of the cluster this node runs in is unavailable from the Kubernetes API")
	errDescribeClusterFailed      = errors.New("the eks:DescribeCluster request failed")
	errNoClusterInResponse        = errors.New("eks:DescribeCluster returned no cluster")
	errDescribedIssuerUnavailable = errors.New("eks:DescribeCluster returned no OIDC issuer for the named cluster")
	errNoClusterARN               = errors.New("eks:DescribeCluster returned no ARN for the named cluster")
)

type Provider interface {
	ClusterVersion() (string, error)
	OIDCIssuer(ctx context.Context) (string, error)
	GetK8sInstanceMetadata(ctx context.Context) (InstanceMetadata, error)
	GetInstanceMetadata(ctx context.Context) (InstanceMetadata, error)
	// ClusterARN must return an ARN only for a cluster the implementation has
	// confirmed is the cluster this node is running in. The cluster name is
	// read from an EC2 tag and two of the tags it can come from are
	// user-writable, so an implementation that returns the ARN of whatever
	// cluster the name resolves to reports a cluster identity that was never
	// established. Returning an ARN without that check is a defect, not a
	// trade-off.
	ClusterARN(ctx context.Context, clusterName string) (string, error)
	SetRegionInstanceID(region, instanceID string)
}

type ec2Client interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

type eksClient interface {
	DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
}

type metadataClient struct {
	instanceMetadata InstanceMetadata
	clientset        kubernetes.Interface
	ec2Client        ec2Client
	eksClient        eksClient
	nodeName         string
}

type InstanceMetadata struct {
	AvailabilityZone string
	AccountID        string
	InstanceID       string
	Region           string
	ClusterName      string
	// ClusterNameAmbiguous reports that ClusterName was left empty because the
	// node's tags named more than one cluster at the same precedence, rather
	// than because they named none. It lets a caller say why the cluster name
	// is missing without reporting any of the names involved.
	ClusterNameAmbiguous bool
	ImageID              string
	InstanceType         string
	Hostname             string
}

var _ Provider = (*metadataClient)(nil)

func NewProvider(cfg aws.Config, nodeName string, apiConf k8sconfig.APIConfig) (Provider, error) {
	conf, err := k8sconfig.CreateRestConfig(apiConf)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize provider. Could not load cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(conf)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize provider. Could not create clientset for Kubernetes client: %w", err)
	}

	return &metadataClient{
		instanceMetadata: InstanceMetadata{},
		nodeName:         nodeName,
		clientset:        clientset,
		ec2Client:        ec2.NewFromConfig(cfg),
		eksClient:        eks.NewFromConfig(cfg),
	}, nil
}

func (c *metadataClient) ClusterVersion() (string, error) {
	serverVersion, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve cluster version: %w", err)
	}
	return serverVersion.GitVersion, nil
}

func (c *metadataClient) OIDCIssuer(ctx context.Context) (string, error) {
	data, err := c.clientset.Discovery().RESTClient().Get().AbsPath("/.well-known/openid-configuration").Do(ctx).Raw()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve OIDC configuration: %w", err)
	}
	var oidcConfig struct {
		Issuer string `json:"issuer"`
	}
	if err := json.Unmarshal(data, &oidcConfig); err != nil {
		return "", fmt.Errorf("failed to parse OIDC configuration: %w", err)
	}
	return oidcConfig.Issuer, nil
}

// GetK8sInstanceMetadata retrieves region, instanceID, and availabilityZone attributes from the K8s node's providerID.
// It requires the node name which is passed to the constructor.
// If region or instanceID are not found, it returns an error as they are mandatory to query ec2 API for the full metadata.
func (c *metadataClient) GetK8sInstanceMetadata(ctx context.Context) (InstanceMetadata, error) {
	if c.nodeName == "" {
		return InstanceMetadata{}, errors.New("can't get K8s Instance Metadata; node name is empty")
	}

	node, err := c.clientset.CoreV1().Nodes().Get(ctx, c.nodeName, metav1.GetOptions{})
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("can't get K8s Instance Metadata; failed to retrieve k8s node %w", err)
	}

	region, availabilityZone, instanceID := parseRegionAndInstanceID(node.Spec.ProviderID)
	if region == "" || instanceID == "" {
		return InstanceMetadata{}, fmt.Errorf("failed to retrieve region or instanceId from mockProviderID: %s", node.Spec.ProviderID)
	}

	c.instanceMetadata = InstanceMetadata{Region: region, InstanceID: instanceID, AvailabilityZone: availabilityZone}
	return c.instanceMetadata, nil
}

func parseRegionAndInstanceID(input string) (string, string, string) {
	// Example mockProviderID: "aws:///us-west-2a/i-049ca2df511bec762"
	parts := strings.Split(input, "/")
	// Prevent NPE and return empty strings if the format is invalid
	if len(parts) < 5 || parts[0] != "aws:" || parts[3] == "" || parts[4] == "" {
		return "", "", ""
	}

	availabilityZone := parts[3]
	region := availabilityZone[:len(availabilityZone)-1] // Remove the last character (zone letter)
	instanceID := parts[4]

	return region, availabilityZone, instanceID
}

// GetInstanceMetadata retrieves detailed instance metadata from AWS EC2.
// It requires region and InstanceID to be set, if not set, it calls GetK8sInstanceMetadata to set them
func (c *metadataClient) GetInstanceMetadata(ctx context.Context) (InstanceMetadata, error) {
	if c.instanceMetadata.Region == "" || c.instanceMetadata.InstanceID == "" {
		if _, err := c.GetK8sInstanceMetadata(ctx); err != nil {
			return InstanceMetadata{}, err
		}
	}
	instances, err := c.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{
			c.instanceMetadata.InstanceID,
		},
	}, func(o *ec2.Options) {
		o.Region = c.instanceMetadata.Region
	})
	if err != nil {
		return c.instanceMetadata, fmt.Errorf("failed to get instance metadata: %w", err)
	}

	if len(instances.Reservations) > 0 && len(instances.Reservations[0].Instances) > 0 {
		c.instanceMetadata.AccountID = aws.ToString(instances.Reservations[0].OwnerId)
		c.instanceMetadata.ImageID = aws.ToString(instances.Reservations[0].Instances[0].ImageId)
		c.instanceMetadata.InstanceType = string(instances.Reservations[0].Instances[0].InstanceType)
		c.instanceMetadata.Hostname = aws.ToString(instances.Reservations[0].Instances[0].PrivateDnsName)
	}
	// An ambiguous set of tags leaves the cluster name empty rather than
	// failing the whole fetch: it says nothing about the account ID, image ID,
	// instance type, or hostname read above, which are still this node's. The
	// reason is recorded so the caller can report why the name is missing
	// without reporting any of the names involved.
	clusterName, err := getClusterNameTagFromReservations(instances.Reservations)
	c.instanceMetadata.ClusterName = clusterName
	c.instanceMetadata.ClusterNameAmbiguous = errors.Is(err, errAmbiguousClusterName)

	return c.instanceMetadata, nil
}

// getClusterNameTagFromReservations picks the cluster name by tag precedence
// rather than by the order EC2 happened to return the tags in. A node can carry
// more than one of these tags, so a first-match scan lets the API response
// order decide which cluster the node is reported as belonging to.
//
// The order is strongest claim first: the EKS-owned tag in the reserved "aws:"
// namespace, then the EKS managed node group tag, then the Kubernetes
// discovery tags, which users and provisioning tools write and which mark a
// cluster as either owning the node or sharing it.
//
// Tags of the same precedence naming different clusters report no name and
// set InstanceMetadata.ClusterNameAmbiguous. Ordering them by any rule,
// including a stable one, still answers a question the tags do not answer,
// and it lets whoever wrote the second tag decide which cluster the node is
// reported as belonging to.
func getClusterNameTagFromReservations(reservations []types.Reservation) (string, error) {
	var ownedByEKS, taggedByEKS, ownedByCluster, sharedWithCluster []string
	for _, reservation := range reservations {
		for i := range reservation.Instances {
			for _, tag := range reservation.Instances[i].Tags {
				key := aws.ToString(tag.Key)
				value := aws.ToString(tag.Value)
				switch key {
				case clusterNameAwsEksTag:
					ownedByEKS = append(ownedByEKS, value)
				case clusterNameEksTag:
					taggedByEKS = append(taggedByEKS, value)
				default:
					name, ok := strings.CutPrefix(key, kubernetesClusterNameTag)
					if !ok {
						continue
					}
					if strings.EqualFold(value, kubernetesClusterOwnedValue) {
						ownedByCluster = append(ownedByCluster, name)
					} else {
						sharedWithCluster = append(sharedWithCluster, name)
					}
				}
			}
		}
	}
	for _, candidates := range [][]string{ownedByEKS, taggedByEKS, ownedByCluster, sharedWithCluster} {
		name, err := soleName(candidates)
		if err != nil {
			return "", err
		}
		if name != "" {
			return name, nil
		}
	}
	return "", nil
}

// soleName returns the one distinct non-empty name in candidates. Repeats of
// the same name are not a conflict: every instance in the response carries the
// node's tags, so the same tag is seen more than once.
func soleName(candidates []string) (string, error) {
	sole := ""
	for _, candidate := range candidates {
		switch {
		case candidate == "":
			continue
		case sole == "":
			sole = candidate
		case candidate != sole:
			return "", errAmbiguousClusterName
		}
	}
	return sole, nil
}

// ClusterARN returns the ARN that EKS reports for the named cluster, once EKS
// has confirmed that the named cluster is the cluster this node is running in.
//
// The name comes from an EC2 tag, and two of the three tags it can come from
// sit in namespaces a user can write to. That a cluster of that name exists in
// this account and Region is therefore not evidence that it is this node's
// cluster: a tag naming any other cluster in the same account and Region
// satisfies it just as well, and the ARN EKS returns for that cluster is
// well-formed and wrong.
//
// What distinguishes the clusters is the OIDC issuer URL, which is unique per
// cluster. The node's own Kubernetes API server publishes the issuer of the
// cluster the node is in, and DescribeCluster reports the issuer of the cluster
// that was named, so the ARN is reported only when those two agree. Anything
// else, including either issuer being unavailable, reports no ARN.
//
// It requires permission to run the eks:DescribeCluster action.
func (c *metadataClient) ClusterARN(ctx context.Context, clusterName string) (string, error) {
	if clusterName == "" {
		return "", errNoClusterName
	}
	region := c.instanceMetadata.Region
	if region == "" {
		return "", errNoRegion
	}

	// Read the live cluster's issuer first: when it is unavailable there is
	// nothing to check the response against, so the request is not worth
	// making.
	liveIssuer, err := c.OIDCIssuer(ctx)
	if err != nil {
		return "", errLiveIssuerUnavailable
	}
	liveIssuer = canonicalIssuer(liveIssuer)
	if liveIssuer == "" {
		return "", errLiveIssuerUnavailable
	}

	out, err := c.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	}, func(o *eks.Options) {
		o.Region = region
	})
	if err != nil {
		// The service error is not wrapped: its message echoes the cluster
		// name it was asked about, and that name can come from a tag anyone
		// can write. The API error code is a fixed vocabulary that carries
		// nothing read from the node, and it is the difference between "the
		// role is missing the grant" and "the tag names a cluster that does
		// not exist", so it is kept.
		if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
			return "", fmt.Errorf("%w: %s", errDescribeClusterFailed, apiErr.ErrorCode())
		}
		return "", errDescribeClusterFailed
	}
	if out == nil || out.Cluster == nil {
		return "", errNoClusterInResponse
	}
	if out.Cluster.Identity == nil || out.Cluster.Identity.Oidc == nil {
		return "", errDescribedIssuerUnavailable
	}
	describedIssuer := canonicalIssuer(aws.ToString(out.Cluster.Identity.Oidc.Issuer))
	if describedIssuer == "" {
		return "", errDescribedIssuerUnavailable
	}
	if describedIssuer != liveIssuer {
		return "", ErrClusterIdentityMismatch
	}
	arn := aws.ToString(out.Cluster.Arn)
	if arn == "" {
		return "", errNoClusterARN
	}
	return arn, nil
}

// canonicalIssuer puts an issuer URL in the one form the two sources are
// compared in. Surrounding space and trailing slashes are the only differences
// allowed: they do not name a different cluster, and rejecting them would drop
// the ARN for a cluster that matched. Every other difference, including a
// difference in case, is left to fail the comparison.
func canonicalIssuer(issuer string) string {
	return strings.TrimRight(strings.TrimSpace(issuer), "/")
}

// SetRegionInstanceID sets the region and instance ID.
// This method is useful when caller has the region and instanceID and wants to fetch the ec2 metadata
// without calling the kube api through GetK8sInstanceMetadata first.
func (c *metadataClient) SetRegionInstanceID(region, instanceID string) {
	c.instanceMetadata.Region = region
	c.instanceMetadata.InstanceID = instanceID
}
