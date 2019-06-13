package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/cmd/cloud-controller-manager/app"

	// NOTE: Importing all in-tree cloud-providers is not required when
	// implementing an out-of-tree cloud-provider.
	// _ "k8s.io/kubernetes/pkg/cloudprovider/providers"
	// _ "k8s.io/kubernetes/pkg/util/prometheusclientgo" // load all the prometheus client-go plugins
	// _ "k8s.io/kubernetes/pkg/version/prometheus"      // for version metric registration

	_ "github.com/metal-pod/metal-ccm/metal"
	"github.com/metal-pod/v"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	// Bogus parameter needed until https://github.com/kubernetes/kubernetes/issues/76205
	// gets resolved.
	flag.CommandLine.String("cloud-provider-gce-lb-src-cidrs", "", "NOT USED (workaround for https://github.com/kubernetes/kubernetes/issues/76205)")

	command := app.NewCloudControllerManagerCommand()

	// TODO: once we switch everything over to Cobra commands, we can go back to calling
	// utilflag.InitFlags() (by removing its pflag.Parse() call). For now, we have to set the
	// normalize func and add the go flag set by hand.
	// utilflag.InitFlags()

	logs.InitLogs()
	defer logs.FlushLogs()
	logger := logs.NewLogger("metal-ccm")
	logger.Printf(" starting version:%s", v.V)

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
