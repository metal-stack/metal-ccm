package metal

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// upsertConfigMap inserts or updates given config map.
func (r *ResourcesController) upsertConfigMap(namespace, name string, configMap map[string]string) error {
	err := retry.RetryOnConflict(updateNodeSpecBackoff, func() error {
		cmi := r.kclient.CoreV1().ConfigMaps(namespace)
		cm, err := cmi.Get(name, metav1.GetOptions{})
		if err == nil {
			cm.Data = configMap

			_, err = cmi.Update(cm)
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

		_, err = cmi.Create(cm)
		return err
	})

	return err
}
