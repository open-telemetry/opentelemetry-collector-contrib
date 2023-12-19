// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient

import (
	"errors"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var transformFunc = func(v any) (any, error) {
	return v, nil
}

var transformFuncWithError = func(v any) (any, error) {
	return v, errors.New("an error")
}

func TestResync(t *testing.T) {
	o := NewObjStore(transformFunc, zap.NewNop())
	assert.Nil(t, o.Resync())
}

func TestGet(t *testing.T) {
	o := NewObjStore(transformFunc, zap.NewNop())
	item, exists, err := o.Get("a")
	assert.Nil(t, item)
	assert.False(t, exists)
	assert.Nil(t, err)
}

func TestGetByKey(t *testing.T) {
	o := NewObjStore(transformFunc, zap.NewNop())
	item, exists, err := o.GetByKey("a")
	assert.Nil(t, item)
	assert.False(t, exists)
	assert.Nil(t, err)
}

func TestGetListKeys(t *testing.T) {
	o := NewObjStore(transformFunc, zap.NewNop())
	o.objs = map[types.UID]any{
		"20036b33-cb03-489b-b778-e516b4dae519": "a",
		"7966452b-d896-4f5b-84a1-afbd4febc366": "b",
		"55f4f8dd-c4ae-4c18-947c-c0880bb0e05e": "c",
	}
	keys := o.ListKeys()
	sort.Strings(keys)
	expected := []string{
		"20036b33-cb03-489b-b778-e516b4dae519",
		"7966452b-d896-4f5b-84a1-afbd4febc366",
		"55f4f8dd-c4ae-4c18-947c-c0880bb0e05e",
	}
	sort.Strings(expected)
	assert.Equal(t, expected, keys)
}

func TestGetList(t *testing.T) {
	o := NewObjStore(transformFunc, zap.NewNop())
	o.objs = map[types.UID]any{
		"20036b33-cb03-489b-b778-e516b4dae519": "a",
	}
	val := o.List()
	assert.Equal(t, 1, len(val))
	expected := o.objs["20036b33-cb03-489b-b778-e516b4dae519"]
	assert.Equal(t, expected, val[0])
}

func TestDelete(t *testing.T) {
	o := NewObjStore(transformFunc, zap.NewNop())
	err := o.Delete(nil)
	assert.NotNil(t, err)

	o.objs = map[types.UID]any{
		"bc5f5839-f62e-44b9-a79e-af250d92dcb1": &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				UID:       "bc5f5839-f62e-44b9-a79e-af250d92dcb1",
				Name:      "kube-proxy-csm88",
				Namespace: "kube-system",
				SelfLink:  "/api/v1/namespaces/kube-system/pods/kube-proxy-csm88",
			},
			Status: v1.PodStatus{
				Phase: "Running",
			},
		},
		"75ab40d2-552a-4c05-82c9-0ddcb3008657": &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				UID:       "75ab40d2-552a-4c05-82c9-0ddcb3008657",
				Name:      "coredns-7554568866-26jdf",
				Namespace: "kube-system",
				SelfLink:  "/api/v1/namespaces/kube-system/pods/coredns-7554568866-26jdf",
			},
			Status: v1.PodStatus{
				Phase: "Running",
			},
		},
	}

	obj := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "bc5f5839-f62e-44b9-a79e-af250d92dcb1",
			Name:      "kube-proxy-csm88",
			Namespace: "kube-system",
			SelfLink:  "/api/v1/namespaces/kube-system/pods/kube-proxy-csm88",
		},
		Status: v1.PodStatus{
			Phase: "Running",
		},
	}
	err = o.Delete(obj)
	assert.Nil(t, err)
	assert.True(t, o.refreshed)

	keys := o.ListKeys()
	assert.Equal(t, 1, len(keys))
	assert.Equal(t, "75ab40d2-552a-4c05-82c9-0ddcb3008657", keys[0])
}

func TestAdd(t *testing.T) {
	o := NewObjStore(transformFunc, zap.NewNop())
	err := o.Add(nil)
	assert.NotNil(t, err)

	o = NewObjStore(transformFuncWithError, zap.NewNop())
	obj := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "bc5f5839-f62e-44b9-a79e-af250d92dcb1",
			Name:      "kube-proxy-csm88",
			Namespace: "kube-system",
			SelfLink:  "/api/v1/namespaces/kube-system/pods/kube-proxy-csm88",
		},
		Status: v1.PodStatus{
			Phase: "Running",
		},
	}
	err = o.Add(obj)
	assert.NotNil(t, err)
}

func TestUpdate(t *testing.T) {
	o := NewObjStore(transformFunc, zap.NewNop())
	o.objs = map[types.UID]any{
		"bc5f5839-f62e-44b9-a79e-af250d92dcb1": &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				UID:       "bc5f5839-f62e-44b9-a79e-af250d92dcb1",
				Name:      "kube-proxy-csm88",
				Namespace: "kube-system",
				SelfLink:  "/api/v1/namespaces/kube-system/pods/kube-proxy-csm88",
			},
			Status: v1.PodStatus{
				Phase: "Running",
			},
		},
	}
	updatedObj := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "bc5f5839-f62e-44b9-a79e-af250d92dcb1",
			Name:      "kube-proxy-csm88",
			Namespace: "kube-system",
			SelfLink:  "/api/v1/namespaces/kube-system/pods/kube-proxy-csm88",
		},
		Status: v1.PodStatus{
			Phase: "Stopped",
		},
	}
	err := o.Update(updatedObj)
	assert.Nil(t, err)

	keys := o.ListKeys()
	assert.Equal(t, 1, len(keys))
	assert.Equal(t, "bc5f5839-f62e-44b9-a79e-af250d92dcb1", keys[0])

	values := o.List()
	assert.Equal(t, 1, len(values))
	assert.Equal(t, updatedObj, values[0])
}

func TestReplace(t *testing.T) {
	o := NewObjStore(transformFuncWithError, zap.NewNop())
	objArray := []any{
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				UID:       "bc5f5839-f62e-44b9-a79e-af250d92dcb1",
				Name:      "kube-proxy-csm88",
				Namespace: "kube-system",
				SelfLink:  "/api/v1/namespaces/kube-system/pods/kube-proxy-csm88",
			},
			Status: v1.PodStatus{
				Phase: "Running",
			},
		},
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				UID:       "75ab40d2-552a-4c05-82c9-0ddcb3008657",
				Name:      "coredns-7554568866-26jdf",
				Namespace: "kube-system",
				SelfLink:  "/api/v1/namespaces/kube-system/pods/coredns-7554568866-26jdf",
			},
			Status: v1.PodStatus{
				Phase: "Running",
			},
		},
	}
	err := o.Replace(objArray, "")
	assert.NotNil(t, err)
}
