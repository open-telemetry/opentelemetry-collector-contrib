package collection

// func getMetricsForService(svc *corev1.Service) pmetric.ResourceMetricsSlice {
// 	metrics := []*metricspb.Metric{
// 		{
// 			MetricDescriptor: &metricspb.MetricDescriptor{
// 				Name:        "k8s.service.port_count",
// 				Description: "The number of ports in the service",
// 				Type:        metricspb.MetricDescriptor_GAUGE_INT64,
// 			},
// 			Timeseries: []*metricspb.TimeSeries{
// 				utils.GetInt64TimeSeries(int64(len(svc.Spec.Ports))),
// 			},
// 		},
// 	}

// 	slice := pmetric.NewResourceMetricsSlice([]*resourceMetrics{
// 		// 	{
// 		// 		resource: getResourceForService(svc),
// 		// 		metrics:  metrics,
// 		// 	},
// 		// })
// 	// return []*resourceMetrics{
// 	// 	{
// 	// 		resource: getResourceForService(svc),
// 	// 		metrics:  metrics,
// 	// 	},
// 	// }
// 	return slice
// }

// func getResourceForService(svc *corev1.Service) *resourcepb.Resource {
// 	return &resourcepb.Resource{
// 		Type: k8sType,
// 		Labels: map[string]string{
// 			"k8s.service.uid":                     string(svc.UID),
// 			conventions.AttributeServiceNamespace: svc.ObjectMeta.Namespace,
// 			conventions.AttributeServiceName:      svc.ObjectMeta.Name,
// 			"k8s.service.cluster_ip":              svc.Spec.ClusterIP,
// 			"k8s.service.type":                    string(svc.Spec.Type),
// 			"k8s.cluster.name":                    "unknown",
// 		},
// 	}
// }
