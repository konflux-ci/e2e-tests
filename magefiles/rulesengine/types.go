package rulesengine

import (
	"fmt"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/onsi/ginkgo/v2/types"
	"k8s.io/klog"
)

type RuleEngine map[string]map[string]RuleCatalog

func (e *RuleEngine) ListCatagoriesOfCatalogs() string {

	var ty []string
	for k := range *e {

		ty = append(ty, k)
	}

	return strings.Join(ty, ",")

}

func (e *RuleEngine) ListCatalogsByCategory(cat string) (string, error) {

	var cats []string
	found := false

	for k, v := range *e {

		if k == cat {
			found = true
			for k := range v {

				cats = append(cats, k)
			}
		}
	}

	if !found {
		return "", fmt.Errorf("%s is not a category registered in the engine", cat)
	}

	return strings.Join(cats, ","), nil

}

func (e *RuleEngine) RunRules(rctx *RuleCtx, args ...string) error {

	var fullCatalogs RuleCatalog
	foundCat := false
	foundCtl := false
	for k, v := range *e {

		if len(args) >= 1 {

			if k != args[0] {
				continue
			}
			foundCat = true
			for k, v := range v {

				if len(args) == 2 && foundCat {
					if k != args[1] {
						continue
					}
					fullCatalogs = append(fullCatalogs, v...)
					foundCtl = true
					klog.Infof("Loading the catalog for, %s, from category, %s", args[1], args[0])
					break

				} else {
					klog.Infof("Loading the catalogs for category %s", args[0])
					fullCatalogs = append(fullCatalogs, v...)

				}

			}
		} else {
			for _, v := range v {

				fullCatalogs = append(fullCatalogs, v...)
			}
		}
	}

	if !foundCat && len(args) == 1 {
		return fmt.Errorf("%s is not a category registered in the engine", args[0])
	}

	if !foundCtl && len(args) == 2 {
		return fmt.Errorf("%s is not a catalog registered in the engine", args[1])
	}

	return e.runLoadedCatalog(fullCatalogs, rctx)

}

func (e *RuleEngine) RunRulesOfCategory(cat string, rctx *RuleCtx) error {

	var fullCatalogs RuleCatalog
	found := false
	for k, v := range *e {

		if k == cat {
			found = true
			for _, v := range v {

				fullCatalogs = append(fullCatalogs, v...)
			}
		}
	}

	if !found {
		return fmt.Errorf("%s is not a category registered in the engine", cat)
	}

	return e.runLoadedCatalog(fullCatalogs, rctx)

}

func (e *RuleEngine) runLoadedCatalog(loaded RuleCatalog, rctx *RuleCtx) error {

	var matched RuleCatalog
	for _, rule := range loaded {
		ok, err := rule.Eval(rctx)
		if err != nil {
			return err
		}
		// In most cases, a rule chain has no action to execute
		// since a majority of the actions are encapsulated
		// within the rules that compose the chain.
		if len(rule.Actions) == 0 {
			// In case the rule chain condition was evaluated to true,
			// it means that the rule was applied, so stop iterating over next catalog rules.
			// Otherwise continue
			if ok {
				return nil
			}
			continue
		}
		if ok {
			matched = append(matched, rule)
		}
	}

	if len(matched) == 0 {

		return nil
	}

	klog.Infof("The following rules have matched %s.", matched.String())
	if rctx.DryRun {

		return e.dryRun(matched, rctx)

	}

	return e.run(matched, rctx)

}

func (e *RuleEngine) dryRun(matched RuleCatalog, rctx *RuleCtx) error {

	klog.Info("DryRun has been enabled will apply them in dry run mode")
	for _, rule := range matched {

		return rule.DryRun(rctx)

	}

	return nil
}

func (e *RuleEngine) run(matched RuleCatalog, rctx *RuleCtx) error {

	klog.Info("Will apply rules")
	for _, rule := range matched {

		err := rule.Apply(rctx)

		if err != nil {
			klog.Errorf("Failed to execute rule: %s", rule.String())
			return err
		}

	}

	return nil
}

type RuleCatalog []Rule

func (rc *RuleCatalog) String() string {

	var names []string
	for _, r := range *rc {

		names = append(names, r.Name)
	}

	return strings.Join(names, ",")
}

type Action interface {
	Execute(rctx *RuleCtx) error
}

type Conditional interface {
	Check(rctx *RuleCtx) (bool, error)
}

type Any []Conditional

func (a Any) Check(rctx *RuleCtx) (bool, error) {

	// Initial logic was to pass on the first
	// eval to true but that might not be the
	// case. So not eval all and as long as any
	// eval true then filter is satisfied
	isAny := false

	for _, c := range a {
		ok, err := c.Check(rctx)
		if err != nil {
			return false, err
		}
		if ok {
			isAny = true
		}
	}

	return isAny, nil
}

type All []Conditional

func (a All) Check(rctx *RuleCtx) (bool, error) {

	for _, c := range a {

		ok, err := c.Check(rctx)
		if err != nil {
			return ok, err
		}
		if !ok {
			return ok, nil
		}
	}

	return true, nil
}

type None []Conditional

func (a None) Check(rctx *RuleCtx) (bool, error) {

	for _, c := range a {

		ok, err := c.Check(rctx)
		if err != nil {
			return ok, err
		}
		if ok {
			return !ok, nil
		}
	}

	return true, nil
}

type ActionFunc func(rctx *RuleCtx) error

func (af ActionFunc) Execute(rctx *RuleCtx) error {

	return af(rctx)
}

type ConditionFunc func(rctx *RuleCtx) (bool, error)

func (cf ConditionFunc) Check(rctx *RuleCtx) (bool, error) {
	return cf(rctx)
}

type Rule struct {
	Name        string
	Description string
	Condition   Conditional
	Actions     []Action
}

func (r *Rule) String() string {

	return fmt.Sprintf("%s: %s", r.Name, r.Description)
}

type IRule interface {
	Eval(rctx *RuleCtx) (bool, error)
	Apply(rctx *RuleCtx) error
	DryRun(rctx *RuleCtx) error
}

func (r *Rule) Eval(rctx *RuleCtx) (bool, error) {

	return r.Condition.Check(rctx)
}

func (r *Rule) Apply(rctx *RuleCtx) error {

	for _, action := range r.Actions {

		err := action.Execute(rctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Rule) DryRun(rctx *RuleCtx) error {

	rctx.DryRun = true
	for _, action := range r.Actions {

		err := action.Execute(rctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Rule) Check(rctx *RuleCtx) (bool, error) {

	ok, err := r.Eval(rctx)
	if err != nil {
		return false, err
	}
	if ok {
		if rctx.DryRun {
			return true, r.DryRun(rctx)
		}
		if err := r.Apply(rctx); err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}

type File struct {
	Status string
	Name   string
}

type Files []File

func (cfs *Files) FilterByDirString(filter string) Files {

	var subfiles Files

	for _, file := range *cfs {

		if !strings.Contains(file.Name, filter) {
			continue
		}

		subfiles = append(subfiles, file)
	}

	return subfiles

}

func (cfs *Files) FilterByDirGlob(filter string) Files {

	var subfiles Files

	for _, file := range *cfs {

		if matched, _ := doublestar.PathMatch(filter, file.Name); !matched {

			continue
		}

		subfiles = append(subfiles, file)
	}

	return subfiles

}

func (cfs *Files) FilterByStatus(filter string) Files {

	var subfiles Files

	for _, file := range *cfs {

		if !strings.Contains(file.Status, strings.ToUpper(filter)) {
			continue
		}

		subfiles = append(subfiles, file)

	}

	return subfiles

}

func (cfs *Files) String() string {

	var names []string
	for _, f := range *cfs {

		names = append(names, f.Name)
	}

	return strings.Join(names, ", ")
}

type IRuleCtx interface {
	AddRuleData(key string, obj any) error
	GetRuleData(key string) any
}

type RuleCtx struct {
	types.CLIConfig
	types.SuiteConfig
	types.ReporterConfig
	types.GoFlagsConfig
	RuleData                      map[string]any
	RepoName                      string
	ComponentImageTag             string
	ComponentEnvVarPrefix         string
	JobName                       string
	JobType                       string
	DiffFiles                     Files
	IsPaired                      bool
	RequiredBinaries              []string
	PrRemoteName                  string
	PrCommitSha                   string
	PrBranchName                  string
	TektonEventType               string
	RequiresMultiPlatformTests    bool
	RequiresSprayProxyRegistering bool
}

func NewRuleCtx() *RuleCtx {

	var suiteConfig = types.NewDefaultSuiteConfig()
	var reporterConfig = types.NewDefaultReporterConfig()
	var cliConfig = types.NewDefaultCLIConfig()
	var goFlagsConfig = types.NewDefaultGoFlagsConfig()
	var envData map[string]any = make(map[string]any)

	r := &RuleCtx{cliConfig,
		suiteConfig,
		reporterConfig,
		goFlagsConfig,
		envData,
		"",
		"",
		"",
		"",
		"",
		Files{},
		false,
		make([]string, 0),
		"",
		"",
		"",
		"",
		false,
		false}

	//init defaults we've used so far
	t, _ := time.ParseDuration("90m")
	r.Timeout = t
	r.OutputInterceptorMode = "none"

	return r

}

func (gca *RuleCtx) AddRuleData(key string, obj any) error {

	gca.RuleData[key] = obj

	return nil
}
func (gca *RuleCtx) GetRuleData(key string) any {

	// When retrieving data the implementing function that needs
	// it will have to cast it to the appropriate type
	if v, ok := gca.RuleData[key]; ok {

		return v
	}

	return nil

}
