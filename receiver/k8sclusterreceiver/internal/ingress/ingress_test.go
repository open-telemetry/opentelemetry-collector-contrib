package ingress

import (
	"testing"

	"github.com/stretchr/testify/assert"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTransform(t *testing.T) {
	originalI := &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-ingress",
			UID:  "my-ingress-uid",
		},
	}
	wantI := &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-ingress",
			UID:  "my-ingress-uid",
		},
	}
	assert.Equal(t, wantI, Transform(originalI))
}
