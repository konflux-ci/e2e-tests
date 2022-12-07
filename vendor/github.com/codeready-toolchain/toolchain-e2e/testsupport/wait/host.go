package wait

import (
	"context"
	"fmt"
	"hash/crc32"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/toolchain-common/pkg/cluster"
	"github.com/codeready-toolchain/toolchain-common/pkg/condition"
	"github.com/codeready-toolchain/toolchain-common/pkg/test"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"
	"github.com/codeready-toolchain/toolchain-e2e/testsupport/md5"
	"github.com/davecgh/go-spew/spew"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HostAwaitility the Awaitility for the Host cluster
type HostAwaitility struct {
	*Awaitility
	RegistrationServiceNs  string
	RegistrationServiceURL string
	APIProxyURL            string
}

// NewHostAwaitility initializes a HostAwaitility
func NewHostAwaitility(t *testing.T, cfg *rest.Config, cl client.Client, ns string, registrationServiceNs string) *HostAwaitility {
	return &HostAwaitility{
		Awaitility: &Awaitility{
			T:             t,
			Client:        cl,
			RestConfig:    cfg,
			Namespace:     ns,
			Type:          cluster.Host,
			RetryInterval: DefaultRetryInterval,
			Timeout:       DefaultTimeout,
		},
		RegistrationServiceNs: registrationServiceNs,
	}
}

// WithRetryOptions returns a new HostAwaitility with the given RetryOptions applied
func (a *HostAwaitility) WithRetryOptions(options ...RetryOption) *HostAwaitility {
	return &HostAwaitility{
		Awaitility:             a.Awaitility.WithRetryOptions(options...),
		RegistrationServiceNs:  a.RegistrationServiceNs,
		RegistrationServiceURL: a.RegistrationServiceURL,
		APIProxyURL:            a.APIProxyURL,
	}
}

func (a *HostAwaitility) ForTest(t *testing.T) *HostAwaitility {
	return &HostAwaitility{
		Awaitility:             a.Awaitility.ForTest(t),
		RegistrationServiceNs:  a.RegistrationServiceNs,
		RegistrationServiceURL: a.RegistrationServiceURL,
		APIProxyURL:            a.APIProxyURL,
	}
}

func (a *HostAwaitility) sprintAllResources() string {
	all, err := a.allResources()
	buf := &strings.Builder{}
	if err != nil {
		buf.WriteString("unable to list other resources in the host namespace:\n")
		buf.WriteString(err.Error())
		buf.WriteString("\n")
	} else {
		buf.WriteString("other resources in the host namespace:\n")
		for _, r := range all {
			y, _ := yaml.Marshal(r)
			buf.Write(y)
			buf.WriteString("\n")
		}
	}
	return buf.String()
}

// list all relevant resources in the host namespace, in case of test failure and for faster troubleshooting.
func (a *HostAwaitility) allResources() ([]runtime.Object, error) {
	all := []runtime.Object{}
	// usersignups
	usersignups := &toolchainv1alpha1.UserSignupList{}
	if err := a.Client.List(context.TODO(), usersignups, client.InNamespace(a.Namespace)); err != nil {
		return nil, err
	}
	for _, i := range usersignups.Items {
		copy := i
		all = append(all, &copy)
	}
	// masteruserrecords
	masteruserrecords := &toolchainv1alpha1.MasterUserRecordList{}
	if err := a.Client.List(context.TODO(), masteruserrecords, client.InNamespace(a.Namespace)); err != nil {
		return nil, err
	}
	for _, i := range masteruserrecords.Items {
		copy := i
		all = append(all, &copy)
	}
	// notifications
	notifications := &toolchainv1alpha1.NotificationList{}
	if err := a.Client.List(context.TODO(), notifications, client.InNamespace(a.Namespace)); err != nil {
		return nil, err
	}
	for _, i := range notifications.Items {
		copy := i
		all = append(all, &copy)
	}
	// nstemplatetiers
	nstemplatetiers := &toolchainv1alpha1.NSTemplateTierList{}
	if err := a.Client.List(context.TODO(), nstemplatetiers, client.InNamespace(a.Namespace)); err != nil {
		return nil, err
	}
	for _, i := range nstemplatetiers.Items {
		copy := i
		all = append(all, &copy)
	}
	// usertiers
	usertiers := &toolchainv1alpha1.UserTierList{}
	if err := a.Client.List(context.TODO(), usertiers, client.InNamespace(a.Namespace)); err != nil {
		return nil, err
	}
	for _, i := range usertiers.Items {
		copy := i
		all = append(all, &copy)
	}
	// toolchainconfig
	toolchainconfigs := &toolchainv1alpha1.ToolchainConfigList{}
	if err := a.Client.List(context.TODO(), toolchainconfigs, client.InNamespace(a.Namespace)); err != nil {
		return nil, err
	}
	for _, i := range toolchainconfigs.Items {
		copy := i
		all = append(all, &copy)
	}
	// toolchainstatus
	toolchainstatuses := &toolchainv1alpha1.ToolchainStatusList{}
	if err := a.Client.List(context.TODO(), toolchainstatuses, client.InNamespace(a.Namespace)); err != nil {
		return nil, err
	}
	for _, i := range usersignups.Items {
		copy := i
		all = append(all, &copy)
	}
	return all, nil
}

// WaitForMasterUserRecord waits until there is a MasterUserRecord available with the given name and the optional conditions
func (a *HostAwaitility) WaitForMasterUserRecord(name string, criteria ...MasterUserRecordWaitCriterion) (*toolchainv1alpha1.MasterUserRecord, error) {
	a.T.Logf("waiting for MasterUserRecord '%s' in namespace '%s' to match criteria", name, a.Namespace)
	var mur *toolchainv1alpha1.MasterUserRecord
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		obj := &toolchainv1alpha1.MasterUserRecord{}
		if err := a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: name}, obj); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		mur = obj
		return matchMasterUserRecordWaitCriterion(obj, criteria...), nil
	})
	// no match found, print the diffs
	if err != nil {
		a.printMasterUserRecordWaitCriterionDiffs(mur, criteria...)
	}
	return mur, err
}

func (a *HostAwaitility) GetMasterUserRecord(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
	mur := &toolchainv1alpha1.MasterUserRecord{}
	if err := a.Client.Get(context.TODO(), test.NamespacedName(a.Namespace, name), mur); err != nil {
		return nil, err
	}
	return mur, nil
}

// UpdateMasterUserRecordSpec tries to update the Spec of the given MasterUserRecord
// If it fails with an error (for example if the object has been modified) then it retrieves the latest version and and tries again
// Returns the updated MasterUserRecord
func (a *HostAwaitility) UpdateMasterUserRecordSpec(murName string, modifyMur func(mur *toolchainv1alpha1.MasterUserRecord)) (*toolchainv1alpha1.MasterUserRecord, error) {
	return a.UpdateMasterUserRecord(false, murName, modifyMur)
}

// UpdateMasterUserRecordStatus tries to update the Status of the given MasterUserRecord
// If it fails with an error (for example if the object has been modified) then it retrieves the latest version and and tries again
// Returns the updated MasterUserRecord
func (a *HostAwaitility) UpdateMasterUserRecordStatus(murName string, modifyMur func(mur *toolchainv1alpha1.MasterUserRecord)) (*toolchainv1alpha1.MasterUserRecord, error) {
	return a.UpdateMasterUserRecord(true, murName, modifyMur)
}

// UpdateMasterUserRecord tries to update the Spec or the Status of the given MasterUserRecord
// If it fails with an error (for example if the object has been modified) then it retrieves the latest version and and tries again
// Returns the updated MasterUserRecord
func (a *HostAwaitility) UpdateMasterUserRecord(status bool, murName string, modifyMur func(mur *toolchainv1alpha1.MasterUserRecord)) (*toolchainv1alpha1.MasterUserRecord, error) {
	var m *toolchainv1alpha1.MasterUserRecord
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		freshMur := &toolchainv1alpha1.MasterUserRecord{}
		if err := a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: murName}, freshMur); err != nil {
			return true, err
		}

		modifyMur(freshMur)
		if status {
			// Update status
			if err := a.Client.Status().Update(context.TODO(), freshMur); err != nil {
				a.T.Logf("error updating MasterUserRecord.Status '%s': %s. Will retry again...", murName, err.Error())
				return false, nil
			}
		} else if err := a.Client.Update(context.TODO(), freshMur); err != nil {
			a.T.Logf("error updating MasterUserRecord.Spec '%s': %s. Will retry again...", murName, err.Error())
			return false, nil
		}
		m = freshMur
		return true, nil
	})
	return m, err
}

// UpdateUserSignup tries to update the Spec of the given UserSignup
// If it fails with an error (for example if the object has been modified) then it retrieves the latest version and tries again
// Returns the updated UserSignup
func (a *HostAwaitility) UpdateUserSignup(userSignupName string, modifyUserSignup func(us *toolchainv1alpha1.UserSignup)) (*toolchainv1alpha1.UserSignup, error) {
	var userSignup *toolchainv1alpha1.UserSignup
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		freshUserSignup := &toolchainv1alpha1.UserSignup{}
		if err := a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: userSignupName}, freshUserSignup); err != nil {
			return true, err
		}

		modifyUserSignup(freshUserSignup)
		if err := a.Client.Update(context.TODO(), freshUserSignup); err != nil {
			a.T.Logf("error updating UserSignup '%s': %s. Will retry again...", userSignupName, err.Error())
			return false, nil
		}
		userSignup = freshUserSignup
		return true, nil
	})
	return userSignup, err
}

// UpdateSpace tries to update the Spec of the given Space
// If it fails with an error (for example if the object has been modified) then it retrieves the latest version and tries again
// Returns the updated Space
func (a *HostAwaitility) UpdateSpace(spaceName string, modifySpace func(s *toolchainv1alpha1.Space)) (*toolchainv1alpha1.Space, error) {
	var s *toolchainv1alpha1.Space
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		freshSpace := &toolchainv1alpha1.Space{}
		if err := a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: spaceName}, freshSpace); err != nil {
			return true, err
		}
		modifySpace(freshSpace)
		if err := a.Client.Update(context.TODO(), freshSpace); err != nil {
			a.T.Logf("error updating Space '%s': %s. Will retry again...", spaceName, err.Error())
			return false, nil
		}
		s = freshSpace
		return true, nil
	})
	return s, err
}

// MasterUserRecordWaitCriterion a struct to compare with an expected MasterUserRecord
type MasterUserRecordWaitCriterion struct {
	Match func(*toolchainv1alpha1.MasterUserRecord) bool
	Diff  func(*toolchainv1alpha1.MasterUserRecord) string
}

func matchMasterUserRecordWaitCriterion(actual *toolchainv1alpha1.MasterUserRecord, criteria ...MasterUserRecordWaitCriterion) bool {
	for _, c := range criteria {
		if !c.Match(actual) {
			return false
		}
	}
	return true
}

func (a *HostAwaitility) printMasterUserRecordWaitCriterionDiffs(actual *toolchainv1alpha1.MasterUserRecord, criteria ...MasterUserRecordWaitCriterion) {
	buf := &strings.Builder{}
	if actual == nil {
		buf.WriteString("failed to find MasterUserRecord\n")
	} else {
		buf.WriteString("failed to find MasterUserRecord with matching criteria:\n")
		buf.WriteString("----\n")
		buf.WriteString("actual:\n")
		y, _ := StringifyObject(actual)
		buf.Write(y)
		buf.WriteString("\n----\n")
		buf.WriteString("diffs:\n")
		for _, c := range criteria {
			if !c.Match(actual) && c.Diff != nil {
				buf.WriteString(c.Diff(actual))
				buf.WriteString("\n")
			}
		}
	}
	// also include other resources relevant in the host namespace, to help troubleshooting
	a.listAndPrint("UserSignups", a.Namespace, &toolchainv1alpha1.UserSignupList{})
	a.listAndPrint("MasterUserRecords", a.Namespace, &toolchainv1alpha1.MasterUserRecordList{})
	a.listAndPrint("Spaces", a.Namespace, &toolchainv1alpha1.SpaceList{})

	a.T.Log(buf.String())
}

// UntilMasterUserRecordIsBeingDeleted checks if MasterUserRecord has Deletion Timestamp
func UntilMasterUserRecordIsBeingDeleted() MasterUserRecordWaitCriterion {
	return MasterUserRecordWaitCriterion{
		Match: func(actual *toolchainv1alpha1.MasterUserRecord) bool {
			return actual.DeletionTimestamp != nil
		},
	}
}

// UntilMasterUserRecordHasCondition checks if MasterUserRecord status has the given conditions (among others)
func UntilMasterUserRecordHasCondition(expected toolchainv1alpha1.Condition) MasterUserRecordWaitCriterion {
	return MasterUserRecordWaitCriterion{
		Match: func(actual *toolchainv1alpha1.MasterUserRecord) bool {
			return test.ContainsCondition(actual.Status.Conditions, expected)
		},
		Diff: func(actual *toolchainv1alpha1.MasterUserRecord) string {
			e, _ := yaml.Marshal(expected)
			a, _ := yaml.Marshal(actual.Status.Conditions)
			return fmt.Sprintf("expected conditions to contain: %s.\n\tactual: %s", e, a)
		},
	}
}

// UntilMasterUserRecordHasConditions checks if MasterUserRecord status has the given set of conditions
func UntilMasterUserRecordHasConditions(expected ...toolchainv1alpha1.Condition) MasterUserRecordWaitCriterion {
	return MasterUserRecordWaitCriterion{
		Match: func(actual *toolchainv1alpha1.MasterUserRecord) bool {
			return test.ConditionsMatch(actual.Status.Conditions, expected...)
		},
		Diff: func(actual *toolchainv1alpha1.MasterUserRecord) string {
			return fmt.Sprintf("expected conditions to match:\n%s", Diff(expected, actual.Status.Conditions))
		},
	}
}

func WithMurName(name string) MasterUserRecordWaitCriterion {
	return MasterUserRecordWaitCriterion{
		Match: func(actual *toolchainv1alpha1.MasterUserRecord) bool {
			return actual.Name == name
		},
		Diff: func(actual *toolchainv1alpha1.MasterUserRecord) string {
			return fmt.Sprintf("expected MasterUserRecord named '%s'", name)
		},
	}
}

// UntilMasterUserRecordHasUserAccountStatuses checks if MasterUserRecord status has the given set of status embedded UserAccounts
func UntilMasterUserRecordHasUserAccountStatuses(expected ...toolchainv1alpha1.UserAccountStatusEmbedded) MasterUserRecordWaitCriterion {
	return MasterUserRecordWaitCriterion{
		Match: func(actual *toolchainv1alpha1.MasterUserRecord) bool {
			if len(actual.Status.UserAccounts) != len(expected) {
				return false
			}
			for _, expUaStatus := range expected {
				if !containsUserAccountStatus(actual.Status.UserAccounts, expUaStatus) {
					return false
				}
			}
			return true
		},
		Diff: func(actual *toolchainv1alpha1.MasterUserRecord) string {
			return fmt.Sprintf("expected UserAccount statuses to match: %s", Diff(expected, actual.Status.UserAccounts))
		},
	}
}

func UntilMasterUserRecordHasTierName(expected string) MasterUserRecordWaitCriterion {
	return MasterUserRecordWaitCriterion{
		Match: func(actual *toolchainv1alpha1.MasterUserRecord) bool {
			return actual.Spec.TierName == expected
		},
		Diff: func(actual *toolchainv1alpha1.MasterUserRecord) string {
			return fmt.Sprintf("expected spec.TierName to match: %s", Diff(expected, actual.Spec.TierName))
		},
	}
}

func UntilMasterUserRecordHasNoTierHashLabel() MasterUserRecordWaitCriterion {
	return MasterUserRecordWaitCriterion{
		Match: func(actual *toolchainv1alpha1.MasterUserRecord) bool {
			for key := range actual.Labels {
				if strings.HasSuffix(key, "-tier-hash") {
					return false
				}
			}
			return true
		},
	}
}

// UserSignupWaitCriterion a struct to compare with an expected UserSignup
type UserSignupWaitCriterion struct {
	Match func(*toolchainv1alpha1.UserSignup) bool
	Diff  func(*toolchainv1alpha1.UserSignup) string
}

func matchUserSignupWaitCriterion(actual *toolchainv1alpha1.UserSignup, criteria ...UserSignupWaitCriterion) bool {
	for _, c := range criteria {
		if !c.Match(actual) {
			return false
		}
	}
	return true
}

func (a *HostAwaitility) printUserSignupWaitCriterionDiffs(actual *toolchainv1alpha1.UserSignup, criteria ...UserSignupWaitCriterion) {
	buf := &strings.Builder{}
	if actual == nil {
		buf.WriteString("failed to find UserSignup\n")
	} else {
		buf.WriteString("failed to find UserSignup with matching criteria:\n")
		buf.WriteString("actual:\n")
		y, _ := StringifyObject(actual)
		buf.Write(y)
		buf.WriteString("\n----\n")
		buf.WriteString("diffs:\n")
		for _, c := range criteria {
			if !c.Match(actual) && c.Diff != nil {
				buf.WriteString(c.Diff(actual))
				buf.WriteString("\n")
			}
		}
	}
	// also include other resources relevant in the host namespace, to help troubleshooting
	buf.WriteString(a.sprintAllResources())

	a.T.Log(buf.String())
}

// UntilUserSignupIsBeingDeleted returns a `UserSignupWaitCriterion` which checks that the given
// UserSignup has deletion timestamp set
func UntilUserSignupIsBeingDeleted() UserSignupWaitCriterion {
	return UserSignupWaitCriterion{
		Match: func(actual *toolchainv1alpha1.UserSignup) bool {
			return actual.DeletionTimestamp != nil
		},
		Diff: func(_ *toolchainv1alpha1.UserSignup) string {
			return "expected a non-nil DeletionTimestamp"
		},
	}
}

// UntilUserSignupHasConditions returns a `UserAccountWaitCriterion` which checks that the given
// UserAccount has exactly all the given status conditions
func UntilUserSignupHasConditions(expected ...toolchainv1alpha1.Condition) UserSignupWaitCriterion {
	return UserSignupWaitCriterion{
		Match: func(actual *toolchainv1alpha1.UserSignup) bool {
			return test.ConditionsMatch(actual.Status.Conditions, expected...)
		},
		Diff: func(actual *toolchainv1alpha1.UserSignup) string {
			return fmt.Sprintf("expected conditions to match:\n%s", Diff(expected, actual.Status.Conditions))
		},
	}
}

// UntilUserSignupContainsConditions returns a `UserAccountWaitCriterion` which checks that the given
// UserAccount contains all the given status conditions
func UntilUserSignupContainsConditions(shouldContain ...toolchainv1alpha1.Condition) UserSignupWaitCriterion {
	return UserSignupWaitCriterion{
		Match: func(actual *toolchainv1alpha1.UserSignup) bool {
			for _, cond := range shouldContain {
				if !test.ContainsCondition(actual.Status.Conditions, cond) {
					return false
				}
			}
			return true
		},
		Diff: func(actual *toolchainv1alpha1.UserSignup) string {
			return fmt.Sprintf("expected conditions to contain:\n%s", Diff(shouldContain, actual.Status.Conditions))
		},
	}
}

// ContainsCondition returns a `UserAccountWaitCriterion` which checks that the given
// UserAccount contains the given status condition
func ContainsCondition(expected toolchainv1alpha1.Condition) UserSignupWaitCriterion {
	return UserSignupWaitCriterion{
		Match: func(actual *toolchainv1alpha1.UserSignup) bool {
			return test.ContainsCondition(actual.Status.Conditions, expected)
		},
		Diff: func(actual *toolchainv1alpha1.UserSignup) string {
			e, _ := yaml.Marshal(expected)
			a, _ := yaml.Marshal(actual.Status.Conditions)
			return fmt.Sprintf("expected conditions to contain: %s.\n\tactual: %s", e, a)
		},
	}
}

// UntilUserSignupHasStateLabel returns a `UserAccountWaitCriterion` which checks that the given
// UserAccount has toolchain.dev.openshift.com/state equal to the given value
func UntilUserSignupHasStateLabel(expected string) UserSignupWaitCriterion {
	return UserSignupWaitCriterion{
		Match: func(actual *toolchainv1alpha1.UserSignup) bool {
			return actual.Labels != nil && actual.Labels[toolchainv1alpha1.UserSignupStateLabelKey] == expected
		},
		Diff: func(actual *toolchainv1alpha1.UserSignup) string {
			if len(actual.Labels) == 0 {
				return fmt.Sprintf("expected to have a label with key '%s' (and value", toolchainv1alpha1.UserSignupStateLabelKey)
			}
			return fmt.Sprintf("expected value of label '%s' to equal '%s'. Actual: '%s'", toolchainv1alpha1.UserSignupStateLabelKey, expected, actual.Labels[toolchainv1alpha1.UserSignupStateLabelKey])
		},
	}
}

// WaitForTestResourcesCleanup waits for all UserSignup and MasterUserRecord deletions to complete
func (a *HostAwaitility) WaitForTestResourcesCleanup(initialDelay time.Duration) error {
	a.T.Logf("waiting for resource cleanup")
	time.Sleep(initialDelay)
	return wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		usList := &toolchainv1alpha1.UserSignupList{}
		if err := a.Client.List(context.TODO(), usList, client.InNamespace(a.Namespace)); err != nil {
			return false, err
		}
		for _, us := range usList.Items {
			if us.DeletionTimestamp != nil {
				return false, nil
			}
		}

		murList := &toolchainv1alpha1.MasterUserRecordList{}
		if err := a.Client.List(context.TODO(), murList, client.InNamespace(a.Namespace)); err != nil {
			return false, err
		}
		for _, mur := range murList.Items {
			if mur.DeletionTimestamp != nil {
				return false, nil
			}
		}
		return true, nil
	})
}

// WaitForUserSignup waits until there is a UserSignup available with the given name and set of status conditions
func (a *HostAwaitility) WaitForUserSignup(name string, criteria ...UserSignupWaitCriterion) (*toolchainv1alpha1.UserSignup, error) {
	a.T.Logf("waiting for UserSignup '%s' in namespace '%s' to match criteria", name, a.Namespace)
	var userSignup *toolchainv1alpha1.UserSignup
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		obj := &toolchainv1alpha1.UserSignup{}
		if err := a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: name}, obj); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		userSignup = obj
		return matchUserSignupWaitCriterion(userSignup, criteria...), nil
	})
	// no match found, print the diffs
	if err != nil {
		a.printUserSignupWaitCriterionDiffs(userSignup, criteria...)
	}
	return userSignup, err
}

// WaitForUserSignup waits until there is a UserSignup available with the given name and set of status conditions
func (a *HostAwaitility) WaitForUserSignupByUserIDAndUsername(userID, username string, criteria ...UserSignupWaitCriterion) (*toolchainv1alpha1.UserSignup, error) {
	a.T.Logf("waiting for UserSignup '%s' or '%s' in namespace '%s' to match criteria", userID, username, a.Namespace)
	encodedUsername := EncodeUserIdentifier(username)
	var userSignup *toolchainv1alpha1.UserSignup
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		obj := &toolchainv1alpha1.UserSignup{}
		if err := a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: userID}, obj); err != nil {
			if errors.IsNotFound(err) {
				if err := a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: encodedUsername}, obj); err != nil {
					if errors.IsNotFound(err) {
						return false, nil
					}
					return false, err
				}
			} else {
				return false, err
			}
		}
		userSignup = obj
		return matchUserSignupWaitCriterion(userSignup, criteria...), nil
	})
	// no match found, print the diffs
	if err != nil {
		a.printUserSignupWaitCriterionDiffs(userSignup, criteria...)
	}
	return userSignup, err
}

// WaitAndVerifyThatUserSignupIsNotCreated waits and checks that the UserSignup is not created
func (a *HostAwaitility) WaitAndVerifyThatUserSignupIsNotCreated(name string) {
	a.T.Logf("waiting and verifying that UserSignup '%s' in namespace '%s' is not created", name, a.Namespace)
	var userSignup *toolchainv1alpha1.UserSignup
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		obj := &toolchainv1alpha1.UserSignup{}
		if err := a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: name}, obj); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		userSignup = obj
		return true, nil
	})
	if err == nil {
		require.Fail(a.T, fmt.Sprintf("UserSignup '%s' should not be created, but it was found: %v", name, userSignup))
	}
}

// WaitForBannedUser waits until there is a BannedUser available with the given email
func (a *HostAwaitility) WaitForBannedUser(email string) (*toolchainv1alpha1.BannedUser, error) {
	a.T.Logf("waiting for BannedUser for user '%s' in namespace '%s'", email, a.Namespace)
	var bannedUser *toolchainv1alpha1.BannedUser
	labels := map[string]string{toolchainv1alpha1.BannedUserEmailHashLabelKey: md5.CalcMd5(email)}
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		bannedUserList := &toolchainv1alpha1.BannedUserList{}
		if err = a.Client.List(context.TODO(), bannedUserList, client.MatchingLabels(labels), client.InNamespace(a.Namespace)); err != nil {
			if len(bannedUserList.Items) == 0 {
				return false, nil
			}
			return false, err
		}
		bannedUser = &bannedUserList.Items[0]
		return true, nil
	})
	// log message if an error occurred
	if err != nil {
		a.T.Logf("failed to find Banned for email address '%s': %v", email, err)
	}
	return bannedUser, err
}

// DeleteToolchainStatus deletes the ToolchainStatus resource with the given name and in the host operator namespace
func (a *HostAwaitility) DeleteToolchainStatus(name string) error {
	a.T.Logf("deleting ToolchainStatus '%s' in namespace '%s'", name, a.Namespace)
	toolchainstatus := &toolchainv1alpha1.ToolchainStatus{}
	if err := a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: name}, toolchainstatus); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return a.Client.Delete(context.TODO(), toolchainstatus)
}

// WaitUntilBannedUserDeleted waits until the BannedUser with the given name is deleted (ie, not found)
func (a *HostAwaitility) WaitUntilBannedUserDeleted(name string) error {
	a.T.Logf("waiting until BannedUser '%s' in namespace '%s' is deleted", name, a.Namespace)
	return wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		user := &toolchainv1alpha1.BannedUser{}
		if err := a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: name}, user); err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	})
}

// WaitUntilUserSignupDeleted waits until the UserSignup with the given name is deleted (ie, not found)
func (a *HostAwaitility) WaitUntilUserSignupDeleted(name string) error {
	a.T.Logf("waiting until UserSignup '%s' in namespace '%s is deleted", name, a.Namespace)
	return wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		userSignup := &toolchainv1alpha1.UserSignup{}
		if err := a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: name}, userSignup); err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	})
}

// WaitUntilMasterUserRecordAndSpaceBindingsDeleted waits until the MUR with the given name and its associated SpaceBindings are deleted (ie, not found)
func (a *HostAwaitility) WaitUntilMasterUserRecordAndSpaceBindingsDeleted(name string) error {
	a.T.Logf("waiting until MasterUserRecord '%s' in namespace '%s' is deleted", name, a.Namespace)
	return wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		mur := &toolchainv1alpha1.MasterUserRecord{}
		if err := a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: name}, mur); err != nil {
			if errors.IsNotFound(err) {
				// once the MUR is deleted, wait for the associated spacebindings to be deleted as well
				if err := a.WaitUntilSpaceBindingsWithLabelDeleted(toolchainv1alpha1.SpaceBindingMasterUserRecordLabelKey, name); err != nil {
					return false, err
				}
				return true, nil
			}
			return false, err
		}
		return false, nil
	})
}

// CheckMasterUserRecordIsDeleted checks that the MUR with the given name is not present and won't be created in the next 2 seconds
func (a *HostAwaitility) CheckMasterUserRecordIsDeleted(name string) {
	a.T.Logf("checking that MasterUserRecord '%s' in namespace '%s' is deleted", name, a.Namespace)
	err := wait.Poll(a.RetryInterval, 2*time.Second, func() (done bool, err error) {
		mur := &toolchainv1alpha1.MasterUserRecord{}
		if err := a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: name}, mur); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return false, fmt.Errorf("the MasterUserRecord '%s' should not be present, but it is", name)
	})
	require.Equal(a.T, wait.ErrWaitTimeout, err)
}

func containsUserAccountStatus(uaStatuses []toolchainv1alpha1.UserAccountStatusEmbedded, uaStatus toolchainv1alpha1.UserAccountStatusEmbedded) bool {
	for _, status := range uaStatuses {
		if reflect.DeepEqual(uaStatus.Cluster, status.Cluster) &&
			test.ConditionsMatch(uaStatus.Conditions, status.Conditions...) {
			return true
		}
	}
	return false
}

// WaitForUserTier waits until an UserTier with the given name exists and matches any given criteria
func (a *HostAwaitility) WaitForUserTier(name string, criteria ...UserTierWaitCriterion) (*toolchainv1alpha1.UserTier, error) {
	a.T.Logf("waiting until UserTier '%s' in namespace '%s' matches criteria", name, a.Namespace)
	tier := &toolchainv1alpha1.UserTier{}
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		obj := &toolchainv1alpha1.UserTier{}
		err = a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: name}, obj)
		if err != nil && !errors.IsNotFound(err) {
			// return the error
			return false, err
		} else if errors.IsNotFound(err) {
			// keep waiting
			return false, nil
		}
		tier = obj
		return matchUserTierWaitCriterion(obj, criteria...), nil
	})
	// no match found, print the diffs
	if err != nil {
		a.printUserTierWaitCriterionDiffs(tier, criteria...)
	}
	return tier, err
}

// UserTierWaitCriterion a struct to compare with an expected UserTier
type UserTierWaitCriterion struct {
	Match func(*toolchainv1alpha1.UserTier) bool
	Diff  func(*toolchainv1alpha1.UserTier) string
}

func matchUserTierWaitCriterion(actual *toolchainv1alpha1.UserTier, criteria ...UserTierWaitCriterion) bool {
	for _, c := range criteria {
		// if at least one criteria does not match, keep waiting
		if !c.Match(actual) {
			return false
		}
	}
	return true
}

func (a *HostAwaitility) printUserTierWaitCriterionDiffs(actual *toolchainv1alpha1.UserTier, criteria ...UserTierWaitCriterion) {
	buf := &strings.Builder{}
	if actual == nil {
		buf.WriteString("failed to find UserTier\n")
	} else {
		buf.WriteString("failed to find UserTier with matching criteria:\n")
		buf.WriteString("actual:\n")
		y, _ := StringifyObject(actual)
		buf.Write(y)
		buf.WriteString("\n----\n")
		buf.WriteString("diffs:\n")
		for _, c := range criteria {
			if !c.Match(actual) {
				buf.WriteString(c.Diff(actual))
				buf.WriteString("\n")
			}
		}
	}

	a.T.Log(buf.String())
}

// UntilUserTierHasDeactivationTimeoutDays verify that the UserTier status.Updates has the specified number of entries
func UntilUserTierHasDeactivationTimeoutDays(expected int) UserTierWaitCriterion {
	return UserTierWaitCriterion{
		Match: func(actual *toolchainv1alpha1.UserTier) bool {
			return actual.Spec.DeactivationTimeoutDays == expected
		},
		Diff: func(actual *toolchainv1alpha1.UserTier) string {
			return fmt.Sprintf("expected deactivationTimeoutDay value %d. Actual: %d", expected, actual.Spec.DeactivationTimeoutDays)
		},
	}
}

func (a *HostAwaitility) WaitUntilBaseUserTierIsUpdated() error {
	_, err := a.WaitForUserTier("deactivate30", UntilUserTierHasDeactivationTimeoutDays(30))
	return err
}

func (a *HostAwaitility) WaitUntilBaseNSTemplateTierIsUpdated() error {
	_, err := a.WaitForNSTemplateTier("base", UntilNSTemplateTierSpec(HasNoTemplateRefWithSuffix("-000000a")))
	return err
}

// WaitForNSTemplateTier waits until an NSTemplateTier with the given name exists and matches the given conditions
func (a *HostAwaitility) WaitForNSTemplateTier(name string, criteria ...NSTemplateTierWaitCriterion) (*toolchainv1alpha1.NSTemplateTier, error) {
	a.T.Logf("waiting until NSTemplateTier '%s' in namespace '%s' matches criteria", name, a.Namespace)
	tier := &toolchainv1alpha1.NSTemplateTier{}
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		obj := &toolchainv1alpha1.NSTemplateTier{}
		err = a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: name}, obj)
		if err != nil && !errors.IsNotFound(err) {
			// return the error
			return false, err
		} else if errors.IsNotFound(err) {
			// keep waiting
			return false, nil
		}
		tier = obj
		return matchNSTemplateTierWaitCriterion(obj, criteria...), nil
	})
	// no match found, print the diffs
	if err != nil {
		a.printNSTemplateTierWaitCriterionDiffs(tier, criteria...)
	}
	return tier, err
}

// WaitForNSTemplateTierAndCheckTemplates waits until an NSTemplateTier with the given name exists matching the given conditions and then it verifies that all expected templates exist
func (a *HostAwaitility) WaitForNSTemplateTierAndCheckTemplates(name string, criteria ...NSTemplateTierWaitCriterion) (*toolchainv1alpha1.NSTemplateTier, error) {
	tier, err := a.WaitForNSTemplateTier(name, criteria...)
	if err != nil {
		return nil, err
	}

	// now, check that the `templateRef` field is set for each namespace and clusterResources (if applicable)
	// and that there's a TierTemplate resource with the same name
	for i, ns := range tier.Spec.Namespaces {
		if ns.TemplateRef == "" {
			return nil, fmt.Errorf("missing 'templateRef' in namespace #%d in NSTemplateTier '%s'", i, tier.Name)
		}
		if _, err := a.WaitForTierTemplate(ns.TemplateRef); err != nil {
			return nil, err
		}
	}
	if tier.Spec.ClusterResources != nil {
		if tier.Spec.ClusterResources.TemplateRef == "" {
			return nil, fmt.Errorf("missing 'templateRef' for the cluster resources in NSTemplateTier '%s'", tier.Name)
		}
		if _, err := a.WaitForTierTemplate(tier.Spec.ClusterResources.TemplateRef); err != nil {
			return nil, err
		}
	}
	return tier, err
}

// WaitForTierTemplate waits until a TierTemplate with the given name exists
// Returns an error if the resource did not exist (or something wrong happened)
func (a *HostAwaitility) WaitForTierTemplate(name string) (*toolchainv1alpha1.TierTemplate, error) { // nolint:unparam
	tierTemplate := &toolchainv1alpha1.TierTemplate{}
	a.T.Logf("waiting until TierTemplate '%s' exists in namespace '%s'...", name, a.Namespace)
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		obj := &toolchainv1alpha1.TierTemplate{}
		if err := a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: name}, obj); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		tierTemplate = obj
		return true, nil
	})
	// log message if an error occurred
	if err != nil {
		a.T.Logf("failed to find TierTemplate '%s': %v", name, err)
	}
	return tierTemplate, err
}

// NSTemplateTierWaitCriterion a struct to compare with an expected NSTemplateTier
type NSTemplateTierWaitCriterion struct {
	Match func(*toolchainv1alpha1.NSTemplateTier) bool
	Diff  func(*toolchainv1alpha1.NSTemplateTier) string
}

func matchNSTemplateTierWaitCriterion(actual *toolchainv1alpha1.NSTemplateTier, criteria ...NSTemplateTierWaitCriterion) bool {
	for _, c := range criteria {
		// if at least one criteria does not match, keep waiting
		if !c.Match(actual) {
			return false
		}
	}
	return true
}

func (a *HostAwaitility) printNSTemplateTierWaitCriterionDiffs(actual *toolchainv1alpha1.NSTemplateTier, criteria ...NSTemplateTierWaitCriterion) {
	buf := &strings.Builder{}
	if actual == nil {
		buf.WriteString("failed to find NSTemplateTier\n")
	} else {
		buf.WriteString("failed to find NSTemplateTier with matching criteria:\n")
		buf.WriteString("actual:\n")
		y, _ := StringifyObject(actual)
		buf.Write(y)
		buf.WriteString("\n----\n")
		buf.WriteString("diffs:\n")
		for _, c := range criteria {
			if !c.Match(actual) {
				buf.WriteString(c.Diff(actual))
				buf.WriteString("\n")
			}
		}
	}
	// also include other resources relevant in the host namespace, to help troubleshooting
	buf.WriteString(a.sprintAllResources())

	a.T.Log(buf.String())
}

// NSTemplateTierSpecMatcher a struct to compare with an expected NSTemplateTierSpec
type NSTemplateTierSpecMatcher struct {
	Match func(toolchainv1alpha1.NSTemplateTierSpec) bool
	Diff  func(toolchainv1alpha1.NSTemplateTierSpec) string
}

// UntilNSTemplateTierSpec verify that the NSTemplateTier spec has the specified condition
func UntilNSTemplateTierSpec(matcher NSTemplateTierSpecMatcher) NSTemplateTierWaitCriterion {
	return NSTemplateTierWaitCriterion{
		Match: func(actual *toolchainv1alpha1.NSTemplateTier) bool {
			return matcher.Match(actual.Spec)
		},
		Diff: func(actual *toolchainv1alpha1.NSTemplateTier) string {
			return matcher.Diff(actual.Spec)
		},
	}
}

// UntilNSTemplateTierStatusUpdates verify that the NSTemplateTier status.Updates has the specified number of entries
func UntilNSTemplateTierStatusUpdates(expected int) NSTemplateTierWaitCriterion {
	return NSTemplateTierWaitCriterion{
		Match: func(actual *toolchainv1alpha1.NSTemplateTier) bool {
			return len(actual.Status.Updates) == expected
		},
		Diff: func(actual *toolchainv1alpha1.NSTemplateTier) string {
			return fmt.Sprintf("expected status.updates count %d. Actual: %d", expected, len(actual.Status.Updates))
		},
	}
}

// HasNoTemplateRefWithSuffix checks that ALL namespaces' `TemplateRef` doesn't have the suffix
func HasNoTemplateRefWithSuffix(suffix string) NSTemplateTierSpecMatcher {
	return NSTemplateTierSpecMatcher{
		Match: func(actual toolchainv1alpha1.NSTemplateTierSpec) bool {
			for _, ns := range actual.Namespaces {
				if strings.HasSuffix(ns.TemplateRef, suffix) {
					return false
				}
			}
			if actual.ClusterResources == nil {
				return false
			}
			return !strings.HasSuffix(actual.ClusterResources.TemplateRef, suffix)
		},
		Diff: func(actual toolchainv1alpha1.NSTemplateTierSpec) string {
			a, _ := yaml.Marshal(actual)
			return fmt.Sprintf("expected no TemplateRef with suffix '%s'. Actual: %s", suffix, a)
		},
	}
}

// HasClusterResourcesTemplateRef checks that the clusterResources `TemplateRef` match the given value
func HasClusterResourcesTemplateRef(expected string) NSTemplateTierSpecMatcher {
	return NSTemplateTierSpecMatcher{
		Match: func(actual toolchainv1alpha1.NSTemplateTierSpec) bool {
			return actual.ClusterResources.TemplateRef == expected
		},
		Diff: func(actual toolchainv1alpha1.NSTemplateTierSpec) string {
			return fmt.Sprintf("expected no ClusterResources.TemplateRef to equal '%s'. Actual: '%s'", expected, actual.ClusterResources.TemplateRef)
		},
	}
}

// WaitForChangeTierRequest waits until there a ChangeTierRequest is available with the given status conditions
func (a *HostAwaitility) WaitForChangeTierRequest(name string, expected toolchainv1alpha1.Condition) (*toolchainv1alpha1.ChangeTierRequest, error) {
	var changeTierRequest *toolchainv1alpha1.ChangeTierRequest
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		obj := &toolchainv1alpha1.ChangeTierRequest{}
		if err := a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: name}, obj); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		changeTierRequest = obj
		return test.ConditionsMatch(obj.Status.Conditions, expected), nil
	})
	// log message if an error occurred
	if err != nil {
		if changeTierRequest == nil {
			e, _ := yaml.Marshal(expected)
			a.T.Logf("failed to find ChangeTierRequest '%s' with condition\n%s. Actual: nil", name, e)
		} else {
			a.T.Logf("expected conditions to match: '%s'", Diff(expected, changeTierRequest.Status.Conditions))
		}
	}
	return changeTierRequest, err
}

// WaitUntilChangeTierRequestDeleted waits until the ChangeTierRequest with the given name is deleted (ie, not found)
func (a *HostAwaitility) WaitUntilChangeTierRequestDeleted(name string) error {
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		changeTierRequest := &toolchainv1alpha1.ChangeTierRequest{}
		if err := a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: name}, changeTierRequest); err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	})
	// log message if an error occurred
	if err != nil {
		a.T.Logf("failed to wait until ChangeTierRequest '%s' was deleted: %v\n", name, err)
	}
	return err
}

// NotificationWaitCriterion a struct to compare with an expected Notification
type NotificationWaitCriterion struct {
	Match func(toolchainv1alpha1.Notification) bool
	Diff  func(toolchainv1alpha1.Notification) string
}

func matchNotificationWaitCriterion(actual []toolchainv1alpha1.Notification, criteria ...NotificationWaitCriterion) bool {
	for _, n := range actual {
		for _, c := range criteria {
			if !c.Match(n) {
				return false
			}
		}
	}
	return true
}

func (a *HostAwaitility) printNotificationWaitCriterionDiffs(actual []toolchainv1alpha1.Notification, criteria ...NotificationWaitCriterion) {
	buf := &strings.Builder{}
	if len(actual) == 0 {
		buf.WriteString("no notification found\n")
	} else {
		buf.WriteString("failed to find notifications with matching criteria:\n")
		buf.WriteString("actual:\n")
		for _, obj := range actual {
			y, _ := StringifyObject(&obj) // nolint:gosec
			buf.Write(y)
		}
		buf.WriteString("\n----\n")
		buf.WriteString("diffs:\n")
		for _, n := range actual {
			for _, c := range criteria {
				if !c.Match(n) {
					buf.WriteString(c.Diff(n))
					buf.WriteString("\n")
				}
			}
		}
	}
	// also include other resources relevant in the host namespace, to help troubleshooting
	buf.WriteString(a.sprintAllResources())

	a.T.Log(buf.String())
}

// WaitForNotifications waits until there is an expected number of Notifications available for the provided user and with the notification type and which match the conditions (if provided).
func (a *HostAwaitility) WaitForNotifications(username, notificationType string, numberOfNotifications int, criteria ...NotificationWaitCriterion) ([]toolchainv1alpha1.Notification, error) {
	a.T.Logf("waiting for notifications to match criteria for user '%s'", username)
	var notifications []toolchainv1alpha1.Notification
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		labels := map[string]string{toolchainv1alpha1.NotificationUserNameLabelKey: username, toolchainv1alpha1.NotificationTypeLabelKey: notificationType}
		opts := client.MatchingLabels(labels)
		notificationList := &toolchainv1alpha1.NotificationList{}
		if err := a.Client.List(context.TODO(), notificationList, opts); err != nil {
			return false, err
		}
		notifications = notificationList.Items
		if numberOfNotifications != len(notificationList.Items) {
			return false, nil
		}
		return matchNotificationWaitCriterion(notificationList.Items, criteria...), nil
	})
	// no match found, print the diffs
	if err != nil {
		a.printNotificationWaitCriterionDiffs(notifications, criteria...)
	}
	return notifications, err
}

// WaitUntilNotificationsDeleted waits until the Notification for the given user is deleted (ie, not found)
func (a *HostAwaitility) WaitUntilNotificationsDeleted(username, notificationType string) error {
	a.T.Logf("waiting until notifications have been deleted for user '%s'", username)
	return wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		labels := map[string]string{toolchainv1alpha1.NotificationUserNameLabelKey: username, toolchainv1alpha1.NotificationTypeLabelKey: notificationType}
		opts := client.MatchingLabels(labels)
		notificationList := &toolchainv1alpha1.NotificationList{}
		if err := a.Client.List(context.TODO(), notificationList, opts); err != nil {
			return false, err
		}
		return len(notificationList.Items) == 0, nil
	})
}

// UntilNotificationHasConditions checks if Notification status has the given set of conditions
func UntilNotificationHasConditions(expected ...toolchainv1alpha1.Condition) NotificationWaitCriterion {
	return NotificationWaitCriterion{
		Match: func(actual toolchainv1alpha1.Notification) bool {
			return test.ConditionsMatch(actual.Status.Conditions, expected...)
		},
		Diff: func(actual toolchainv1alpha1.Notification) string {
			return fmt.Sprintf("expected Notification conditions to match:\n%s", Diff(expected, actual.Status.Conditions))
		},
	}
}

// ToolchainStatusWaitCriterion a struct to compare with an expected ToolchainStatus
type ToolchainStatusWaitCriterion struct {
	Match func(*toolchainv1alpha1.ToolchainStatus) bool
	Diff  func(*toolchainv1alpha1.ToolchainStatus) string
}

func matchToolchainStatusWaitCriterion(actual *toolchainv1alpha1.ToolchainStatus, criteria ...ToolchainStatusWaitCriterion) bool {
	for _, c := range criteria {
		if !c.Match(actual) {
			return false
		}
	}
	return true
}

func (a *HostAwaitility) printToolchainStatusWaitCriterionDiffs(actual *toolchainv1alpha1.ToolchainStatus, criteria ...ToolchainStatusWaitCriterion) {
	buf := &strings.Builder{}
	if actual == nil {
		buf.WriteString("failed to find Toolchainstatus\n")
	} else {
		buf.WriteString("failed to find ToolchainStatus with matching criteria:\n")
		buf.WriteString("actual:\n")
		y, _ := StringifyObject(actual)
		buf.Write(y)
		buf.WriteString("\n----\n")
		buf.WriteString("diffs:\n")
		for _, c := range criteria {
			if !c.Match(actual) {
				buf.WriteString(c.Diff(actual))
				buf.WriteString("\n")
			}
		}
	}
	// also include other resources relevant in the host namespace, to help troubleshooting
	buf.WriteString(a.sprintAllResources())

	a.T.Log(buf.String())
}

// UntilToolchainStatusHasConditions returns a `ToolchainStatusWaitCriterion` which checks that the given
// ToolchainStatus has exactly all the given status conditions
func UntilToolchainStatusHasConditions(expected ...toolchainv1alpha1.Condition) ToolchainStatusWaitCriterion {
	return ToolchainStatusWaitCriterion{
		Match: func(actual *toolchainv1alpha1.ToolchainStatus) bool {
			return test.ConditionsMatch(actual.Status.Conditions, expected...)
		},
		Diff: func(actual *toolchainv1alpha1.ToolchainStatus) string {
			return fmt.Sprintf("expected ToolchainStatus conditions to match:\n%s", Diff(expected, actual.Status.Conditions))
		},
	}
}

// UntilToolchainStatusUpdated returns a `ToolchainStatusWaitCriterion` which checks that the
// ToolchainStatus ready condition was updated after the given time
func UntilToolchainStatusUpdatedAfter(t time.Time) ToolchainStatusWaitCriterion {
	return ToolchainStatusWaitCriterion{
		Match: func(actual *toolchainv1alpha1.ToolchainStatus) bool {
			cond, found := condition.FindConditionByType(actual.Status.Conditions, toolchainv1alpha1.ConditionReady)
			return found && t.Before(cond.LastUpdatedTime.Time)
		},
		Diff: func(actual *toolchainv1alpha1.ToolchainStatus) string {
			cond, found := condition.FindConditionByType(actual.Status.Conditions, toolchainv1alpha1.ConditionReady)
			if !found {
				return fmt.Sprintf("expected ToolchainStatus ready conditions to updated after %s, but it was not found: %v", t.String(), actual.Status.Conditions)
			}
			return fmt.Sprintf("expected ToolchainStatus ready conditions to updated after %s, but is: %v", t.String(), cond.LastUpdatedTime)
		},
	}
}

// UntilAllMembersHaveUsageSet returns a `ToolchainStatusWaitCriterion` which checks that the given
// ToolchainStatus has all members with some non-zero resource usage
func UntilAllMembersHaveUsageSet() ToolchainStatusWaitCriterion {
	return ToolchainStatusWaitCriterion{
		Match: func(actual *toolchainv1alpha1.ToolchainStatus) bool {
			for _, member := range actual.Status.Members {
				if !hasMemberStatusUsageSet(member.MemberStatus) {
					return false
				}
			}
			return true
		},
		Diff: func(actual *toolchainv1alpha1.ToolchainStatus) string {
			a, _ := yaml.Marshal(actual.Status.Members)
			return fmt.Sprintf("expected all status members to have usage set. Actual: %s", a)
		},
	}
}

func UntilAllMembersHaveAPIEndpoint(apiEndpoint string) ToolchainStatusWaitCriterion {
	return ToolchainStatusWaitCriterion{
		Match: func(actual *toolchainv1alpha1.ToolchainStatus) bool {
			//Since all member operators currently run in the same cluster in the e2e test environment, then using the same memberCluster.Spec.APIEndpoint for all the member clusters should be fine.
			for _, member := range actual.Status.Members {
				// check Member field ApiEndpoint is assigned
				if member.APIEndpoint != apiEndpoint {
					return false
				}
			}
			return true
		},
		Diff: func(actual *toolchainv1alpha1.ToolchainStatus) string {
			a, _ := yaml.Marshal(actual.Status.Members)
			return fmt.Sprintf("expected all status members to have API Endpoint '%s'. Actual: %s", apiEndpoint, a)
		},
	}
}

func UntilProxyURLIsPresent(proxyURL string) ToolchainStatusWaitCriterion {
	return ToolchainStatusWaitCriterion{
		Match: func(actual *toolchainv1alpha1.ToolchainStatus) bool {
			return strings.TrimSuffix(actual.Status.HostRoutes.ProxyURL, "/") == strings.TrimSuffix(proxyURL, "/")
		},
		Diff: func(actual *toolchainv1alpha1.ToolchainStatus) string {
			return fmt.Sprintf("Proxy endpoint in the ToolchainStatus doesn't match. Expected: '%s'. Actual: %s", proxyURL, actual.Status.HostRoutes.ProxyURL)
		},
	}
}

// UntilHasMurCount returns a `ToolchainStatusWaitCriterion` which checks that the given
// ToolchainStatus has the given count of MasterUserRecords
func UntilHasMurCount(domain string, expectedCount int) ToolchainStatusWaitCriterion {
	return ToolchainStatusWaitCriterion{
		Match: func(actual *toolchainv1alpha1.ToolchainStatus) bool {
			murs, ok := actual.Status.Metrics[toolchainv1alpha1.MasterUserRecordsPerDomainMetricKey]
			if !ok {
				return false
			}
			return murs[domain] == expectedCount
		},
		Diff: func(actual *toolchainv1alpha1.ToolchainStatus) string {
			murs, ok := actual.Status.Metrics[toolchainv1alpha1.MasterUserRecordsPerDomainMetricKey]
			if !ok {
				return "MasterUserRecordPerDomain metric not found"
			}
			return fmt.Sprintf("expected MasterUserRecordPerDomain metric to be %d. Actual: %d", expectedCount, murs[domain])
		},
	}
}

// WaitForToolchainStatus waits until the ToolchainStatus is available with the provided criteria, if any
func (a *HostAwaitility) WaitForToolchainStatus(criteria ...ToolchainStatusWaitCriterion) (*toolchainv1alpha1.ToolchainStatus, error) {
	// there should only be one toolchain status with the name toolchain-status
	name := "toolchain-status"
	toolchainStatus := &toolchainv1alpha1.ToolchainStatus{}
	err := wait.Poll(a.RetryInterval, 2*a.Timeout, func() (done bool, err error) {
		obj := &toolchainv1alpha1.ToolchainStatus{}
		// retrieve the toolchainstatus from the host namespace
		err = a.Client.Get(context.TODO(),
			types.NamespacedName{
				Namespace: a.Namespace,
				Name:      name,
			},
			obj)
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		toolchainStatus = obj
		return matchToolchainStatusWaitCriterion(toolchainStatus, criteria...), nil
	})
	// no match found, print the diffs
	if err != nil {
		a.printToolchainStatusWaitCriterionDiffs(toolchainStatus, criteria...)
	}
	return toolchainStatus, err
}

// GetToolchainConfig returns ToolchainConfig instance, nil if not found
func (a *HostAwaitility) GetToolchainConfig() *toolchainv1alpha1.ToolchainConfig {
	config := &toolchainv1alpha1.ToolchainConfig{}
	if err := a.Client.Get(context.TODO(), test.NamespacedName(a.Namespace, "config"), config); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		require.NoError(a.T, err)
	}
	return config
}

// ToolchainConfigWaitCriterion a struct to compare with an expected ToolchainConfig
type ToolchainConfigWaitCriterion struct {
	Match func(*toolchainv1alpha1.ToolchainConfig) bool
	Diff  func(*toolchainv1alpha1.ToolchainConfig) string
}

func matchToolchainConfigWaitCriterion(actual *toolchainv1alpha1.ToolchainConfig, criteria ...ToolchainConfigWaitCriterion) bool {
	for _, c := range criteria {
		if !c.Match(actual) {
			return false
		}
	}
	return true
}

func (a *HostAwaitility) printToolchainConfigWaitCriterionDiffs(actual *toolchainv1alpha1.ToolchainConfig, criteria ...ToolchainConfigWaitCriterion) {
	buf := &strings.Builder{}
	if actual == nil {
		buf.WriteString("failed to find ToolchainConfig\n")
	} else {
		buf.WriteString("failed to find ToolchainConfig with matching criteria:\n")
		buf.WriteString("actual:\n")
		y, _ := StringifyObject(actual)
		buf.Write(y)
		buf.WriteString("\n----\n")
		buf.WriteString("diffs:\n")
		for _, c := range criteria {
			if !c.Match(actual) {
				buf.WriteString(c.Diff(actual))
				buf.WriteString("\n")
			}
		}
	}
	// also include other resources relevant in the host namespace, to help troubleshooting
	buf.WriteString(a.sprintAllResources())

	a.T.Log(buf.String())
}

func UntilToolchainConfigHasSyncedStatus(expected toolchainv1alpha1.Condition) ToolchainConfigWaitCriterion {
	return ToolchainConfigWaitCriterion{
		Match: func(actual *toolchainv1alpha1.ToolchainConfig) bool {
			return test.ContainsCondition(actual.Status.Conditions, expected)
		},
		Diff: func(actual *toolchainv1alpha1.ToolchainConfig) string {
			e, _ := yaml.Marshal(expected)
			a, _ := yaml.Marshal(actual.Status.Conditions)
			return fmt.Sprintf("expected conditions to contain: %s.\n\tactual: %s", e, a)
		},
	}
}

// WaitForToolchainConfig waits until the ToolchainConfig is available with the provided criteria, if any
func (a *HostAwaitility) WaitForToolchainConfig(criteria ...ToolchainConfigWaitCriterion) (*toolchainv1alpha1.ToolchainConfig, error) {
	// there should only be one ToolchainConfig with the name "config"
	name := "config"
	var toolchainConfig *toolchainv1alpha1.ToolchainConfig
	err := wait.Poll(a.RetryInterval, 2*a.Timeout, func() (done bool, err error) {
		obj := &toolchainv1alpha1.ToolchainConfig{}
		// retrieve the ToolchainConfig from the host namespace
		if err := a.Client.Get(context.TODO(),
			types.NamespacedName{
				Namespace: a.Namespace,
				Name:      name,
			},
			obj); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		toolchainConfig = obj
		return matchToolchainConfigWaitCriterion(toolchainConfig, criteria...), nil
	})
	// no match found, print the diffs
	if err != nil {
		a.printToolchainConfigWaitCriterionDiffs(toolchainConfig, criteria...)
	}
	return toolchainConfig, err
}

// UpdateToolchainConfig updates the current resource of the ToolchainConfig CR with the given options.
// If there is no existing resource already, then it creates a new one.
// At the end of the test it returns the resource back to the original value/state.
func (a *HostAwaitility) UpdateToolchainConfig(options ...testconfig.ToolchainConfigOption) {
	var originalConfig *toolchainv1alpha1.ToolchainConfig
	// try to get the current ToolchainConfig
	config := a.GetToolchainConfig()
	if config == nil {
		// if it doesn't exist, then create a new one
		config = &toolchainv1alpha1.ToolchainConfig{
			ObjectMeta: v1.ObjectMeta{
				Namespace: a.Namespace,
				Name:      "config",
			},
		}
	} else {
		// if it exists then create a copy to store the original values
		originalConfig = config.DeepCopy()
	}

	// modify using the given options
	for _, option := range options {
		option.Apply(config)
	}

	// if it didn't exist before
	if originalConfig == nil {
		// then create a new one
		err := a.Client.Create(context.TODO(), config)
		require.NoError(a.T, err)

		// and as a cleanup function delete it at the end of the test
		a.T.Cleanup(func() {
			err := a.Client.Delete(context.TODO(), config)
			if err != nil && !errors.IsNotFound(err) {
				require.NoError(a.T, err)
			}
		})
		return
	}

	// if the config did exist before the tests, then update it
	err := a.updateToolchainConfigWithRetry(config)
	require.NoError(a.T, err)

	// and as a cleanup function update it back to the original value
	a.T.Cleanup(func() {
		config := a.GetToolchainConfig()
		// if the current config wasn't found
		if config == nil {
			if originalConfig != nil {
				// then create it back with the original values
				err := a.Client.Create(context.TODO(), originalConfig)
				require.NoError(a.T, err)
			}
		} else {
			// otherwise just update it
			err := a.updateToolchainConfigWithRetry(originalConfig)
			require.NoError(a.T, err)
		}
	})
}

// updateToolchainConfigWithRetry attempts to update the toolchainconfig, helpful because the toolchainconfig controller updates the toolchainconfig
// resource periodically which can cause errors like `Operation cannot be fulfilled on toolchainconfigs.toolchain.dev.openshift.com "config": the object has been modified; please apply your changes to the latest version and try again`
// in some cases. Retrying mitigates the potential for test flakiness due to this behaviour.
func (a *HostAwaitility) updateToolchainConfigWithRetry(updatedConfig *toolchainv1alpha1.ToolchainConfig) error {
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		config := a.GetToolchainConfig()
		config.Spec = updatedConfig.Spec
		if err := a.Client.Update(context.TODO(), config); err != nil {
			a.T.Logf("Retrying ToolchainConfig update due to error: %s", err.Error())
			return false, nil
		}
		return true, nil
	})
	return err
}

// GetHostOperatorPod returns the pod running the host operator controllers
func (a *HostAwaitility) GetHostOperatorPod() (corev1.Pod, error) {
	pods := corev1.PodList{}
	if err := a.Client.List(context.TODO(), &pods, client.InNamespace(a.Namespace), client.MatchingLabels{"control-plane": "controller-manager"}); err != nil {
		return corev1.Pod{}, err
	}
	if len(pods.Items) != 1 {
		return corev1.Pod{}, fmt.Errorf("unexpected number of pods with label 'control-plane=controller-manager' in namespace '%s': %d ", a.Namespace, len(pods.Items))
	}
	return pods.Items[0], nil
}

// CreateAPIProxyClient creates a client to the appstudio api proxy using the given user token
func (a *HostAwaitility) CreateAPIProxyClient(usertoken string) client.Client {
	apiConfig, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	require.NoError(a.T, err)
	defaultConfig, err := clientcmd.NewDefaultClientConfig(*apiConfig, &clientcmd.ConfigOverrides{}).ClientConfig()
	require.NoError(a.T, err)

	s := scheme.Scheme
	builder := append(runtime.SchemeBuilder{}, corev1.AddToScheme)
	require.NoError(a.T, builder.AddToScheme(s))

	proxyKubeConfig := &rest.Config{
		Host:            a.APIProxyURL,
		TLSClientConfig: defaultConfig.TLSClientConfig,
		BearerToken:     usertoken,
	}
	proxyCl, err := client.New(proxyKubeConfig, client.Options{
		Scheme: s,
	})
	require.NoError(a.T, err)
	return proxyCl
}

type SpaceWaitCriterion struct {
	Match func(*toolchainv1alpha1.Space) bool
	Diff  func(*toolchainv1alpha1.Space) string
}

func matchSpaceWaitCriterion(actual *toolchainv1alpha1.Space, criteria ...SpaceWaitCriterion) bool {
	for _, c := range criteria {
		if !c.Match(actual) {
			return false
		}
	}
	return true
}

// WaitForSpace waits until the Space with the given name is available with the provided criteria, if any
func (a *HostAwaitility) WaitForSpace(name string, criteria ...SpaceWaitCriterion) (*toolchainv1alpha1.Space, error) {
	a.T.Logf("waiting for Space '%s' with matching criteria", name)
	var space *toolchainv1alpha1.Space
	err := wait.Poll(a.RetryInterval, 2*a.Timeout, func() (done bool, err error) {
		obj := &toolchainv1alpha1.Space{}
		// retrieve the Space from the host namespace
		if err := a.Client.Get(context.TODO(),
			types.NamespacedName{
				Namespace: a.Namespace,
				Name:      name,
			},
			obj); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		space = obj
		return matchSpaceWaitCriterion(space, criteria...), nil
	})
	// no match found, print the diffs
	if err != nil {
		a.printSpaceWaitCriterionDiffs(space, criteria...)
	}
	return space, err
}

func (a *HostAwaitility) printSpaceWaitCriterionDiffs(actual *toolchainv1alpha1.Space, criteria ...SpaceWaitCriterion) {
	buf := &strings.Builder{}
	if actual == nil {
		buf.WriteString("failed to find Space\n")
	} else {
		buf.WriteString("failed to find Space with matching criteria:\n")
		buf.WriteString("----\n")
		buf.WriteString("actual:\n")
		y, _ := StringifyObject(actual)
		buf.Write(y)
		buf.WriteString("\n----\n")
		buf.WriteString("diffs:\n")
		for _, c := range criteria {
			if !c.Match(actual) {
				buf.WriteString(c.Diff(actual))
				buf.WriteString("\n")
			}
		}
	}
	// also include Spaces resources in the host namespace, to help troubleshooting
	a.listAndPrint("Spaces", a.Namespace, &toolchainv1alpha1.SpaceList{})
	a.T.Log(buf.String())
}

// UntilSpaceIsBeingDeleted checks if Space has Deletion Timestamp
func UntilSpaceIsBeingDeleted() SpaceWaitCriterion {
	return SpaceWaitCriterion{
		Match: func(actual *toolchainv1alpha1.Space) bool {
			return actual.DeletionTimestamp != nil
		},
	}
}

// UntilSpaceHasLabelWithValue returns a `SpaceWaitCriterion` which checks that the given
// Space has the expected label with the given value
func UntilSpaceHasLabelWithValue(key, value string) SpaceWaitCriterion {
	return SpaceWaitCriterion{
		Match: func(actual *toolchainv1alpha1.Space) bool {
			return actual.Labels[key] == value
		},
		Diff: func(actual *toolchainv1alpha1.Space) string {
			return fmt.Sprintf("expected space to contain label %s:%s:\n%s", key, value, spew.Sdump(actual.Labels))
		},
	}
}

// UntilSpaceHasCreationTimestampOlderThan returns a `SpaceWaitCriterion` which checks that the given
// Space has a timestamp that has elapsed the provided difference duration
func UntilSpaceHasCreationTimestampOlderThan(expectedElapsedTime time.Duration) SpaceWaitCriterion {
	return SpaceWaitCriterion{
		Match: func(actual *toolchainv1alpha1.Space) bool {
			t := time.Now().Add(expectedElapsedTime)
			return t.After(actual.CreationTimestamp.Time)
		},
		Diff: func(actual *toolchainv1alpha1.Space) string {
			return fmt.Sprintf("expected space to be created after %s; Actual creation timestamp %s", expectedElapsedTime.String(), actual.CreationTimestamp.String())
		},
	}
}

// UntilSpaceHasTier returns a `SpaceWaitCriterion` which checks that the given
// Space has the expected tier name set in its Spec
func UntilSpaceHasTier(expected string) SpaceWaitCriterion {
	return SpaceWaitCriterion{
		Match: func(actual *toolchainv1alpha1.Space) bool {
			return actual.Spec.TierName == expected
		},
		Diff: func(actual *toolchainv1alpha1.Space) string {
			return fmt.Sprintf("expected tier name to match:\n%s", Diff(expected, actual.Spec.TierName))
		},
	}
}

// UntilSpaceHasConditions returns a `SpaceWaitCriterion` which checks that the given
// Space has exactly all the given status conditions
func UntilSpaceHasConditions(expected ...toolchainv1alpha1.Condition) SpaceWaitCriterion {
	return SpaceWaitCriterion{
		Match: func(actual *toolchainv1alpha1.Space) bool {
			return test.ConditionsMatch(actual.Status.Conditions, expected...)
		},
		Diff: func(actual *toolchainv1alpha1.Space) string {
			return fmt.Sprintf("expected conditions to match:\n%s", Diff(expected, actual.Status.Conditions))
		},
	}
}

// UntilSpaceHasStateLabel returns a `SpaceWaitCriterion` which checks that the
// Space has the expected value of the state label
func UntilSpaceHasStateLabel(expected string) SpaceWaitCriterion {
	return SpaceWaitCriterion{
		Match: func(actual *toolchainv1alpha1.Space) bool {
			return actual.Labels != nil && actual.Labels[toolchainv1alpha1.SpaceStateLabelKey] == expected
		},
		Diff: func(actual *toolchainv1alpha1.Space) string {
			return fmt.Sprintf("expected Space to match the state label value: %s \nactual labels: %s", expected, actual.Labels)
		},
	}
}

// UntilSpaceHasConditionForTime returns a `SpaceWaitCriterion` which checks that the given
// Space has the condition set at least for the given amount of time
func UntilSpaceHasConditionForTime(expected toolchainv1alpha1.Condition, duration time.Duration) SpaceWaitCriterion {
	return SpaceWaitCriterion{
		Match: func(actual *toolchainv1alpha1.Space) bool {
			foundCond, exists := condition.FindConditionByType(actual.Status.Conditions, expected.Type)
			if exists && foundCond.Reason == expected.Reason && foundCond.Status == expected.Status {
				return foundCond.LastTransitionTime.Time.Before(time.Now().Add(-duration))
			}
			return false
		},
		Diff: func(actual *toolchainv1alpha1.Space) string {
			return fmt.Sprintf("expected conditions to match:\n%s\nAnd having the LastTransitionTime %s or older", Diff(expected, actual.Status.Conditions), time.Now().Add(-duration).String())
		},
	}
}

// UntilSpaceHasAnyTargetClusterSet returns a `SpaceWaitCriterion` which checks that the given
// Space has any `spec.targetCluster` set
func UntilSpaceHasAnyTargetClusterSet() SpaceWaitCriterion {
	return SpaceWaitCriterion{
		Match: func(actual *toolchainv1alpha1.Space) bool {
			return actual.Spec.TargetCluster != ""
		},
		Diff: func(actual *toolchainv1alpha1.Space) string {
			return fmt.Sprintf("expected target clusters not to be empty. Actual Space resource:\n%v", actual)
		},
	}
}

// UntilSpaceHasAnyTierNameSet returns a `SpaceWaitCriterion` which checks that the given
// Space has any `spec.tierName` set
func UntilSpaceHasAnyTierNameSet() SpaceWaitCriterion {
	return SpaceWaitCriterion{
		Match: func(actual *toolchainv1alpha1.Space) bool {
			return actual.Spec.TierName != ""
		},
		Diff: func(actual *toolchainv1alpha1.Space) string {
			return fmt.Sprintf("expected tier name not to be empty. Actual Space resource:\n%v", actual)
		},
	}
}

// UntilSpaceHasStatusTargetCluster returns a `SpaceWaitCriterion` which checks that the given
// Space has the expected `targetCluster` in its status
func UntilSpaceHasStatusTargetCluster(expected string) SpaceWaitCriterion {
	return SpaceWaitCriterion{
		Match: func(actual *toolchainv1alpha1.Space) bool {
			return actual.Status.TargetCluster == expected
		},
		Diff: func(actual *toolchainv1alpha1.Space) string {
			return fmt.Sprintf("expected status target clusters to match:\n%s", Diff(expected, actual.Status.TargetCluster))
		},
	}
}

// WaitUntilSpaceAndSpaceBindingsDeleted waits until the Space with the given name and its associated SpaceBindings are deleted (ie, not found)
func (a *HostAwaitility) WaitUntilSpaceAndSpaceBindingsDeleted(name string) error {
	a.T.Logf("waiting until Space '%s' in namespace '%s' is deleted", name, a.Namespace)
	var s *toolchainv1alpha1.Space
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		obj := &toolchainv1alpha1.Space{}
		if err := a.Client.Get(context.TODO(),
			types.NamespacedName{
				Namespace: a.Namespace,
				Name:      name,
			}, obj); err != nil {
			if errors.IsNotFound(err) {
				// once the space is deleted, wait for the associated spacebindings to be deleted as well
				if err := a.WaitUntilSpaceBindingsWithLabelDeleted(toolchainv1alpha1.SpaceBindingSpaceLabelKey, name); err != nil {
					return false, err
				}
				return true, nil
			}
			return false, err
		}
		s = obj
		return false, nil
	})
	if err != nil {
		y, _ := yaml.Marshal(s)
		a.T.Logf("Space '%s' was not deleted as expected: %s", name, y)
		return err
	}
	return nil
}

// WaitUntilSpaceBindingDeleted waits until the SpaceBinding with the given name is deleted (ie, not found)
func (a *HostAwaitility) WaitUntilSpaceBindingDeleted(name string) error {

	return wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		mur := &toolchainv1alpha1.SpaceBinding{}
		if err := a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: name}, mur); err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	})
}

// WaitUntilSpaceBindingsWithLabelDeleted waits until there are no SpaceBindings listed using the given labels
func (a *HostAwaitility) WaitUntilSpaceBindingsWithLabelDeleted(key, value string) error {
	labels := map[string]string{key: value}
	a.T.Logf("waiting until SpaceBindings with labels '%v' in namespace '%s' are deleted", labels, a.Namespace)
	var spaceBindingList *toolchainv1alpha1.SpaceBindingList
	err := wait.Poll(a.RetryInterval, 2*a.Timeout, func() (done bool, err error) {
		// retrieve the SpaceBinding from the host namespace
		spaceBindingList = &toolchainv1alpha1.SpaceBindingList{}
		if err = a.Client.List(context.TODO(), spaceBindingList, client.MatchingLabels(labels), client.InNamespace(a.Namespace)); err != nil {
			return false, err
		}
		return len(spaceBindingList.Items) == 0, nil
	})
	// print the listed spacebindings
	if err != nil {
		buf := &strings.Builder{}
		buf.WriteString(fmt.Sprintf("spacebindings still found with labels %v:\n", labels))
		for _, sb := range spaceBindingList.Items {
			y, _ := yaml.Marshal(sb)
			buf.Write(y)
			buf.WriteString("\n")
		}
	}
	return err
}

type SpaceBindingWaitCriterion struct {
	Match func(*toolchainv1alpha1.SpaceBinding) bool
	Diff  func(*toolchainv1alpha1.SpaceBinding) string
}

func matchSpaceBindingWaitCriterion(actual *toolchainv1alpha1.SpaceBinding, criteria ...SpaceBindingWaitCriterion) bool {
	for _, c := range criteria {
		if !c.Match(actual) {
			return false
		}
	}
	return true
}

// WaitForSpaceBinding waits until the SpaceBinding with the given MUR and Space names is available with the provided criteria, if any
func (a *HostAwaitility) WaitForSpaceBinding(murName, spaceName string, criteria ...SpaceBindingWaitCriterion) (*toolchainv1alpha1.SpaceBinding, error) {
	var spacebinding *toolchainv1alpha1.SpaceBinding
	labels := map[string]string{
		toolchainv1alpha1.SpaceBindingMasterUserRecordLabelKey: murName,
		toolchainv1alpha1.SpaceBindingSpaceLabelKey:            spaceName,
	}

	err := wait.Poll(a.RetryInterval, 2*a.Timeout, func() (done bool, err error) {
		// retrieve the SpaceBinding from the host namespace
		spaceBindingList := &toolchainv1alpha1.SpaceBindingList{}
		if err = a.Client.List(context.TODO(), spaceBindingList, client.MatchingLabels(labels), client.InNamespace(a.Namespace)); err != nil {
			return false, err
		}
		if len(spaceBindingList.Items) == 0 {
			return false, nil
		}
		spacebinding = &spaceBindingList.Items[0]
		return matchSpaceBindingWaitCriterion(spacebinding, criteria...), nil
	})
	// no match found, print the diffs
	if err != nil {
		a.printSpaceBindingWaitCriterionDiffs(spacebinding, criteria...)
	}
	return spacebinding, err
}

func (a *HostAwaitility) printSpaceBindingWaitCriterionDiffs(actual *toolchainv1alpha1.SpaceBinding, criteria ...SpaceBindingWaitCriterion) {
	buf := &strings.Builder{}
	if actual == nil {
		buf.WriteString("failed to find SpaceBinding\n")
	} else {
		buf.WriteString("failed to find SpaceBinding with matching criteria:\n")
		buf.WriteString("----\n")
		buf.WriteString("actual:\n")
		y, _ := StringifyObject(actual)
		buf.Write(y)
		buf.WriteString("\n----\n")
		buf.WriteString("diffs:\n")
		for _, c := range criteria {
			if !c.Match(actual) {
				buf.WriteString(c.Diff(actual))
				buf.WriteString("\n")
			}
		}
	}
	// also include SpaceBindings resources in the host namespace, to help troubleshooting
	a.listAndPrint("SpaceBindings", a.Namespace, &toolchainv1alpha1.SpaceBindingList{})
	a.T.Log(buf.String())
}

func (a *HostAwaitility) ListSpaceBindings(spaceName string) ([]toolchainv1alpha1.SpaceBinding, error) {
	bindings := &toolchainv1alpha1.SpaceBindingList{}
	if err := a.Client.List(context.TODO(), bindings, client.InNamespace(a.Namespace), client.MatchingLabels{
		toolchainv1alpha1.SpaceBindingSpaceLabelKey: spaceName,
	}); err != nil {
		return nil, err
	}
	return bindings.Items, nil
}

// UntilSpaceBindingHasMurName returns a `SpaceBindingWaitCriterion` which checks that the given
// SpaceBinding has the expected MUR name set in its Spec
func UntilSpaceBindingHasMurName(expected string) SpaceBindingWaitCriterion {
	return SpaceBindingWaitCriterion{
		Match: func(actual *toolchainv1alpha1.SpaceBinding) bool {
			return actual.Spec.MasterUserRecord == expected
		},
		Diff: func(actual *toolchainv1alpha1.SpaceBinding) string {
			return fmt.Sprintf("expected MUR name to match:\n%s", Diff(expected, actual.Spec.MasterUserRecord))
		},
	}
}

// UntilSpaceBindingHasSpaceName returns a `SpaceBindingWaitCriterion` which checks that the given
// SpaceBinding has the expected MUR name set in its Spec
func UntilSpaceBindingHasSpaceName(expected string) SpaceBindingWaitCriterion {
	return SpaceBindingWaitCriterion{
		Match: func(actual *toolchainv1alpha1.SpaceBinding) bool {
			return actual.Spec.Space == expected
		},
		Diff: func(actual *toolchainv1alpha1.SpaceBinding) string {
			return fmt.Sprintf("expected Space name to match:\n%s", Diff(expected, actual.Spec.Space))
		},
	}
}

// UntilSpaceBindingHasSpaceRole returns a `SpaceBindingWaitCriterion` which checks that the given
// SpaceBinding has the expected SpaceRole name set in its Spec
func UntilSpaceBindingHasSpaceRole(expected string) SpaceBindingWaitCriterion {
	return SpaceBindingWaitCriterion{
		Match: func(actual *toolchainv1alpha1.SpaceBinding) bool {
			return actual.Spec.SpaceRole == expected
		},
		Diff: func(actual *toolchainv1alpha1.SpaceBinding) string {
			return fmt.Sprintf("expected Space role to match:\n%s", Diff(expected, actual.Spec.SpaceRole))
		},
	}
}

const (
	DNS1123NameMaximumLength         = 63
	DNS1123NotAllowedCharacters      = "[^-a-z0-9]"
	DNS1123NotAllowedStartCharacters = "^[^a-z0-9]+"
)

func EncodeUserIdentifier(subject string) string {
	// Convert to lower case
	encoded := strings.ToLower(subject)

	// Remove all invalid characters
	nameNotAllowedChars := regexp.MustCompile(DNS1123NotAllowedCharacters)
	encoded = nameNotAllowedChars.ReplaceAllString(encoded, "")

	// Remove invalid start characters
	nameNotAllowedStartChars := regexp.MustCompile(DNS1123NotAllowedStartCharacters)
	encoded = nameNotAllowedStartChars.ReplaceAllString(encoded, "")

	// Add a checksum prefix if the encoded value is different to the original subject value
	if encoded != subject {
		encoded = fmt.Sprintf("%x-%s", crc32.Checksum([]byte(subject), crc32.IEEETable), encoded)
	}

	// Trim if the length exceeds the maximum
	if len(encoded) > DNS1123NameMaximumLength {
		encoded = encoded[0:DNS1123NameMaximumLength]
	}

	return encoded
}
