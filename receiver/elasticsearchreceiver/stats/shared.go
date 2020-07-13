package stats

import (
	"fmt"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/utils"
	"go.uber.org/zap"
)

type sharedInfo struct {
	nodeIsCurrentMaster  bool
	defaultDimensions    map[string]string
	nodeMetricDimensions map[string]string
	lock                 sync.RWMutex
	logger               *zap.Logger
}

func (sinfo *sharedInfo) fetchNodeAndClusterMetadata(esClient ESStatsHTTPClient, configuredClusterName string) error {
	// Collect info about master for the cluster
	masterInfoOutput, err := esClient.GetMasterNodeInfo()
	if err != nil {
		return fmt.Errorf("failed to GET master node info: %v", err)
	}

	// Collect node info
	nodeInfoOutput, err := esClient.GetNodeInfo()
	if err != nil {
		return fmt.Errorf("failed to GET node info: %v", err)
	}

	// Hold the lock while updating info shared between different fetchers
	sinfo.lock.Lock()
	defer sinfo.lock.Unlock()

	nodeDimensions, err := prepareNodeMetricsDimensions(nodeInfoOutput.Nodes)
	if err != nil {
		return fmt.Errorf("failed to prepare node dimensions: %v", err)
	}

	sinfo.nodeIsCurrentMaster, err = isCurrentMaster(nodeDimensions["node_id"], masterInfoOutput)
	if err != nil {
		sinfo.logger.Warn(err.Error())
		sinfo.nodeIsCurrentMaster = false
	}

	clusterName := nodeInfoOutput.ClusterName
	sinfo.defaultDimensions, err = prepareDefaultDimensions(configuredClusterName, clusterName)

	if err != nil {
		return fmt.Errorf("failed to prepare plugin_instance dimension: %v", err)
	}

	sinfo.nodeMetricDimensions = utils.MergeStringMaps(sinfo.defaultDimensions, nodeDimensions)

	return nil
}

// Returns all fields of a shared info object
func (sinfo *sharedInfo) getAllSharedInfo() (map[string]string, map[string]string, bool) {
	sinfo.lock.RLock()
	defer sinfo.lock.RUnlock()

	return sinfo.defaultDimensions, sinfo.nodeMetricDimensions, sinfo.nodeIsCurrentMaster
}
