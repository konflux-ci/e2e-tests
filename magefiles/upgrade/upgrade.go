package upgrade

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/openshift/oc/pkg/cli/admin/upgrade"
	"github.com/openshift/oc/pkg/cli/admin/upgrade/channel"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeclient "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	k8swait "k8s.io/apimachinery/pkg/util/wait"
)

var adminAcks = map[string]string{
	"4.14": "{\"data\":{\"ack-4.13-kube-1.27-api-removals-in-4.14\":\"true\"}}",
}

const majorMinorVersionFormat = "4.%d"
const fastChannelFormat = "fast-" + majorMinorVersionFormat

type statusHelper struct {
	configClientset *configv1client.Clientset
	kubeClientSet   *kubeclient.Clientset

	clusterVersion  *configv1.ClusterVersion
	currentProgress string

	desiredChannel           string
	desiredMajorMinorVersion string
	desiredFullVersion       string

	initialVersion string
}

func (s *statusHelper) update() error {
	var err error
	s.clusterVersion, err = s.configClientset.ConfigV1().ClusterVersions().Get(context.TODO(), "version", metav1.GetOptions{})
	if err != nil {
		return err
	}
	if c := findClusterOperatorStatusCondition(s.clusterVersion.Status.Conditions, configv1.OperatorProgressing); c != nil && len(c.Message) > 0 {
		s.currentProgress = c.Message
	}
	return nil
}

func newStatusHelper(kcs *kubeclient.Clientset, ccs *configv1client.Clientset) (*statusHelper, error) {
	var initialVersion string
	clusterVersion, err := ccs.ConfigV1().ClusterVersions().Get(context.TODO(), "version", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	for _, update := range clusterVersion.Status.History {
		if update.State == configv1.CompletedUpdate {
			initialVersion = update.Version
			break
		}
	}

	currentChannel := clusterVersion.Spec.Channel
	klog.Infof("current channel is %q, current ocp version is %q", currentChannel, initialVersion)

	var minorVersion int
	if _, err = fmt.Sscanf(currentChannel, fastChannelFormat, &minorVersion); err != nil {
		return nil, fmt.Errorf("can't detect the next version channel: %+v", err)
	}
	nextMajorMinorVersion := fmt.Sprintf(majorMinorVersionFormat, minorVersion+1)
	nextVersionChannel := fmt.Sprintf(fastChannelFormat, minorVersion+1)

	var foundNextVersionChannel bool
	for _, ch := range clusterVersion.Status.Desired.Channels {
		if ch == nextVersionChannel {
			foundNextVersionChannel = true
		}
	}
	if !foundNextVersionChannel {
		return nil, fmt.Errorf("the channel for updating to next version was not found in the list of desired channels: %+v", clusterVersion.Status.Desired.Channels)
	}

	klog.Infof("desired major.minor version is %q, desired channel is %q", nextMajorMinorVersion, nextVersionChannel)

	return &statusHelper{
		kubeClientSet:            kcs,
		configClientset:          ccs,
		desiredChannel:           nextVersionChannel,
		desiredMajorMinorVersion: nextMajorMinorVersion,
		initialVersion:           initialVersion,
	}, nil
}

func (s *statusHelper) isCompleted() bool {
	if c := findClusterOperatorStatusCondition(s.clusterVersion.Status.Conditions, configv1.OperatorProgressing); c != nil && len(c.Message) > 0 {
		if c.Status == configv1.ConditionTrue {
			return false
		}
	}
	if c := findClusterOperatorStatusCondition(s.clusterVersion.Status.Conditions, configv1.OperatorAvailable); c != nil && len(c.Message) > 0 {
		if c.Status == configv1.ConditionTrue && strings.Contains(c.Message, s.desiredFullVersion) {
			return true
		}
	}
	return false
}

func (s *statusHelper) performAdminAck(ackData string) error {
	_, err := s.kubeClientSet.CoreV1().ConfigMaps("openshift-config").Patch(context.Background(), "admin-acks", types.MergePatchType, []byte(ackData), metav1.PatchOptions{})
	if err != nil {
		return err
	}
	return nil
}

func PerformUpgrade() error {

	u := upgrade.NewOptions(genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr})
	ch := channel.NewOptions(genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr})

	kubeconfig, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("error when getting config: %+v", err)
	}

	clientset, err := configv1client.NewForConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("error when creating client: %+v", err)
	}

	kubeClientset, err := kubeclient.NewForConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("error when creating client: %+v", err)
	}

	ch.Client = clientset
	u.Client = clientset

	us, err := newStatusHelper(kubeClientset, clientset)
	if err != nil {
		return fmt.Errorf("failed to initialize upgrade status helper: %+v", err)
	}

	ch.Channel = us.desiredChannel

	err = ch.Run()
	if err != nil {
		return fmt.Errorf("failed when updating the upgrade channel to %q: %+v", ch.Channel, err)
	}

	if val, ok := adminAcks[us.desiredMajorMinorVersion]; ok {
		klog.Infof("about to perform an admin ack, since %q version requires it...", us.desiredMajorMinorVersion)
		if err := us.performAdminAck(val); err != nil {
			return fmt.Errorf("unable to perform admin ack: %+v", err)
		}
	}

	err = k8swait.PollUntilContextTimeout(context.Background(), 2*time.Second, 5*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		if err := us.update(); err != nil {
			klog.Errorf("failed to get an update about upgrade status: %+v", err)
			return false, nil
		}

		for _, au := range us.clusterVersion.Status.ConditionalUpdates {
			if strings.Contains(au.Release.Version, us.desiredMajorMinorVersion) {
				klog.Infof("found the desired version %q in available updates", au.Release.Version)
				us.desiredFullVersion = au.Release.Version
				return true, nil
			}
		}
		klog.Infof("desired minor version %q not yet present in available updates", us.desiredMajorMinorVersion)
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("timed out waiting for desired version %q to appear in available updates", us.desiredMajorMinorVersion)
	}

	u.ToLatestAvailable = true
	u.AllowNotRecommended = true

	if err := u.Run(); err != nil {
		return fmt.Errorf("error when triggering the upgrade: %+v", err)
	}

	err = k8swait.PollUntilContextTimeout(context.Background(), 20*time.Second, 90*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		if err := us.update(); err != nil {
			klog.Errorf("failed to get an update about upgrade status: %+v", err)
			return false, nil
		}

		if us.isCompleted() {
			klog.Infof("upgrade completed: %+v", utils.ToPrettyJSONString(us.clusterVersion.Status))
			return true, nil
		}
		klog.Infof("upgrading from %s - current progress: %s", us.initialVersion, us.currentProgress)
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("timed out waiting for the upgrade to finish: %s", utils.ToPrettyJSONString(us.clusterVersion.Status))
	}

	return nil
}

func findClusterOperatorStatusCondition(conditions []configv1.ClusterOperatorStatusCondition, name configv1.ClusterStatusConditionType) *configv1.ClusterOperatorStatusCondition {
	for i := range conditions {
		if conditions[i].Type == name {
			return &conditions[i]
		}
	}
	return nil
}
