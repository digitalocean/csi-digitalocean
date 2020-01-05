# end-to-end testing

It is possible to run the SIG Storage-managed end-to-end tests provided by upstream Kubernetes to validate the correctness of the DigitalOcean CSI driver. This document describes how the end-to-end are structured and how they can be performed using the files in this directory.

## Introduction

The Kubernetes project maintains a set of end-to-end tests that are used to validate the correctness of the upstream development. Since Kubernetes 1.14, these tests can also be consumed externally by provisioning a Kubernetes cluster to run the tests against.

This also holds for the storage-related tests but comes with a caveat: the CSI specification gives a lot of flexibility as to which features may be implemented by a drivers, with the implication that not all drivers may implement all functionality that is being verified by the end-to-end tests. Fortunately, the external storage tests [provide a mechanism](https://github.com/kubernetes/kubernetes/tree/master/test/e2e/storage/external) to declare the set of features implemented by a driver-under-test, and the testing framework takes this information into account to selectively choose tests during runtime.

Our end-to-end testing integration customizes the test configuration for DigitalOcean's CSI driver and provides tooling to easily perform the testing.

## Components

The end-to-end test integration consists of the following components:

- a Dockerfile bundling Kubernetes version-specific dependencies and tools to drive test execution through a test runner image
- a set of Go files preparing the right test environment and triggering the test execution
- a shell wrapper start script

Details for each component follow below.

### Dockerfile

The Dockerfile bundles a fork of the Kubernetes end-to-end test binary for each supported Kubernetes version along with some tooling. The resulting test runner image can be invoked to perform the end-to-end tests.

The fork is needed for all Kubernetes versions that do not ship with [this PR](https://github.com/kubernetes/kubernetes/pull/86000) since the tests otherwise fail. (See the PR description for details.) They are maintained on the <https://github.com/digitalocean/kubernetes> repository in the `do-doks-<Kubernetes major-minor version>` branches, e.g., `do-doks-1.16`. Each branch is based on the upstream branch `origin/release-<Kubernetes major-minor version>`; see the git history in our repository to check what the latest upstream HEAD for each forked branch is. In the Dockerfile, we point to the exact commit hash for each branch to enable deterministic builds.
Forks will be phased out as we are able to use Kubernetes releases that ship with our patch.

A test binary is needed for each Kubernetes version since upstream does not offer backwards compatibility for older versions; that is, the test binary for a given version is guaranteed to work with that version only.

We also ship ginkgo and a few other tools needed to run the tests. See the Dockerfile comments for details.

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
2. Execute `e2e.sh`, passing in parameters as needed

### Update the test runner image

1. Make and push any necessary updates to our Kubernetes fork
2. If the fork was updated: Update the `SHA_*` build argument(s) in the Dockerfile to point to the new commit hash(es)
3. Run `handle-image.sh build`
4. Run `handle-image.sh push`

For testing purposes, you may also overwrite a commit hash on-the-fly by defining the corresponding environment variable(s) `SHA_*`, e.g.: `SHA_1_16=deadbeef handle-image build`.

The image to be built may be overwritten by defining the `IMAGE` variable, e.g.: `IMAGE=timoreimann/k8s-e2e-test-runner ./handle-image.sh build`.
