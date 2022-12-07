package auth

import (
	"context"
	"os/exec"
	"strings"

	cfg "github.com/codeready-toolchain/toolchain-e2e/setup/configuration"
	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetTokenRequestURI(cl client.Client) (string, error) {
	route := routev1.Route{}
	if err := cl.Get(context.TODO(), types.NamespacedName{
		Namespace: cfg.OauthNS,
		Name:      cfg.OauthName,
	}, &route); err != nil {
		return "", err
	}
	return "https://" + route.Spec.Host + "/oauth/token/display", nil
}

func GetTokenFromOC() (string, error) {
	cmd := exec.Command("oc", "whoami", "-t")
	o, err := cmd.Output()

	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(o)), nil
}
