package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/knative/pkg/apis/duck"
	"github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	//	"k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

var cmd = &cobra.Command{
	Use:   "kawait",
	Short: "Waits for Kubernetes resources to become ready",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Awaiting %q\n", args)
/*
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		
		config, err := kubeConfig.ClientConfig()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		*/
		config, err := clientcmd.BuildConfigFromFlags("", "c:\\users\\evank\\.kube\\config")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		dynamicClient, err := dynamic.NewForConfig(config)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		//		finder := util.NewCRDFinder(util.CRDFromDynamic(dynamicClient))

		tif := &duck.TypedInformerFactory{
			Client: dynamicClient,
			Type:   &v1alpha1.KResource{},
			ResyncPeriod: 1 * time.Minute,
		}

		for _, item := range args {
			gvr, name := ParseGVRAndName(item)
			/*
				gkName, name := parts[0], parts[1]
				gk := schema.ParseGroupKind(gkName)

				if ok, err := finder.HasCRD(gk); ok != true || err != nil {
					fmt.Printf("Unable to find an object of type %q (%v)\n", gkName, gk)
					continue
				}
				// todo gk.WithVersion using version from CRD
			*/
			fmt.Printf("Looking for %q, a %v", name, gvr)
			_, lister, err := tif.Get(gvr)
			fmt.Println("Got lister")
			untyped, err := lister.Get(name)
			fmt.Println("Read from lister")
			if err != nil {
				fmt.Printf("Failed to fetch %q (%q): %v\n", item, gvr, err)
				continue
			}
			obj := untyped.(*v1alpha1.KResource)

			fmt.Printf("Got it! %v", obj.Status.Conditions)
		}
	},
}

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func ParseGVRAndName(s string) (schema.GroupVersionResource, string) {
	parts := strings.Split(s, ":")
	gvrName, name := parts[0], parts[1]
	parts = strings.Split(gvrName, "/")
	grName, version := parts[0], parts[1]
	gr := schema.ParseGroupResource(grName)
	return gr.WithVersion(version), name
}
