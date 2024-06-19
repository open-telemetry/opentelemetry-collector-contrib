package clusterrolebinding

import (
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTransform(t *testing.T) {
	originalCRB := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-crb",
			UID:  "my-crb-uid",
		},
	}
	wantCRB := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-crb",
			UID:  "my-crb-uid",
		},
	}
	assert.Equal(t, wantCRB, Transform(originalCRB))
}
