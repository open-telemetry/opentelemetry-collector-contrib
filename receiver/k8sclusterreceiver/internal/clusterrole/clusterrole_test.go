package clusterrole

import (
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTransform(t *testing.T) {
	originalCR := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-cr",
			UID:  "my-cr-uid",
		},
	}
	wantCR := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-cr",
			UID:  "my-cr-uid",
		},
	}
	assert.Equal(t, wantCR, Transform(originalCR))
}
