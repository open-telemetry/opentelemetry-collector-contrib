// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sclient

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	cacheTTL = 10 * time.Minute
)

var client *K8sClient
var Get func() *K8sClient

func get() *K8sClient {
	if !client.inited {
		client.init()
	}
	return client
}

type K8sClient struct {
	sync.Mutex
	inited bool

	ClientSet *kubernetes.Clientset

	Ep   EpClient
	Pod  PodClient
	Node NodeClient

	Job        JobClient
	ReplicaSet ReplicaSetClient
}

func (c *K8sClient) init() {
	c.Lock()
	defer c.Unlock()
	if c.inited {
		return
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("W! Cannot find in cluster config: %v", err)
		config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube/config"))
		if err != nil {
			log.Printf("E! Failed to build config: %v", err)
			return
		}
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("E! Failed to build ClientSet: %v", err)
		return
	}
	c.ClientSet = client
	c.Ep = new(epClient)
	c.Pod = new(podClient)
	c.Node = new(nodeClient)
	c.Job = new(jobClient)
	c.ReplicaSet = new(replicaSetClient)
	c.inited = true
}

func (c *K8sClient) shutdown() {
	c.Lock()
	defer c.Unlock()
	if !c.inited {
		return
	}
	if c.Ep != nil {
		c.Ep.Shutdown()
	}
	if c.Pod != nil {
		c.Pod.Shutdown()
	}
	if c.Node != nil {
		c.Node.Shutdown()
	}
	if c.Job != nil {
		c.Job.Shutdown()
	}
	if c.ReplicaSet != nil {
		c.ReplicaSet.Shutdown()
	}
	c.inited = false
}

func init() {
	client = new(K8sClient)
	Get = get
}
