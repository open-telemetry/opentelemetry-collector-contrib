package role

import (
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestTransform(t *testing.T) {
	originalR := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-role",
			UID:  "my-role-uid",
		},
	}
	wantR := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-role",
			UID:  "my-role-uid",
		},
	}
	assert.Equal(t, wantR, Transform(originalR))
}
