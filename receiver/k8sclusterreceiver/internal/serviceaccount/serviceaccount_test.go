package serviceaccount

import (
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestTransform(t *testing.T) {
	originalSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-sa",
			UID:  "my-sa-uid",
		},
	}
	wantSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-sa",
			UID:  "my-sa-uid",
		},
	}
	assert.Equal(t, wantSA, Transform(originalSA))
}
