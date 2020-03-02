package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/cmd/cloud-controller-manager/app"

	_ "github.com/metal-stack/metal-ccm/cmd"
	"github.com/metal-stack/v"
	"github.com/spf13/pflag"
)

const providerName = "metal"

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	command := app.NewCloudControllerManagerCommand()

	// Set static flags for which we know the values.
	command.Flags().VisitAll(func(fl *pflag.Flag) {
		var err error
		switch fl.Name {
		case "allow-untagged-cloud",
			// Untagged clouds must be enabled explicitly as they were once marked
			// deprecated. See
			// https://github.com/kubernetes/cloud-provider/issues/12 for an ongoing
			// discussion on whether that is to be changed or not.
			"authentication-skip-lookup":
			// Prevent reaching out to an authentication-related ConfigMap that
			// we do not need, and thus do not intend to create RBAC permissions
			// for. See also
			// https://github.com/digitalocean/digitalocean-cloud-controller-manager/issues/217
			// and https://github.com/kubernetes/cloud-provider/issues/29.
			err = fl.Value.Set("true")
		case "cloud-provider":
			// Specify the name we register our own cloud provider implementation
			// for.
			err = fl.Value.Set(providerName)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to set flag %q: %s\n", fl.Name, err)
			os.Exit(1)
		}
	})

	logs.InitLogs()
	defer logs.FlushLogs()
	logger := logs.NewLogger("metal-ccm ")
	logger.Printf("starting version %q", v.V)

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
