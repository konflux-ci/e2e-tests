package godog_tests

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"

	"github.com/cucumber/godog"
)

type TestContext struct {
	GinkgoTestSuitePath string
	Concurrency         int
}

type Result struct {
	Index  int
	Output string
	Error  error
}

func actor(resultChan <-chan Result, ginkgoTestOutputs *[]string, errors *[]error) {
	for result := range resultChan {
		(*ginkgoTestOutputs)[result.Index] = result.Output
		(*errors)[result.Index] = result.Error
	}
}

func TestMain(m *testing.M) {
	status := godog.TestSuite{
		Name: "godogs",
		TestSuiteInitializer: func(ctx *godog.TestSuiteContext) {
			ctx.BeforeSuite(func() {
				// Any setup you need before the test suite runs
			})
			ctx.AfterSuite(func() {
				// Any cleanup you need after the test suite runs
			})
		},
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			testCtx := &TestContext{}
			testCtx.FeatureContext(ctx)
		},
		Options: &godog.Options{
			Output:    os.Stdout,
			Format:    "pretty",
			Randomize: 0,
			Paths:     []string{"./sample.feature"},
		},
	}.Run()

	if st := m.Run(); st > status {
		status = st
	}
	os.Exit(status)
}

func (ctx *TestContext) FeatureContext(s *godog.ScenarioContext) {
	var ginkgoTestOutputs []string
	var errors []error

	s.Step(`^Ginkgo test suite "([^"]*)" is available$`, func(path string) error {
		ctx.GinkgoTestSuitePath = path
		_, err := os.Stat(ctx.GinkgoTestSuitePath)
		if err != nil {
			return fmt.Errorf("ginkgo test suite not found: %v", err)
		}
		return nil
	})

	s.Step(`^I run the Ginkgo test suite (\d+) times concurrently$`, func(concurrency int) error {
		ctx.Concurrency = concurrency
		errors = make([]error, ctx.Concurrency)
		ginkgoTestOutputs = make([]string, ctx.Concurrency)

		var wg sync.WaitGroup
		wg.Add(ctx.Concurrency)

		resultChan := make(chan Result, ctx.Concurrency)

		go actor(resultChan, &ginkgoTestOutputs, &errors)

		for i := 0; i < ctx.Concurrency; i++ {
			go func(index int) {
				defer wg.Done()
				// cmd := exec.Command("ginkgo", ctx.GinkgoTestSuitePath)
				cmd := exec.Command("go", "test", "-v", ctx.GinkgoTestSuitePath)
				output, err := cmd.CombinedOutput()
				resultChan <- Result{Index: index, Output: string(output), Error: err}
			}(i)
		}

		wg.Wait()
		close(resultChan)

		// Print Ginkgo test suite output - if wanting the regular ginkgo test suite output too
		for _, output := range ginkgoTestOutputs {
			fmt.Println(output)
		}

		for _, err := range errors {
			if err != nil {
				for _, output := range ginkgoTestOutputs {
					fmt.Println(output)
				}
				return err
			}
		}

		return nil
	})

	s.Step(`^Ginkgo test suite should pass$`, func() error {
		for i, err := range errors {
			if err != nil {
				return fmt.Errorf("ginkgo test suite failed: %v\n%s", err, ginkgoTestOutputs[i])
			}
		}
		return nil
	})
}
