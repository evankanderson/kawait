package readychecker

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis/duck/v1alpha1"
)

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
