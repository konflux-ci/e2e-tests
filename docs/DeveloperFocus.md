# Running the e2e suite from a developer perspective

## Prerequisites

For setting up your cluster to run the e2e tests locally against, follow the [Installation guide](./Installation.md).

Note that it is recommended to install AppStudio in e2e mode, but the e2e suite should also be usable in development and preview modes.

## Building the e2e suite

Whenever changes are made to the underlying e2e code, a new e2e suite binary needs to be built. From the e2e-tests directory run:

   ```bash
      make build
   ```

The resulting e2e-appstudio binary is located in the bin folder.

## Focused testing

The e2e suite is written using the [Ginkgo test framework](https://onsi.github.io/ginkgo/) and, therefore, benefits from all the default flags Ginkgo provides.

Running the following command will provide all the flags available when running the suite:

   ```bash
      ./bin/e2e-appstudio -h
   ```

Of interest to a developer are the `--ginkgo.focus` and `--ginkgo.focus-file` flags.

`--ginkgo.focus` accepts a regular expression string to match against for any of the strings used in a test "descriptors" (such as Context, It, When, etc). Multiple `--ginkgo.focus` flags can be provided on the command line. When run with the focus flags the suite with OR the flags and run only the matching tests, skipping the rest.

`--ginkgo.focus-file` can be used to specify a specific .go file (such as `tests/build/build.go`). Multiple `--ginkgo.focus-file` flags can be provided on the command line. When run with this flag the suite will only run the tests specified in that file, skipping the rest.

By using these or the other flags that Ginkgo provides you can run e2e-appstudio (after rebuilding to include your changes) focused on the changes that you have made to ensure they are working before you commit your code.