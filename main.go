package main

import (
	"flag"
	"io"
	"os"

	"k8s.io/apimachinery/pkg/util/wait"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	"k8s.io/cloud-provider/options"
	"k8s.io/cloud-provider/names"


	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"

	"k8s.io/component-base/cli"
	cliflag "k8s.io/component-base/cli/flag"

	_ "k8s.io/component-base/metrics/prometheus/clientgo" // load all the prometheus client-go plugins
	_ "k8s.io/component-base/metrics/prometheus/version"  // for version metric registration
	"k8s.io/klog/v2"

	"github.com/metal-stack/metal-ccm/metal"
	"github.com/metal-stack/metal-ccm/pkg/resources/constants"
	"github.com/metal-stack/v"
	"github.com/spf13/pflag"
)

func main() {
	opts, err := options.NewCloudControllerManagerOptions()
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}
	opts.KubeCloudShared.CloudProvider.Name = constants.ProviderName

	controllerInitializers := app.DefaultInitFuncConstructors
	fss := cliflag.NamedFlagSets{
		NormalizeNameFunc: cliflag.WordSepNormalizeFunc,
	}

	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	command := app.NewCloudControllerManagerCommand(opts, cloudInitializer, controllerInitializers, names.CCMControllerAliases(), fss, wait.NeverStop)

	klog.Infof("starting version %s", v.V.String())
	code := cli.Run(command)
	os.Exit(code)
}
func cloudInitializer(config *cloudcontrollerconfig.CompletedConfig) cloudprovider.Interface {
	cloudConfig := config.ComponentConfig.KubeCloudShared.CloudProvider

	cloudprovider.RegisterCloudProvider(constants.ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		return metal.NewCloud(config)
	})
	// initialize cloud provider with the cloud provider name and config file provided
	cloud, err := cloudprovider.InitCloudProvider(cloudConfig.Name, cloudConfig.CloudConfigFile)
	if err != nil {
		klog.Fatalf("cloud provider could not be initialized: %v", err)
	}
	if cloud == nil {
		klog.Fatal("cloud provider is nil")
	}

	if !cloud.HasClusterID() {
		if config.ComponentConfig.KubeCloudShared.AllowUntaggedCloud {
			klog.Warning("detected a cluster without a ClusterID.  A ClusterID will be required in the future.  Please tag your cluster to avoid any future issues")
		} else {
			klog.Fatal("no ClusterID found.  A ClusterID is required for the cloud provider to function properly.  This check can be bypassed by setting the allow-untagged-cloud option")
		}
	}

	return cloud
}
