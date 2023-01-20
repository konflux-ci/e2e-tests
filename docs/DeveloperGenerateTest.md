# Generating Tests

Before generating tests you should be aware of the following environment variables that are used to render new test files from a template:

| Variable | Required | Explanation | Default Value |
|---|---|---|---|
| `TEMPLATE_SUITE_PACKAGE_NAME` | yes | The name of the package directory and spec file created in `tests/` and the name used when creating the suite file in `cmd/`  | EMPTY  |
| `TEMPLATE_SPEC_FILE_NAME` | no | It will override the name of the test spec file and the text within the `Describe/It` containers. Useful when you are looking to test a different component within a service. Refer to `tests/build/` for a good example.    | `template` |
| `TEMPLATE_APPEND_FRAMEWORK_DESCRIBE_FILE` | no | By default, when the test spec file is generated, the root `Describe` container function is appended to the `pkg/framework/describe.go` file, using the value of either `TEMPLATE_SUITE_PACKAGE_NAME` or `TEMPLATE_SPEC_FILE_NAME`, so it can be utilized in the current e2e test suite. In some cases, due the type of testing, you may not want to perform this action. | `true`  |
| `TEMPLATE_JOIN_SUITE_SPEC_FILE_NAMES` | no | It will join the values of `TEMPLATE_SUITE_PACKAGE_NAME` and `TEMPLATE_SPEC_FILE_NAME` to be used for the `pkg/framework/describe.go` root `Describe` container function and in all text within the Gingko container text. An example might be that a `tests/chaos` package directory is created and you want the test spec file to be named per component, i.e. `build.go`, but you want the test functions and container text to be more descriptive so that they say `ChaosBuildSuiteDescribe`. | `false` |

 
 ## Generating Gingko Test Spec File
  
 ```bash
   $ make local/template/generate-test-spec
   ./mage -v local:generateTestSpecFile
	Running target: Local:GenerateTestSpecFile
	I1129 10:34:56.777665   11171 magefile.go:360] TEMPLATE_SPEC_FILE_NAME not set. Defaulting test spec file to value of `chaos`.
	I1129 10:34:56.777923   11171 magefile.go:379] Creating new test package directory and spec file tests/chaos/chaos.go.
	exec: go "fmt" "tests/chaos/chaos.go"
	tests/chaos/chaos.go

 ```
 As noted above, this command will create a new package under the `tests/`directory and a test spec file `<SpecFile>.go` for you. It will contain some basic imports but more importantly it will generate a basic structured Ginkgo spec like the one below. You can update as necessary and build upon it.

 ```golang
 package chaos

/* This was generated from a template file. Please feel free to update as necessary */

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	//framework imports edit as required
)

var _ = framework.ChaosSuiteDescribe("Chaos tests", Label("Chaos"), func() {


	defer GinkgoRecover()
	var err error
	var f *framework.Framework
	// use 'f' to access common controllers or the specific service controllers within the framework
	BeforeAll(func() {
		// Initialize the tests controllers
		f, err = framework.NewFramework()
		Expect(err).NotTo(HaveOccurred())
	})


	Describe("Chaos scenario to test", Label("Chaos"), func() {
		// Declare variables here.

		BeforeEach(func() {

			// Initialize variables here.
			// Assert setup here.

		})

		It("chaos does some test action", func() {

			// Implement test and assertions here

		})

	})

})

 ```

 ## Generating Ginkgo Test Suite File

 ```bash
   $ make local/template/generate-test-suite 
   ./mage -v local:generateTestSuiteFile
   Running target: Local:GenerateTestSuiteFile
   I1128 13:21:30.908854    5645 magefile.go:311] Creating new test suite file cmd/chaos_test.go.
   exec: go "fmt" "cmd/chaos_test.go"
   cmd/chaos_test.go
```

This command will help setup a test suite file within the `cmd/` directory. It will do the test package import based on the value of `TEMPLATE_SUITE_PACKAGE_NAME`. So using the example above it will assume there is a `tests/chaos` package to import as well. It uses a simplified version of the `cmd/e2e_test.go` as a template to allow you to leverage the existing functionality built into the framework like webhooks eventing and Polarion formatted XML test case generation. Edit this file as you feel necessary.

NOTE: You may not need to generate this file. This is useful when you want to move a type of testing into a separate suite that wouldn't go into the existing e2e test suite package. i.e. chaos testing. We have a current example with the existing `cmd/loadsTest.go` which are used the run the AppStudio Load tests.
