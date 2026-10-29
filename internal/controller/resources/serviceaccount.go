package resources

import (
	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func RenderServiceAccount(fs *v1alpha1.ForgeService) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fs.Name + "-sa",
			Namespace: fs.Namespace,
			Labels:    labels(fs),
		},
		AutomountServiceAccountToken: ptr.To(false),
	}
}
