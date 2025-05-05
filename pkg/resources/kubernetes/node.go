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
func GetNodes(ctx context.Context, client clientset.Interface) ([]v1.Node, error) {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	return nodes.Items, nil
}

// UpdateNodeLabelsWithBackoff updates labels on a given node with a given backoff retry.
func UpdateNodeLabelsWithBackoff(ctx context.Context, client clientset.Interface, nodeName string, labels map[string]string, backoff wait.Backoff) error {
	return retry.RetryOnConflict(backoff, func() error {

		node, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		for key, value := range labels {
			node.Labels[key] = sanitizeLabelValue(value)
		}

		_, err = client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
		return err
	})
}

// Labels must match: (([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?
func sanitizeLabelValue(value string) string {
	const whitelist = "-_.0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	var str strings.Builder
	str.Grow(len(value))

	for _, ch := range value {
		if strings.ContainsRune(whitelist, ch) {
			_, _ = str.WriteRune(ch)
		} else {
			_, _ = str.WriteRune('_')
		}
	}
	return str.String()
}

// UpdateNodeAnnotationsWithBackoff updates labels on a given node with a given backoff retry.
func UpdateNodeAnnotationsWithBackoff(ctx context.Context, client clientset.Interface, nodeName string, annotations map[string]string, backoff wait.Backoff) error {
	return retry.RetryOnConflict(backoff, func() error {

		node, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		for key, value := range annotations {
			node.Annotations[key] = value
		}

		_, err = client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
		return err
	})
}

// NodeNamesOfNodes returns the node names of the nodes
func NodeNamesOfNodes(nodes []v1.Node) string {
	var nn []string
	for _, n := range nodes {
		nn = append(nn, n.Name)
	}
	return strings.Join(nn, ",")
}

func NodeAddress(node v1.Node) (string, error) {
	for _, a := range node.Status.Addresses {
		if a.Type == v1.NodeInternalIP {
			return a.Address, nil
		}
	}
	return "", fmt.Errorf("unable to determine node address")
}
