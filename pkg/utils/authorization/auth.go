package authorization

import (
	"context"
	"os/exec"
	"strings"
	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CollectTokenFromOC() (string, error) {
	cmd := exec.Command("oc", "whoami", "-t")
	o, err := cmd.Output()

	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(o)), nil
}

func FindTokenRequestURI(cl client.Client) (string, error) {
	route := routev1.Route{}
	if err := cl.Get(context.TODO(), types.NamespacedName{
		Namespace: "openshift-authentication",
		Name:      "oauth-openshift",
	}, &route); err != nil {
		return "", err
	}
	return "https://" + route.Spec.Host + "/oauth/token/display", nil
}
