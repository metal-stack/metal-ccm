package constants

const (
	MetalAPIUrlEnvVar      = "METAL_API_URL"
	MetalAuthTokenEnvVar   = "METAL_AUTH_TOKEN"
	MetalAuthHMACEnvVar    = "METAL_AUTH_HMAC"
	MetalProjectIDEnvVar   = "METAL_PROJECT_ID"
	MetalPartitionIDEnvVar = "METAL_PARTITION_ID"

	ProviderName = "metal"

	IPCountServiceAnnotation = "machine.metal-pod.io/ip-count"
	ASNNodeAnnotation        = "machine.metal-pod.io/network.primary.asn"

	IPPrefix = "metallb-"
)
