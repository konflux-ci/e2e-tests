# Mage Engine

Mage Engine is a rules engine framework with the intent to take the existing business 
logic that dictates configuring the environment and test execution we have 
accumulated over time in our magefile and organize them better.

The core of what drives the rules will still be the functions we write that check for 
specific conditions to be met and actions to take on said conditions. The framework 
just allows us to take those higher order functions and compose them together into
more descriptive rules.

There is NO INTENT to be a full fledged rules engine to be able to evaluate very complex expressions
or assign priority to rules if multiple rules evaluate to true. As of right now it wasn't obvious
if we had such complex requirements based on the data we use.

## Architecture

The following Architecture of MageEngine. 

### Rule

The core of the engine is a `Rule`. A Rule describes and executes the business logic. It is composed 
of a `Conditional` which implements the checking of data that should be evaluated to `true`. 
Once a conditional of a `Rule` evaulates true, the `Rule` can take an `Action`. The action implements 
the execution of the task the rule should take.

A `Rule` implements the `IRule` interface: 
 * To evaluate the registered condition the engine calls `Eval()` on the rule.
 * To take action when evaluation is true the engine calls `Apply()` on the rule.
 * To simulate an action on a rule the engine can call `DryRun()` (DryRun needs to be set on the `RuleCtx` 
   for the framework to make this call.)

### Rule Context
a `RuleCtx` is the context object to insert data into and gets passed around so that rules can evaluate and take action. In our use case we have very specific key pieces of data that triggers our business logic so 
this isn't meant for generalized use cases.

That said, the `RuleCtx` implements a `AddRuleData(key string, obj any)` and `GetRuleData(key string)`
functions that would allow a developer to store other types of data. It is important to note that
the writer of a conditional or action that depends on data stored in this manner will have to make 
sure they type cast the returned object appropriately in their implementation.

### Conditionals

`Conditional` is an interface that is meant to perform a check of the passed in data. A conditional can 
be:
 * a full fledged object/struct that implements the `Check()` function 
 * any anonymous or higher order function as long as it is registered in the Rule as a `ConditionalFunc`

#### Filters

To make constructing `Rules` with some complex conditionals easier the framework implements a couple of filters which implement the `Conditional` interface:

 * All: `Conditional|ConditionalFunc` registered will evaluate that all evaluated to `true`
 * Any: `Conditional|ConditionalFunc` registered will evaluate that one evaluated to `true`
 * None: `Conditional|ConditionalFunc` registered will evaluate that all have NOT evaluated to `true`. The absensce of `true`

### Actions

`Action` is an interface that is meant to execute on the passed in data. Like a conditional, an action can 
be:
 * a full fledged object/struct that implements the `Execute()` function
 * any anonymous or higher order function as long as it is regisered in the Rule as a `ActionFunc`

### RuleCatalog

This is really a collection/slice of Rules. You create a catalog and register the catalog with the engine.
As of now most of the Business Logic really is tied by repo. So catalogs are created by repo/domain

### Engine 

It is a map of a map. The idea was that each repo would create a catalog of Rules. Those Catalogs could be registered 
as map to a `category`. Using the example of test execution, a category could be `tests`. 
Then under `tests` we regsiter a map, where a key is the repo/domain, i.e. `e2e-repo` and its test catalog is assigned to it.  

### RuleChain

A `RuleChain` is more of a concept than it is an actual type. A rulechain is a `Rule` type but it also implements
the `Conditional` interface. So what does that mean? A rule can now be composed of other rules by registering them 
as conditionals.

Since Rules are conditionals as well. The evaluation process is a little different. When a RuleChain evaluates a rule
it will run the `Eval()` and `Apply()|DryRun()`. Both those calls have to execute without fail for the Rule to
evaluate true. 

This will help reuse existing rules to create broader flows.


 ## Example of constructing Rules/RulesCatalogs/RuleChains

 ### Creating a basic rule using anonymous functions and its rule catalog

 In this example, we create a rule for infra-deployments test execution using anonymous condition and action functions. Some 
 conditions and actions may be so simple that it is easier to implement as anonymous functions. I then define this Rule within
 a Rule Catalog. 

 The logic in this rule is straightforward: WHEN the RepoName is infra-deployments, THEN execute ginkgo with the specific label filter.

 ```go
 // Example rule for infra-deployments using anonymous functions embedded in the rule
var InfraDeploymentsTestRulesCatalog = rulesengine.RuleCatalog{
	rulesengine.Rule{Name: "Infra Deployments Default Test Execution",
		Description: "Run the default test suites which include the demo and components suites.",
		Condtion:    rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {
			if rctx.RepoName == "infra-deployments" {
				return true
			}
			return false
		}),
		Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
			rctx.LabelFilter = "e2e-demo,rhtap-demo,spi-suite,remote-secret,integration-service,ec,build-templates,multi-platform"
			return ExecuteTestAction(rctx)
		})}},

} 
 
 ```

 ### Registering a rule catalog to the engine

 Here I've created a catagory called tests within the MageEngine and I've assigned infra-deployments catalog to the infra-deployments key
 under the tests category

 ```go
 var MageEngine = rulesengine.RuleEngine{
	"tests": {
		"infra-deployments": testselection.InfraDeploymentsTestRulesCatalog,
	},
}
 ```

 ### Executing the MageEngine within a mage file

 This will cause the engine to load and iterate over the rules of the category test and apply any that had their conditions match.

 ```go

    rctx := rulesengine.NewRuleCtx()
	rctx.DryRun = true

	err = engine.MageEngine.RunRulesOfCategory("tests", rctx)
 ```

 ### Creating a complex rule using a mixture of conditional filters

 In this example, the release-catalog-repo has a slightly more complex set of rules where depending on certain conditions
 met by the CI job the test should execute with a different set of ginkgo label filters. So you will see a mixture of the 
 Any/All/None conditional filters

 The first rule's logic is: WHEN EITHER RepoName equals release-service-catalog AND NOT pr paired OR RepoName equals release-service-catalog
 AND is a rehearse job AND NOT pr paired THEN run ginkgo with release-pipelines label filter

 The second rule's logic is: WHEN RepoName equals release-service-catalog AND pr is paired AND NOT a rehearse job THEN run ginkgo with 
 release-pipelines && !fbc-tests label filters


 ```go

 rulesengine.Rule{Name: "Release Catalog Test Execution",
		Description: "Runs release catalog tests on release-service-catalog repo on pull/rehearsal jobs.",
		Condtion:    rulesengine.Any{rulesengine.All{rulesengine.ConditionFunc(releaseCatalogRepoCondition), rulesengine.None{rulesengine.ConditionFunc(isPaired)}}, rulesengine.All{rulesengine.ConditionFunc(releaseCatalogRepoCondition), rulesengine.ConditionFunc(isRehearse)}, rulesengine.None{rulesengine.ConditionFunc(isPaired)}},
		Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
			rctx.LabelFilter = "release-pipelines"
			return ExecuteTestAction(rctx)
		})}},
rulesengine.Rule{Name: "Release Catalog PR paired Test Execution",
		Description: "Runs release catalog tests except for the fbc tests on release-service-catalog repo when PR paired and not a rehearsal job.",
		Condtion:    rulesengine.All{rulesengine.ConditionFunc(releaseCatalogRepoCondition), rulesengine.ConditionFunc(isPaired), rulesengine.None{rulesengine.ConditionFunc(isRehearse)}},
		Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
			rctx.LabelFilter = "release-pipelines && !fbc-tests"
			return ExecuteTestAction(rctx)
		})}},

...

var isRehearse = func(rctx *rulesengine.RuleCtx) bool {
	if strings.Contains(rctx.JobName, "rehearse") {
		return true
	}

	return false
}

// Demo of func isPRPairingRequired() for testing purposes
var isPaired = func(rctx *rulesengine.RuleCtx) bool {
	if true {
		return true
	}

	return false
}


func releaseCatalogRepoCondition(rctx *rulesengine.RuleCtx) bool {

	if rctx.RepoName == "release-service-catalog" {
		return true
	}

	return false

}
```

### Creating a Rule Chain

You can refer to `testselection/rule_demo.go` to see an example. There we reorganize 
the `LocalE2E` mage function which calls `PreflightCheck`, `BootstrapCluster`, `RunE2ETest`, into
a rule chain composed of a set of 3 rules.

You can run this demo through mage by running `./mage -v local:runRuleDemo`

