package constants

const (
	MetalAPIUrlEnvVar = "METAL_API_URL"
	//nolint
	MetalAuthTokenEnvVar              = "METAL_AUTH_TOKEN"
	MetalAuthHMACEnvVar               = "METAL_AUTH_HMAC"
	MetalProjectIDEnvVar              = "METAL_PROJECT_ID"
	MetalPartitionIDEnvVar            = "METAL_PARTITION_ID"
	MetalClusterIDEnvVar              = "METAL_CLUSTER_ID"
	MetalDefaultExternalNetworkEnvVar = "METAL_DEFAULT_EXTERNAL_NETWORK_ID"
	MetalAdditionalNetworks           = "METAL_ADDITIONAL_NETWORKS"

	// MetalSSHPublicKeys slice of ssh public keys, separated by ,
	MetalSSHPublicKeys = "METAL_SSH_PUBLICKEYS"

	ProviderName = "metal"

	MetalLBSpecificAddressPool = "metallb.universe.tf/address-pool"

	IPPrefix = "metallb-"
)
