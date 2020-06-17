package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/util/retry"
)

// GetNodes returns all nodes of this cluster.
func GetNodes(client clientset.Interface) ([]v1.Node, error) {
	nodes, err := client.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %s", err)
	}
	return nodes.Items, nil
}

// UpdateNodeWithBackoff update a given node with a given backoff retry.
func UpdateNodeWithBackoff(client clientset.Interface, node v1.Node, backoff wait.Backoff) error {
	return retry.RetryOnConflict(backoff, func() error {
		_, err := client.CoreV1().Nodes().Update(context.Background(), &node, metav1.UpdateOptions{})
		return err
	})
}

// NodeNamesOfNodes returns the node names of the nodess
func NodeNamesOfNodes(nodes []v1.Node) string {
	var nn []string
	for _, n := range nodes {
		nn = append(nn, n.Name)
	}
	return strings.Join(nn, ",")
}
