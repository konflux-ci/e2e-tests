# Test Labels

We would like to also have tests distinguished by labels, not only by folders for specific components. We can also run the tests only with specific label(s).

## Usage

See: https://onsi.github.io/ginkgo/#spec-labels 

## Types of labels
- component
- test type
- category

### Component Labels

Component Labels | Description
--- | --- 
has | HAS related tests
build | Build related tests
cluster-registration | cluster registration related tests
e2e-demos | e2e-demos related tests
pipeline | Pipeline Service related tests

### Test Types Labels

Test Types Labels | Description
--- | --- 
slow | Slow tests (See --slow-spec-threshold (https://onsi.github.io/ginkgo/#other-settings))
e2e-demos | e2e demos tests
load | Load tests
performance | Performance related tests
smoke | Critical functionality tests
serial | Tests which can’t be run in parallel
security | Security related tests

### Test Stability Labels

Test Types Labels | Description
--- | --- 
flaky | Flaky tests (all tests are stable unless marked flaky)

### Test Categorization Labels

Test Types Labels | Description
--- | --- 
customer-feedback | Test created upon feedback from any customer channel (customer issue, telemetry data, …)
demo | Tests related to milestone demos
