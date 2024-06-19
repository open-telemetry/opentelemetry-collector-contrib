package rolebinding

import (
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTransform(t *testing.T) {
	originalRB := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-rb",
			UID:  "my-rb-uid",
		},
	}
	wantRB := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-rb",
			UID:  "my-rb-uid",
		},
	}
	assert.Equal(t, wantRB, Transform(originalRB))
}
