Feature: Run Ginkgo test suite

  Scenario: Run Ginkgo test suite concurrently
    Given Ginkgo test suite "../books" is available
    When I run the Ginkgo test suite 100 times concurrently
    Then Ginkgo test suite should pass
