package moid

const (
	CLUSTER     = "Cluster"
	NAMESPACE   = "Namespace"
	NODE        = "Node"
	SERVICE     = "Service"
	DEPLOYMENT  = "Deployment"
	DAEMONSET   = "DaemonSet"
	REPLICASET  = "ReplicaSet"
	STATEFULSET = "StatefulSet"
	POD         = "Pod"
	SEPARATOR   = "_"
)

type Moid struct {
	clusterName     string
	clusterUuid     string
	namespaceName   string
	nodeName        string
	serviceName     string
	deploymentName  string
	replicasetName  string
	daemonsetName   string
	statefulsetName string
	podName         string
}

func NewMoid(clusterName string) *Moid {
	return &Moid{clusterName: clusterName}
}

func (m *Moid) WithClusterUuid(clusterUuid string) *Moid {
	m.clusterUuid = clusterUuid
	return m
}

func (m *Moid) WithNamespaceName(namespaceName string) *Moid {
	m.namespaceName = namespaceName
	return m
}

func (m *Moid) WithNodeName(nodeName string) *Moid {
	m.nodeName = nodeName
	return m
}

func (m *Moid) WithServiceName(serviceName string) *Moid {
	m.serviceName = serviceName
	return m
}

func (m *Moid) WithDeploymentName(deploymentName string) *Moid {
	m.deploymentName = deploymentName
	return m
}

func (m *Moid) WithReplicasetName(replicasetName string) *Moid {
	m.replicasetName = replicasetName
	return m
}

func (m *Moid) WithDaemonsetName(daemonsetName string) *Moid {
	m.daemonsetName = daemonsetName
	return m
}

func (m *Moid) WithStatefulsetName(statefulsetName string) *Moid {
	m.statefulsetName = statefulsetName
	return m
}

func (m *Moid) WithPodName(podName string) *Moid {
	m.podName = podName
	return m
}

func (m *Moid) PodMoid() (moid string) {
	moid = m.clusterName + SEPARATOR + m.namespaceName + SEPARATOR

	if m.replicasetName != "" {
		moid += REPLICASET + SEPARATOR + m.replicasetName + SEPARATOR
	} else if m.daemonsetName != "" {
		moid += DAEMONSET + SEPARATOR + m.daemonsetName + SEPARATOR
	} else if m.statefulsetName != "" {
		moid += STATEFULSET + SEPARATOR + m.statefulsetName + SEPARATOR
	}

	moid += POD + SEPARATOR + m.podName
	return
}

func (m *Moid) NodeMoid() (moid string) {
	moid = m.clusterName + SEPARATOR + NODE + SEPARATOR + m.nodeName
	return
}

func (m *Moid) NamespaceMoid() (moid string) {
	moid = m.clusterName + SEPARATOR + NAMESPACE + SEPARATOR + m.namespaceName
	return
}

func (m *Moid) DaemonSetMoid() (moid string) {
	moid = m.clusterName + SEPARATOR + m.namespaceName + SEPARATOR

	moid += DAEMONSET + SEPARATOR + m.daemonsetName
	return
}

func (m *Moid) ReplicaSetMoid() (moid string) {
	moid = m.clusterName + SEPARATOR + m.namespaceName + SEPARATOR

	moid += REPLICASET + SEPARATOR + m.replicasetName
	return
}

func (m *Moid) StatefulSetMoid() (moid string) {
	moid = m.clusterName + SEPARATOR + m.namespaceName + SEPARATOR

	moid += STATEFULSET + SEPARATOR + m.statefulsetName
	return
}

func (m *Moid) DeploymentMoid() (moid string) {
	moid = m.clusterName + SEPARATOR + m.namespaceName + SEPARATOR

	moid += DEPLOYMENT + SEPARATOR + m.deploymentName
	return
}

func (m *Moid) ServiceMoid() (moid string) {
	moid = m.clusterName + SEPARATOR + m.namespaceName + SEPARATOR

	moid += SERVICE + SEPARATOR + m.serviceName
	return
}

func (m *Moid) ClusterMoid() (moid string) {
	moid = m.clusterName + SEPARATOR + CLUSTER + SEPARATOR + m.clusterUuid
	return
}
