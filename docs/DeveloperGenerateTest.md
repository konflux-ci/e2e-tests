# Generating Tests

## Context

We rely on Ginkgo and Gomega as the core of our E2E functional test framework. Ginkgo allows you to write expressive BDD style tests that act as specifications by example. The intent is to allow a team to write specifications, which we call a spec outline, that look like this:

```
BookSuiteDescribe: Book service E2E tests
    Describe: Categorizing book length @book
        When: the book has more than 300 pages @slow
            It: should be a novel
        When: the book has fewer than 300 pages @fast
            It: should be a short story

    Describe: Creating bookmarks in a book @book, @bookmark, @parallel
        It: has no bookmarks by default
        It: can add bookmarks

```

and be able to generate skeleton Ginkgo Test Files that look like this:

```golang
package books

/* This was generated from a template file. Please feel free to update as necessary!
   a couple things to note:
    - Remember to implement specific logic of the service/domain you are trying to test if it not already there in the pkg/

    - To include the tests as part of the E2E Test suite:
       - Update the pkg/framework/describe.go to include the `Describe func` of this new test suite, If you haven't already done so.
       - Import this new package into the cmd/e2e_test.go
*/

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	//framework imports edit as required
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
)

var _ = framework.BookSuiteDescribe("Book service E2E tests", func() {

	defer GinkgoRecover()
	var err error
	var f *framework.Framework
	// use 'f' to access common controllers or the specific service controllers within the framework
	BeforeAll(func() {
		// Initialize the tests controllers
		f, err = framework.NewFramework()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Categorizing book length  ", Label("book"), func() {
		// Declare variables here.

		When("the book has more than 300 pages  ", Label("slow"), func() {

			It("should be a novel", func() {

				// Implement test and assertions here

			})

		})

		When("the book has fewer than 300 pages  ", Label("fast"), func() {

			It("should be a short story", func() {

				// Implement test and assertions here

			})

		})

	})

	Describe("Creating bookmarks in a book  ", Label("book"), Label("bookmark"), Label("parallel"), func() {
		// Declare variables here.

		It("has no bookmarks by default", func() {

			// Implement test and assertions here

		})

		It("can add bookmarks", func() {

			// Implement test and assertions here

		})

	})

})

```

or parse Ginkgo Test Files that have large amounts of code, like [build.go](../tests/build/build.go), and generate spec outlines that looks like this.

```
BuildSuiteDescribe: Build service E2E tests @build, @HACBS
  Describe: test PaC component build @github-webhook, @pac-build, @pipeline
    When: a new component without specified branch is created @pac-custom-default-branch
      It: correctly targets the default branch (that is not named 'main') with PaC
      It: triggers a PipelineRun
      It: a related PipelineRun and Github webhook should be deleted after deleting the component
      It: PR branch should not exists in the repo
    When: a new component with specified custom branch branch is created
      It: triggers a PipelineRun
      It: should lead to a PaC init PR creation
      It: the PipelineRun should eventually finish successfully
      It: eventually leads to a creation of a PR comment with the PipelineRun status report
    When: the PaC init branch is updated
      It: eventually leads to triggering another PipelineRun
      It: PipelineRun should eventually finish
      It: eventually leads to another update of a PR with a comment about the PipelineRun status report
    When: the PaC init branch is merged
      It: eventually leads to triggering another PipelineRun
      It: pipelineRun should eventually finish
    When: the component is removed and recreated (with the same name in the same namespace)
      It: should no longer lead to a creation of a PaC PR

  Describe: Creating component with container image source
    It: should not trigger a PipelineRun

  Describe: PLNSRVCE-799 - test pipeline selector @pipeline-selector
    It: a specific Pipeline bundle should be used and additional pipeline params should be added to the PipelineRun if all WhenConditions match
    It: default Pipeline bundle should be used and no additional Pipeline params should be added to the PipelineRun if one of the WhenConditions does not match

  Describe: A secret with dummy quay.io credentials is created in the testing namespace
    It: should override the shared secret
    It: should not be possible to push to quay.io repo (PipelineRun should fail)

```

## How it works

We leverage existing Ginkgo's tool set to be able to do this translation back and forth. 
* `ginkgo outline` we use to be able to generate the initial spec outline for our internal model based on what is in the file Ginkgo Test File. This command depends on the GoLang AST to generates the output. 
* `ginkgo generate` we use to pass a customize template and data so that it can render a spec file using Ginkgo's extensive use of closures to allow us to build a descriptive spec hierarchy. 

## Schema

The text outline file must be in the following format:

 * Each line MUST be in key/value format using `:` as delimiter
    * Each key MUST be a Ginkgo DSL word for Container and Subject nodes, `Describe/When/Context/It/By/DescribeTable/Entry`
    * The value is essentially the description text of the container
 * All lines MUST be nested, by using spaces, to represent the logical tree hierarchy of the specification
 * The first line MUST be a framework decorator function type `Describe` node that will get implemented in `/pkg/framework/describe.go`
 * To assign Labels: 
 		* each string intended to be a label MUST be prefixed with `@`
    * the set of labels MUST be a comma separated list
    * they MUST be assigned AFTER the description text
 * When using the `DescribeTable` key, the proceeding nested lines MUST have the `Entry` or `By` key or Ginkgo will not render the template properly

For the time being we don't support any of Ginkgo's Setup/Teardown nodes. We could technically graph it together from the text outline but it won't render with our base template. The important thing is to expressively model the behavior to test. Test developers will be able to insert Setup/Teardown nodes where they see fit when the spec has been rendered. 


## Prerequisite

Before generating anything make sure you have Ginkgo in place. To install Ginkgo:

`go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo`

## Team specific template
Teams can create their own templates according to their needs.
These should utilize the provided `specs.tmpl` by including the following line wherever they want their specs to be. (do not forget the dot!) 

`{{ template "specs" . }}`

Everything needed to get started is in the [templates/default](../templates/default) directory.

Please see the provided [recommended](../templates/default/recommended.tmpl) and [barebones](../templates/default/barebones.tmpl) templates.
Copy them and make your own.
 
 ## Usage

 ### Printing a text outline of an existing ginkgo spec file
 This will generate the outline and output to your terminal. If you would like to save the outline to a file refer to the next section.

`./mage PrintOutlineOfGinkgoSpec tests/<subdirectory>/<test-file>.go`

```bash
$ ./mage PrintOutlineOfGinkgoSpec tests/books/books.go
tests/books/books.goI0622 22:11:30.391491   19273 testcasemapper.go:26] Mapping outline from a Ginkgo test file, tests/books/books.go
I0622 22:11:30.395240   19273 testcasemapper.go:40] Printing outline:

BookSuiteDescribe: Book service E2E tests
  Describe: Categorizing book length  @book
    When: the book has more than 300 pages  @slow
      It: Should be a novel
    When: the book has fewer than 300 pages  @fast
      It: should be a short story

  Describe: Creating bookmarks in a book  @book, @bookmark, @parallel
    It: Has no bookmarks by default
    It: Can add bookmarks

  DescribeTable: Reading invalid books always errors is table
    Entry: Empty book
    Entry: Only title
    Entry: Only author
    Entry: Missing pages

```
### Generating the text outline file from an existing ginkgo spec file
This will generate the outline and output to a text file to the desired file location

`./mage GenerateTextOutlineFromGinkgoSpec tests/<subdirectory>/<test-file>.go <dest>/<sub-path>/<file>`

```bash
$ ./mage GenerateTextOutlineFromGinkgoSpec tests/books/books.go /tmp/outlines/books.outline
I0622 22:19:41.857966   20923 testcasemapper.go:26] Mapping outline from a Ginkgo test file, tests/books/books.go
I0622 22:19:41.865725   20923 testcasemapper.go:70] Mapping outline to a text file, /tmp/outlines/books.outline
I0622 22:19:41.865837   20923 textspec.go:67] successfully written to /tmp/outlines/books.outline

```
 
 ### Generating a generic Ginkgo spec file from an existing text outline file
 This will generate a generic Ginkgo spec in a subdirectory within our tests directory

 `./mage GenerateGinkgoSpecFromTextOutline <path>/<to>/<outline> <subpath-under-tests>/<filename>.go`

 ```bash
 $ ./mage GenerateGinkoSpecFromTextOutline dummy_test.outline books/books.go
I0622 22:14:22.140583   20356 testcasemapper.go:58] Mapping outline from a text file, dummy_test.outline
I0622 22:14:22.140673   20356 testcasemapper.go:47] Mapping outline to a Ginkgo test file, books/books.go
I0622 22:14:22.140841   20356 ginkgosspec.go:242] Creating new test package directory and spec file tests/books/books.go.
 ```
As noted above, this command will create a new package under the `tests/` directory and a test spec file `<filename>.go` for you. It will contain some basic imports but more importantly it will generate a basic structured Ginkgo spec skeleton that you can code against.

### Generating a team specific Ginkgo spec file from an existing text outline file
This will generate the Ginkgo spec in a subdirectory within our tests directory using a team specific template provided by user. Please see the [Team specific template](#team-specific-template) section.

Feel free to use the provided [testOutline](../templates/default/testOutline) file for testing.

`./mage GenerateTeamSpecificGinkgoSpecFromTextOutline <path>/<to>/<outline> <path>/<to>/<team-specific-template> <subpath-under-tests>/<filename>.go`

```bash
➜ ./mage GenerateTeamSpecificGinkgoSpecFromTextOutline templates/default/testOutline templates/default/recommended.tmpl tests/template_poc/template_poc.go
I0219 15:42:17.808595  351210 magefile.go:755] Mapping outline from a text file, templates/default/testOutline
I0219 15:42:17.808685  351210 magefile.go:762] Mapping outline to a Ginkgo spec file, tests/template_poc/template_poc.go
I0219 15:42:17.809210  351210 ginkgosspec.go:144] Creating new test package directory and spec file /home/tnevrlka/Work/e2e-tests/tests/template_poc/template_poc.go.
```

### Printing a text outline in JSON format of an existing ginkgo spec file
 This will generate the outline and output to your terminal in JSON format. This is the format we use when rendering the template. You can pipe this output to tools like `jq` for formatting and filtering. This would only be useful for troubleshooting purposes 

`./mage PrintJsonOutlineOfGinkgoSpec tests/<subdirectory>/<test-file>.go`

```bash
$ ./mage PrintJsonOutlineOfGinkgoSpec tests/books/books.go 
I0622 22:28:15.661455   23214 testcasemapper.go:26] Mapping outline from a Ginkgo test file, tests/books/books.go
[{"Name":"BookSuiteDescribe","Text":"Book service E2E tests","Labels":[],"Nodes":[{"Name":"Describe","Text":"Categorizing book length ","Labels":["book"],"Nodes":[{"Name":"When","Text":"the book has more than 300 pages ","Labels":["slow"],"Nodes":[{"Name":"It","Text":"Should be a novel","Labels":[],"Nodes":[],"InnerParentContainer":false,"LineSpaceLevel":0}],"InnerParentContainer":false,"LineSpaceLevel":0},{"Name":"When","Text":"the book has fewer than 300 pages ","Labels":["fast"],"Nodes":[{"Name":"It","Text":"should be a short story","Labels":[],"Nodes":[],"InnerParentContainer":false,"LineSpaceLevel":0}],"InnerParentContainer":false,"LineSpaceLevel":0}],"InnerParentContainer":true,"LineSpaceLevel":0},{"Name":"Describe","Text":"Creating bookmarks in a book ","Labels":["book","bookmark","parallel"],"Nodes":[{"Name":"It","Text":"Has no bookmarks by default","Labels":[],"Nodes":[],"InnerParentContainer":false,"LineSpaceLevel":0},{"Name":"It","Text":"Can add bookmarks","Labels":[],"Nodes":[],"InnerParentContainer":false,"LineSpaceLevel":0}],"InnerParentContainer":true,"LineSpaceLevel":0},{"Name":"DescribeTable","Text":"Reading invalid books always errors is table","Labels":[],"Nodes":[{"Name":"Entry","Text":"Empty book","Labels":[],"Nodes":[],"InnerParentContainer":false,"LineSpaceLevel":0},{"Name":"Entry","Text":"Only title","Labels":[],"Nodes":[],"InnerParentContainer":false,"LineSpaceLevel":0},{"Name":"Entry","Text":"Only author","Labels":[],"Nodes":[],"InnerParentContainer":false,"LineSpaceLevel":0},{"Name":"Entry","Text":"Missing pages","Labels":[],"Nodes":[],"InnerParentContainer":false,"LineSpaceLevel":0}],"InnerParentContainer":true,"LineSpaceLevel":0}],"InnerParentContainer":false,"LineSpaceLevel":0}] 

``` 

### Printing a text outline of an existing text outline file
 This will generate the outline and output to your terminal. This would only be useful for troubleshooting purposes. i.e. To make sure complex text outlines are graphed properly in the tree.

`./mage PrintOutlineOfTextSpec <path>/<to>/<outline-file>`

```bash
$ ./mage PrintOutlineOfTextSpec /tmp/outlines/books.outline
I0622 22:32:34.429681   23787 testcasemapper.go:58] Mapping outline from a text file, /tmp/outlines/books.outline
I0622 22:32:34.429771   23787 testcasemapper.go:40] Printing outline:

BookSuiteDescribe: Book service E2E tests
  Describe: Categorizing book length   @book
    When: the book has more than 300 pages   @slow
      It: Should be a novel
    When: the book has fewer than 300 pages   @fast
      It: should be a short story
  Describe: Creating bookmarks in a book   @book, @bookmark, @parallel
    It: Has no bookmarks by default
    It: Can add bookmarks
  DescribeTable: Reading invalid books always errors is table
    Entry: Empty book
    Entry: Only title
    Entry: Only author
    Entry: Missing pages

``` 

### Updating the pkg framework describe file

Once you are comfortable with your test you can update the framework/describe.go in our package directory.

`./mage AppendFrameworkDescribeGoFile tests/<test-package>/<specfile>.go`

```bash
$ ./mage AppendFrameworkDescribeGoFile tests/books/books.go
I0623 11:56:42.997793   20862 magefile.go:670] Inspecting Ginkgo spec file, tests/books/books.go
pkg/framework/describe.go

```

 ### Generating Ginkgo Test Suite File

 This command will help setup a test suite file within the `cmd/` directory. It will do the test package import based on the name of the package you passed in. So using the example below it will assume there is a `tests/chaos` package to import as well. It uses a simplified version of the `cmd/e2e_test.go` as a template to allow you to leverage the existing functionality built into the framework like webhooks eventing. Edit this file as you feel necessary.

NOTE: You may not need to generate this file. This is useful when you want to move a type of testing into a separate suite that wouldn't go into the existing e2e test suite package. i.e. chaos testing. We have a current example with the existing `cmd/loadsTest.go` which are used to run the AppStudio Load tests.

`./mage GenerateTestSuiteFile <name of test package under tests directory>`

 ```bash
$ ./mage GenerateTestSuiteFile chaos
I0623 12:48:13.761038   31196 magefile.go:467] Creating new test suite file cmd/chaos_test.go.
cmd/chaos_test.go

```
