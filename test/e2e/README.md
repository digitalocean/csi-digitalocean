# end-to-end testing

It is possible to run the SIG Storage-managed end-to-end tests provided by upstream Kubernetes to validate the correctness of the DigitalOcean CSI driver. This document describes how the end-to-end tests are structured and how they can be performed using the files in this directory.

## Introduction

The Kubernetes project maintains a set of end-to-end tests that are used to validate the correctness of the upstream development. Since Kubernetes 1.14, these tests can also be consumed externally by provisioning a Kubernetes cluster to run the tests against.

This also holds for the storage-related tests but comes with a caveat: the CSI specification gives a lot of flexibility as to which features may be implemented by a drivers, with the implication that not all drivers may implement all functionality that is being verified by the end-to-end tests. Fortunately, the external storage tests provide a mechanism to declare the set of features implemented by a driver-under-test, and the testing framework takes this information into account to selectively choose tests during runtime.

Our end-to-end testing integration customizes the test configuration for DigitalOcean's CSI driver and provides tooling to easily perform the testing.

## Components

The end-to-end test integration consists of the following components:

- a Dockerfile bundling Kubernetes version-specific dependencies and tools to drive test execution through a test runner image
- a set of Kubernetes release-specific testdriver YAML configuration files for our CSI driver
- a set of Go files preparing the right test environment and triggering the test execution
- a shell wrapper start script

Details for each component follow below.

### Dockerfile

The Dockerfile bundles a fork of the Kubernetes end-to-end test binary for each supported Kubernetes version along with some tooling. The resulting test runner image can be invoked to perform the end-to-end tests.

The fork is needed for all Kubernetes versions that do not ship with [this PR](https://github.com/kubernetes/kubernetes/pull/86000) since the tests otherwise fail. (See the PR description for details.) They are maintained on the <https://github.com/digitalocean/kubernetes> repository in the `do-doks-<Kubernetes major-minor version>` branches, e.g., `do-doks-1.16`. Each branch is based on the upstream branch `origin/release-<Kubernetes major-minor version>`; see the git history in our repository to check what the latest upstream HEAD for each forked branch is. In the Dockerfile, we point to the exact commit hash for each branch to enable deterministic builds.
Forks will be phased out as we are able to use Kubernetes releases that ship with our patch.

A test binary is needed for each Kubernetes version since upstream does not offer backwards compatibility for older versions; that is, the test binary for a given version is guaranteed to work with that version only.

We also ship ginkgo and a few other tools needed to run the tests. See the Dockerfile comments for details.

### Testdrivers

External storage tests for CSI drivers require a so-called _testdriver_ specification, which is a YAML file describing the capabilities of a particular driver. Roughly speaking, it codifies the set of supported features, which the upstream test framework takes into account to dynamically determine the set of tests to run.

For posterity, the steps required to define a testdriver and run the tests [are documented upstream](https://github.com/kubernetes/kubernetes/tree/master/test/e2e/storage/external). The test runner provided here encapsulate all of this, however, so DigitalOcean CSI driver authors only need to care about updating or adding testdriver files in the `tests` sub-directory. Each file is specific to a particular Kubernetes release and must be named `<major version>.<minor version>.yaml`, e.g., `1.16.yaml`.

Frequently, representing newly supported features in a testdriver file means flipping on additional capabilities [which are documented in-code](https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/framework/testdriver.go).

### Go files

`e2e_test.go` and other Go files in the same directory implement a CLI tool on top of Go's testing package that supports spinning up a DOKS cluster on-the-fly (leveraging [our deploy script](/test/kubernetes/deploy)), installing a custom CSI driver image, and executing the end-to-end tests.

It accepts a number of command-line arguments to control and customize the test procedure. Note that the boolean flag `-long` is required to run the tests at all -- this is to allow building/linting/vetting the Go files without actually running the tests.

### e2e.sh

`e2e.sh` is a small shell wrapper that builds the Go files and executes the resulting binary. It is meant to be the single interface for invoking the end-to-end tests.

It accepts an optional `TIMEOUT` environment variable to configure the test timeout in [Go time parseable duration](https://golang.org/pkg/time/#ParseDuration). The default value is 30 minutes (`30m`). This may need to be increased if tests are run against mutliple Kubernetes versions with clusters provisioned dynamically.

Command-line arguments are passed as-in to the test tool. Run `e2e.sh -h` for usage instructions and examples.

## How to

### Run the end-to-end tests

1. If necessary, update the test runner image (see below for instructions)
1. If necessary, add or update the testdriver file definitions (see the [related section above](#testdrivers) for details)
1. Execute `e2e.sh`, passing in parameters as needed

### Update an existing test runner image

1. Make and push any necessary updates to our Kubernetes fork
1. If the fork was updated: Update the `SHA_*` build argument(s) in the runner image Dockerfile to point to the new commit hash(es)
1. Add the build argument

### Add support for a new Kubernetes release

1. Add a new Kubernetes version-specific block to the [runner image Dockerfile](https://github.com/digitalocean/csi-digitalocean/blob/master/test/e2e/Dockerfile); make sure to update the `SHA_*` commit hash and/or `*_SHA256_*` e2e.test binary checksum variables as well. (You can use `scripts/get-e2etest-sha256.sh` to generate the e2e.test binary checksum for a given Kubernetes version.)
   1. Like so: [X is your minor version and Y is the patch version]()
      ```Dockerfile
      ### Kubernetes 1.XX
      FROM builder AS tests-X.Y
      ARG KUBE_VERSION_1_X=1.X.Y
      ARG KUBE_VERSION_1_X_E2E_BIN_SHA256_CHECKSUM=<generated-sha-hash>

      RUN curl --fail --location https://dl.k8s.io/v${KUBE_VERSION_1_X}/kubernetes-test-linux-amd64.tar.gz | tar xvzf - --strip-components 3 kubernetes/test/bin/e2e.test kubernetes/test/bin/ginkgo
      RUN echo "${KUBE_VERSION_1_X_E2E_BIN_SHA256_CHECKSUM}" e2e.test | sha256sum --check
      RUN cp e2e.test /e2e.1.X.test
      RUN cp ginkgo /ginkgo-1.X
      ```
   2. Ensure that a version specific ginkgo binary is copied into the final runner image
2. Update the Makefile `runner-build` and `runner-push` targets.
3. Extend the Kubernetes release-specific build arguments in the [`handle-images.sh`](https://github.com/digitalocean/csi-digitalocean/blob/master/test/e2e/handle-image.sh) script.
4. Add a new testdriver YAML configuration file.
5. Extend the list of supported Kubernetes releases in `e2e_test.go`.
   1. [Update the `e2e_test.go` file `supportedKubernetesVersions` variable with the up-to-date versions](https://github.com/digitalocean/csi-digitalocean/blob/master/test/e2e/e2e_test.go)
6. Extend the list of tested Kubernetes releases in `.github/workflows/test.yaml`.
7. Extend the list of deleted container images in `.github/workflows/delete.yaml`.
8. Update the [_Kubernetes Compatibility_ matrix](../../README.md#kubernetes-compatibility) in the README file.
9. If needed, [remove deprecated version in Dockerfile](https://github.com/digitalocean/csi-digitalocean/blob/master/test/e2e/Dockerfile)
10. [Upgrade DOCTL_VERSION environment variable with latest version](https://github.com/digitalocean/csi-digitalocean/blob/master/test/e2e/Dockerfile)
    1. Latest, version can be found [here](https://github.com/digitalocean/doctl/releases)
11. [Add copy commands under the `final test container section` to the Dockerfile where XX is the minor version](https://github.com/digitalocean/csi-digitalocean/blob/master/test/e2e/Dockerfile)
       ```Dockerfile
       COPY --from=tests-1.XX /e2e.1.XX.test /
       COPY --from=tests-1.XX /ginkgo-1.XX /usr/local/bin
       ```

### handle-image.sh

The `handle-image.sh` script can be used to either `build` or `push` a runner image. It should only be needed for testing images hosted on a personal Docker Hub account because the CI manages the canonical images under the `digitalocean` organization.

You may also overwrite a commit hash on-the-fly by defining the corresponding environment variable(s) `SHA_*`, e.g.: `SHA_1_16=deadbeef ./handle-image.sh build`.

The image to be built may be overwritten by defining the `IMAGE` variable, e.g.: `IMAGE=timoreimann/k8s-e2e-test-runner ./handle-image.sh build`.
