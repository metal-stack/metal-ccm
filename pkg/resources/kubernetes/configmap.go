package kubernetes

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ApplyConfigMap creates or updates given config map.
func ApplyConfigMap(client kubernetes.Interface, namespace, name string, configMap map[string]string) error {
	var (
		ctx = context.Background()
		cm  = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Annotations: map[string]string{
					// we enforce updating the metallb config every time such that
					// deleted service ips will be cleaned up from the config regularly.
					"cluster.metal-stack.io.metal-ccm/last-update-time": time.Now().String(),
				},
			},
			Data: configMap,
		}
	)

	_, err := client.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil && errors.IsNotFound(err) {
		_, err = client.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{})
		return err
	}

	return err
}
