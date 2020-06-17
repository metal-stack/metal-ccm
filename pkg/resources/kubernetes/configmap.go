package kubernetes

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Apply inserts or updates given config map.
func ApplyConfigMap(client kubernetes.Interface, namespace, name string, configMap map[string]string) error {
	ctx := context.Background()
	cmi := client.CoreV1().ConfigMaps(namespace)
	cm, err := cmi.Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		cm.Data = configMap

		_, err = cmi.Update(ctx, cm, metav1.UpdateOptions{})
		return err
	}

	cm = &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       name,
			Namespace:                  namespace,
			DeletionGracePeriodSeconds: nil,
			Labels:                     nil,
			Annotations:                nil,
		},
		Data: configMap,
	}

	_, err = cmi.Create(ctx, cm, metav1.CreateOptions{})
	return err
}
