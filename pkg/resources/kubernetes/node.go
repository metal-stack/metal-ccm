package kubernetes

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/util/retry"
)

// getNodes returns all nodes of this cluster.
func GetNodes(client clientset.Interface) ([]v1.Node, error) {
	nodes, err := client.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %s", err)
	}
	return nodes.Items, nil
}

// updateNodes updates given nodes.
func UpdateNodeWithBackoff(client clientset.Interface, node *v1.Node, backoff wait.Backoff) error {
	return retry.RetryOnConflict(backoff, func() error {
		_, err := client.CoreV1().Nodes().Update(node)
		return err
	})
}

func NodeNamesOfNodes(nodes []*v1.Node) string {
	var nn []string
	for _, n := range nodes {
		nn = append(nn, n.Name)
	}
	return strings.Join(nn, ",")
}
