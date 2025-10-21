package starttimeattribute

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/selection"
)

type podIdentifier struct {
	Value string
	Type  idType
}

type idType byte

const (
	podIP idType = iota
	podName
	podUID
)

func (i idType) String() string {
	switch i {
	case podIP:
		return "podIP"
	case podName:
		return "podName"
	case podUID:
		return "podUID"
	default:
		return ""
	}
}

type podClient interface {
	GetPodStartTime(ctx context.Context, podID podIdentifier) time.Time
}

type informerFilter struct {
	node         string
	namespace    string
	LabelFilters []labelFilter
}

type labelFilter struct {
	Key   string
	Value string
	Op    selection.Operator
}

func toInformerFilter(cfg AttributesFilterConfig) informerFilter {
	f := informerFilter{
		node:      cfg.Node,
		namespace: cfg.Namespace,
	}
	for _, label := range cfg.Labels {
		f.LabelFilters = append(f.LabelFilters, labelFilter{
			Key:   label.Key,
			Value: label.Value,
			Op:    selection.Operator(label.Op),
		})
	}
	return f
}
