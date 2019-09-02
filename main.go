package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/knative/pkg/apis/duck"
	"github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

var cmd = &cobra.Command{
	Use:   "kawait",
	Short: "Waits for Kubernetes resources to become ready",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Awaiting %q\n", args)

		tif, mapper := connectToServer()

		for _, item := range args {
			gvr, ns, name, err := ParseGVRAndName(item, mapper)
			if err != nil {
				fmt.Printf("Error on %q: %v\n", item, err)
				continue
			}
			_, lister, err := tif.Get(gvr)
			rc := &ReadyChecker{
				GVR:       gvr,
				Namespace: ns,
				Name:      name,
				Lister:    lister,
			}
			fmt.Printf("%q is ready=%t\n", rc, rc.IsReady())
		}
	},
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func connectToServer() (*duck.TypedInformerFactory, meta.RESTMapper) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	discovery := memory.NewMemCacheClient(kubernetes.NewForConfigOrDie(config).Discovery())
	apiResources, err := restmapper.GetAPIGroupResources(discovery)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(apiResources)

	tif := &duck.TypedInformerFactory{
		Client:       dynamicClient,
		Type:         &v1alpha1.KResource{},
		ResyncPeriod: 1 * time.Minute,
	}

	return tif, mapper
}

// ReadyChecker stores the information needed to determine if a Kubernetes
// resource is ready or succeeded yet.
type ReadyChecker struct {
	GVR       schema.GroupVersionResource
	Namespace string
	Name      string
	Lister    cache.GenericLister
}

// String represents the resource that a ReadyChecker is watching as a string.
func (r *ReadyChecker) String() string {
	return fmt.Sprintf("%s/%s, a %s", r.Namespace, r.Name, r.GVR)
}

// IsReady returns whether the selected resource is in a terminal state.
// (Ready or Succeeded Condition Type with a Status of True.)
func (r *ReadyChecker) IsReady() bool {
	untyped, err := r.Lister.ByNamespace(r.Namespace).Get(r.Name)
	if err != nil {
		fmt.Printf("Failed to fetch %s\n", r)
		return false
	}
	obj, ok := untyped.(*v1alpha1.KResource)
	if !ok {
		fmt.Printf("Could not find `status.Conditions` for %s\n", r)
		return false
	}
	for _, n := range []v1alpha1.ConditionType{v1alpha1.ConditionSucceeded, v1alpha1.ConditionReady} {
		c := obj.Status.GetCondition(n)
		if c == nil {
			continue
		}
		if c.IsUnknown() {
			fmt.Printf("%s has not finished yet\n", r)
			continue
		}
		fmt.Printf("%s is %s: %t\n", r, n, c.IsTrue())
		return c.IsTrue()
	}
	fmt.Printf("Could not find terminal condition on %s\n", r)
	return false
}

// ParseGVRAndName parses a string like "Deployments.extensions:foo" or
// "Deployments.extensions:namespace/foo" and returns a GroupVersion,
// namespace, and resource name.
func ParseGVRAndName(s string, mapper meta.RESTMapper) (schema.GroupVersionResource, string, string, error) {
	parts := strings.Split(s, ":")
	gvrName, nsName := parts[0], parts[1]
	namespace, name := "default", nsName
	parts = strings.Split(nsName, "/")
	if len(parts) > 1 {
		namespace, name = parts[0], parts[1]
	}
	gvr, err := determineREST(gvrName, mapper)
	return gvr, namespace, name, err
}

// determineREST determines the mapping of the supplied name to an apiserver
// REST interface. This is based on the `mappingFor` code in
// k8s.io/cli-runtime/pkg/resource/builder.go
func determineREST(resourceName string, mapper meta.RESTMapper) (schema.GroupVersionResource, error) {
	fullGVR, groupResource := schema.ParseResourceArg(resourceName)

	var gvk schema.GroupVersionKind
	if fullGVR != nil {
		gvk, _ = mapper.KindFor(*fullGVR)
	}
	if gvk.Empty() {
		gvk, _ = mapper.KindFor(groupResource.WithVersion(""))
	}
	if !gvk.Empty() {
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		return mapping.Resource, err
	}

	fullGVK, groupKind := schema.ParseKindArg(resourceName)
	if fullGVK == nil {
		gvk = groupKind.WithVersion("")
	} else {
		gvk = *fullGVK
	}
	if !gvk.Empty() {
		if mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version); err == nil {
			return mapping.Resource, nil
		}
	}

	mapping, err := mapper.RESTMapping(groupKind, "")
	if err != nil {
		// See builder.go 735 for comment; this is basically the last path we could take.
		if meta.IsNoMatchError(err) {
			return schema.GroupVersionResource{}, fmt.Errorf("the server doesn't have a resource type %q", groupResource.Resource)
		}
		return schema.GroupVersionResource{}, err
	}
	return mapping.Resource, nil
}
