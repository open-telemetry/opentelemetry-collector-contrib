package opsrampk8sattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/opsrampk8sattributesprocessor"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/opsrampk8sattributesprocessor/internal/moid"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/opsrampk8sattributesprocessor/internal/redis"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type opsrampk8sattributesprocessor struct {
	RedisHost   string
	RedisPort   string
	RedisPass   string
	ClusterName string
	ClusterUid  string
	NodeName    string
	logger      *zap.Logger
	rc          *redis.Client
}

// newOpsrampK8sObjectsProcessor returns a processor
func newOpsrampK8sAttributesProcessor(logger *zap.Logger, rHost, rPort, rPass, clusterName, clusterUid, nodeName string) *opsrampk8sattributesprocessor {
	return &opsrampk8sattributesprocessor{
		logger:      logger,
		RedisHost:   rHost,
		RedisPort:   rPort,
		RedisPass:   rPass,
		ClusterName: clusterName,
		ClusterUid:  clusterUid,
		NodeName:    nodeName,
	}
}

func (op *opsrampk8sattributesprocessor) Start(_ context.Context, _ component.Host) error {
	op.logger.Info("ops k8s attr processor start", zap.Any("redisHost", op.RedisHost), zap.Any("redisPort", op.RedisPort), zap.Any("redisPass", op.RedisPass))
	op.rc = redis.NewClient(op.logger, op.RedisHost, op.RedisPort, op.RedisPass)
	//if !op.rc.Connected {
	//	Take this from k8sattributesprocessor kp.telemetrySettings.ReportStatus(component.NewFatalErrorEvent(err))
	//}
	return nil
}

func (op *opsrampk8sattributesprocessor) Shutdown(context.Context) error {

	return nil
}

// processTraces process traces and add k8s metadata using resource IP or incoming IP as pod origin.
func (op *opsrampk8sattributesprocessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		op.processResource(ctx, rss.At(i).Resource())
	}

	return td, nil
}

// processMetrics process metrics and add k8s metadata using resource IP, hostname or incoming IP as pod origin.
func (op *opsrampk8sattributesprocessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rm := md.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		op.processResource(ctx, rm.At(i).Resource())
	}

	return md, nil
}

// processLogs process logs and add k8s metadata using resource IP, hostname or incoming IP as pod origin.
func (op *opsrampk8sattributesprocessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	rl := ld.ResourceLogs()
	for i := 0; i < rl.Len(); i++ {
		op.processResource(ctx, rl.At(i).Resource())
	}

	return ld, nil
}

func (op *opsrampk8sattributesprocessor) GetResourceUuidUsingPodMoid(ctx context.Context, resource pcommon.Resource) (resourceUuid string) {
	var namespace, podname, rsname, dsname, ssname pcommon.Value
	var found bool

	if namespace, found = resource.Attributes().Get("k8s.namespace.name"); !found {
		return
	}
	if podname, found = resource.Attributes().Get("k8s.pod.name"); !found {
		return
	}

	podMoid := moid.NewMoid(op.ClusterName).WithNamespaceName(namespace.Str()).WithPodName(podname.Str())

	if rsname, found = resource.Attributes().Get("k8s.replicaset.name"); found {
		podMoid.WithReplicasetName(rsname.Str())
	} else if dsname, found = resource.Attributes().Get("k8s.daemonset.name"); found {
		podMoid.WithDaemonsetName(dsname.Str())
	} else if ssname, found = resource.Attributes().Get("k8s.statefulset.name"); found {
		podMoid.WithStatefulsetName(ssname.Str())
	}

	podMoidKey := podMoid.PodMoid()

	resourceUuid = op.rc.GetValueInString(ctx, podMoidKey)
	op.logger.Debug("redis KV ", zap.Any("key", podMoidKey), zap.Any("value", resourceUuid))
	return
}

func (op *opsrampk8sattributesprocessor) GetResourceUuidUsingResourceNodeMoid(ctx context.Context, resource pcommon.Resource) (resourceUuid string) {
	var nodename pcommon.Value
	var found bool
	if nodename, found = resource.Attributes().Get("k8s.node.name"); !found {
		return
	}

	nodeMoidKey := moid.NewMoid(op.ClusterName).WithNodeName(nodename.Str()).NodeMoid()

	resourceUuid = op.rc.GetValueInString(ctx, nodeMoidKey)
	op.logger.Debug("redis KV ", zap.Any("key", nodeMoidKey), zap.Any("value", resourceUuid))
	return
}

func (op *opsrampk8sattributesprocessor) GetResourceUuidUsingCurrentNodeMoid(ctx context.Context, resource pcommon.Resource) (resourceUuid string) {

	nodeMoidKey := moid.NewMoid(op.ClusterName).WithNodeName(op.NodeName).NodeMoid()

	resourceUuid = op.rc.GetValueInString(ctx, nodeMoidKey)
	op.logger.Debug("redis KV ", zap.Any("key", nodeMoidKey), zap.Any("value", resourceUuid))
	return
}

func (op *opsrampk8sattributesprocessor) GetResourceUuidUsingClusterMoid(ctx context.Context, resource pcommon.Resource) (resourceUuid string) {

	nodeMoidKey := moid.NewMoid(op.ClusterName).WithNodeName(op.NodeName).NodeMoid()

	resourceUuid = op.rc.GetValueInString(ctx, nodeMoidKey)
	op.logger.Debug("redis KV ", zap.Any("key", nodeMoidKey), zap.Any("value", resourceUuid))
	return
}

// processResource adds Pod metadata tags to resource based on pod association configuration
func (op *opsrampk8sattributesprocessor) processResource(ctx context.Context, resource pcommon.Resource) {
	var found bool
	var resourceUuid string

	resource.Attributes().PutStr("opsramp.k8s.cluster.name", op.ClusterName)
	resource.Attributes().PutStr("opsramp.k8s.cluster.uid", op.ClusterUid)
	if _, found = resource.Attributes().Get("k8s.pod.uid"); found {
		if resourceUuid = op.GetResourceUuidUsingPodMoid(ctx, resource); resourceUuid == "" {
			if resourceUuid = op.GetResourceUuidUsingResourceNodeMoid(ctx, resource); resourceUuid == "" {
				if resourceUuid = op.GetResourceUuidUsingCurrentNodeMoid(ctx, resource); resourceUuid == "" {
					resourceUuid = op.GetResourceUuidUsingClusterMoid(ctx, resource)
				}
			}
		}
	} else if _, found = resource.Attributes().Get("k8s.node.name"); found {
		if resourceUuid = op.GetResourceUuidUsingResourceNodeMoid(ctx, resource); resourceUuid == "" {
			if resourceUuid = op.GetResourceUuidUsingCurrentNodeMoid(ctx, resource); resourceUuid == "" {
				resourceUuid = op.GetResourceUuidUsingClusterMoid(ctx, resource)
			}
		}
	} else {
		if resourceUuid = op.GetResourceUuidUsingCurrentNodeMoid(ctx, resource); resourceUuid == "" {
			resourceUuid = op.GetResourceUuidUsingClusterMoid(ctx, resource)
		}
	}

	if resourceUuid != "" {
		resource.Attributes().PutStr("opsramp.resource.uuid", resourceUuid)
	}
}
