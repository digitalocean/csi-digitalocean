package e2e

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	testRunnerImage                 = "digitalocean/k8s-e2e-test-runner:latest"
	envVarDigitalOceanAccessToken   = "DIGITALOCEAN_ACCESS_TOKEN"
	envVarSkipTestsParallel         = "SKIP_PARALLEL_TESTS"
	envVarSkipTestsSequential       = "SKIP_SEQUENTIAL_TESTS"
	testdriverDirectoryRelativePath = "testdrivers"
	deployScriptName                = "deploy.sh"
	e2eContainerName                = "do-k8s-e2e"
)

var (
	tokenMissingErr = errors.New("token must be specified in DIGITALOCEAN_ACCESS_TOKEN environment variable")

	// De-facto global variables that require initialization at runtime.
	supportedKubernetesVersions     = []string{"1.16", "1.15", "1.14"}
	sourceFileDir                   string
	testdriverDirectoryAbsolutePath string
	deployScriptPath                string

	// Variables initialized in TestMain that are leveraged by the tests.
	ctx context.Context
	p   params
)

type params struct {
	long           bool
	driverImage    string
	runnerImage    string
	focus          string
	kubeconfig     string
	nameSuffix     string
	retainClusters bool
	kubeVersions   []string
	skipParallel   bool
	skipSequential bool
}

func init() {
	_, filePath, _, _ := runtime.Caller(0)
	sourceFileDir = filepath.Dir(filePath)
	testdriverDirectoryAbsolutePath = filepath.Join(sourceFileDir, testdriverDirectoryRelativePath)
	deployScriptPath = filepath.Join(sourceFileDir, "..", "kubernetes", "deploy", deployScriptName)

	flag.Usage = func() {
		fmt.Println(`e2e.test runs containerized, external storage end-to-end tests from upstream Kubernetes against a CSI driver.

It supports dynamically creating (and post-test deleting) a DOKS cluster to run a driver-under-test in. The environment
variable DIGITALOCEAN_ACCESS_TOKEN must be set to a DigitalOcean API key for this purpose.
The cluster will be tagged with "csi-e2e-test" and "version:<sanitized Kubernetes version>" where
<sanitized Kubernetes version> is the Kubernetes major-minor version replacing dots by dashes, e.g., "version:1-16"
(DigitalOcean tags must not contain dots).
The name of a cluster will be "csi-e2e-<sanitized Kubernetes version>-test-<suffix>" where <suffix> is a random
alphanumeric suffix if not customized through the corresponding command-line flag.

One or more Kubernetes major-minor versions to run tests for may be given. If omitted, tests will be run for all supported
Kubernetes versions.

External storage end-to-end tests require a Kubernetes version-specific testdriver YAML file to be defined. An error is
returned if no corresponding file is found for a given Kubernetes release.

Examples:

# Run tests for all supported versions:
e2e.test

# Run tests for 1.16 only:
e2e.test 1.16

# Run tests for 1.16 and 1.14 (but not 1.15):
e2e.test 1.16 1.14

# Create cluster with a specific suffix:
e2e.test --name-suffix=$(git rev-parse --short HEAD)

# Retain cluster after erroneous completion of the tests:
e2e.test --retain

# Use cluster referenced by kubeconfig file instead of using a dynamic cluster:
e2e.test --kubeconfig=$HOME/.kube/config

# Use a custom driver image:
e2e.test --driver-image=timoreimann/do-csi-plugin:dev

# Use a custom end-to-end test runner image:
e2e.test --runner-image=timoreimann/k8s-e2e-test-runner:latest

# Skip the parallel tests
e2e.test -skip-parallel

# Skip the sequential tests
e2e.test -skip-sequential

Options:`)
		flag.PrintDefaults()
	}
}

func TestMain(m *testing.M) {
	flag.BoolVar(&p.long, "long", false, "Run long tests")
	flag.StringVar(&p.driverImage, "driver-image", "", "The driver container image to use. Triggers a deployment of the \"latest\"-suffixed development manifest into the cluster if given. Otherwise, the built-in driver of the cluster is used.")
	flag.StringVar(&p.runnerImage, "runner-image", testRunnerImage, "The end-to-end runner image to use.")
	flag.StringVar(&p.focus, "focus", "", "A custom ginkgo focus to use for external storage tests. Defaults to running all external tests.")
	flag.StringVar(&p.kubeconfig, "kubeconfig", "", "The kubeconfig file to use. For DOKS clusters where the kubeconfig has been retrieved via doctl, the DIGITALOCEAN_ACCESS_TOKEN environment variable must be set. If not specified, add-hoc DOKS clusters will be created and cleaned up afterwards for each tested Kubernetes version (unless the test failed and -retain is specified).")
	flag.StringVar(&p.nameSuffix, "name-suffix", "", "A suffix to append to the cluster name. If not specified, a random suffix will be chosen. Ignored if -kubeconfig is specified.")
	flag.BoolVar(&p.retainClusters, "retain", false, "Retain the created cluster(s) on failure. (Clusters are always cleaned up on success.) Ignored if -kubeconfig is specified.")
	flag.BoolVar(&p.skipParallel, "skip-parallel", false, "Skip parallel tests")
	flag.BoolVar(&p.skipSequential, "skip-sequential", false, "Skip sequential tests")
	flag.Parse()

	p.kubeVersions = flag.Args()

	if p.nameSuffix == "" {
		p.nameSuffix = rand.String(5)
	}

	var cancel func()
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancel()
	}()

	os.Exit(m.Run())
}

func TestE2E(t *testing.T) {
	if !p.long {
		t.Skip("Skipping test because long mode is not set")
	}

	token := os.Getenv(envVarDigitalOceanAccessToken)

	if len(p.kubeVersions) == 0 {
		p.kubeVersions = supportedKubernetesVersions
	}

	for _, kubeVer := range p.kubeVersions {
		t.Run(kubeVer, func(t *testing.T) {
			var found bool
			for _, supportedKubeVer := range supportedKubernetesVersions {
				if supportedKubeVer == kubeVer {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("unsupported Kubernetes version: %s", kubeVer)
			}

			testdriverFilename := filepath.Join(testdriverDirectoryAbsolutePath, fmt.Sprintf("%s.yaml", kubeVer))
			if _, err := os.Stat(testdriverFilename); os.IsNotExist(err) {
				t.Fatalf("testdriver file %q does not exist in %q", testdriverFilename, testdriverDirectoryAbsolutePath)
			}

			kubeconfig := p.kubeconfig
			if kubeconfig == "" {
				client, err := createDOClient(ctx, token)
				if err != nil {
					t.Fatalf("failed to create DigitalOcean API client: %s", err)
				}

				kubeconfigData, cleanup, err := createCluster(ctx, client, p.nameSuffix, kubeVer)
				// Ignore error in order to clean up any partial cluster setups
				// as long as we received a cleanup function and do not intend
				// to retain the cluster.
				if cleanup != nil {
					defer func() {
						// Do not clean up if run succeeded, or if it failed
						// (including cancelations) but we do not want to retain
						// the cluster.
						ctxCanceled := ctx.Err() != nil
						if (!ctxCanceled && !t.Failed()) || !p.retainClusters {
							return
						}
						cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
						defer cleanupCancel()
						if err := cleanup(cleanupCtx); err != nil {
							t.Errorf("failed to clean up cluster: %s", err)
						}
					}()
				}
				if err != nil {
					t.Fatalf("failed to create cluster for Kubernetes version %s: %s", kubeVer, err)
				}

				// Create temporary directory where the test lives. The operating
				// system-specific temporary folder would not be bind-mountable into
				// our e2e container by default on Mac.
				tmpfile, err := ioutil.TempFile(sourceFileDir, "csi-e2e-kubeconfig-*")
				if err != nil {
					t.Fatalf("failed to create temporary file: %s", err)
				}
				defer os.Remove(tmpfile.Name())

				if _, err := tmpfile.Write(kubeconfigData.KubeconfigYAML); err != nil {
					t.Fatalf("failed to write kubeconfig data to temporary file %s: %s", tmpfile.Name(), err)
				}
				if err := tmpfile.Close(); err != nil {
					t.Fatalf("failed to close temporary file %s: %s", tmpfile.Name(), err)
				}
				kubeconfig = tmpfile.Name()
			}

			if p.driverImage != "" {
				err := deployDriver(ctx, p.driverImage, kubeconfig, token)
				if err != nil {
					t.Fatalf("failed to deploy driver image %s: %s", p.driverImage, err)
				}
			}

			err := runE2ETests(ctx, kubeVer, p.runnerImage, testdriverFilename, p.focus, kubeconfig, token, p.skipParallel, p.skipSequential)
			if err != nil {
				t.Fatalf("end-to-end tests failed: %s", err)
			}
		})
	}
}

func createDOClient(ctx context.Context, token string) (client *godo.Client, err error) {
	if token == "" {
		return nil, tokenMissingErr
	}

	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: token,
	})
	oauthClient := oauth2.NewClient(ctx, tokenSource)

	opts := []godo.ClientOpt{
		godo.SetUserAgent("csi-digitalocean/e2e-tests"),
	}

	doClient, err := godo.New(oauthClient, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create godo client: %s", err)
	}

	return doClient, nil
}

func createCluster(ctx context.Context, client *godo.Client, nameSuffix, kubeVersion string) (*godo.KubernetesClusterConfig, func(ctx context.Context) error, error) {
	kubeVerSanitized := strings.ReplaceAll(kubeVersion, ".", "-")
	clusterName := fmt.Sprintf("csi-e2e-%s-test-%s", kubeVerSanitized, nameSuffix)
	versionTag := fmt.Sprintf("version:%s", kubeVerSanitized)

	// Find and delete any existing cluster that goes by the same name.
	page := 1
	for {
		clusters, resp, err := client.Kubernetes.List(ctx, &godo.ListOptions{
			Page:    page,
			PerPage: 50,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list clusters: %s", err)
		}

		for _, cluster := range clusters {
			for _, tag := range cluster.Tags {
				if tag == versionTag && cluster.Name == clusterName {
					if err := deleteCluster(ctx, client, cluster.ID); err != nil {
						return nil, nil, fmt.Errorf("failed to delete previous cluster %s (%s): %s", cluster.ID, cluster.Name, err)
					}
				}
			}
		}

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		p, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get current page: %s", err)
		}
		page = p + 1
	}

	cluster, resp, err := client.Kubernetes.Create(ctx, &godo.KubernetesClusterCreateRequest{
		Name:        clusterName,
		RegionSlug:  "fra1",
		VersionSlug: kubeVersion,
		Tags:        []string{"csi-e2e-test", versionTag},
		NodePools: []*godo.KubernetesNodePoolCreateRequest{
			&godo.KubernetesNodePoolCreateRequest{
				Name:  clusterName + "-pool",
				Size:  "s-4vcpu-8gb",
				Count: 3,
			},
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cluster: %s", err)
	}
	fmt.Printf("Created cluster %s (%s) (response code: %d)\n", cluster.ID, cluster.Name, resp.StatusCode)

	cleanup := func(ctx context.Context) error {
		if err := deleteCluster(ctx, client, cluster.ID); err != nil {
			return fmt.Errorf("failed to delete used cluster %s (%s): %s", cluster.ID, cluster.Name, err)
		}
		fmt.Printf("Cleaned up cluster %s (%s)\n", cluster.ID, cluster.Name)

		return nil
	}

	pollCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	fmt.Printf("Waiting for cluster %s (%s) to become running\n", cluster.ID, cluster.Name)
	err = wait.PollUntil(10*time.Second, func() (done bool, waitErr error) {
		c, _, err := client.Kubernetes.Get(pollCtx, cluster.ID)
		if err != nil {
			return false, err
		}

		if c.Status.State == godo.KubernetesClusterStatusRunning {
			return true, nil
		}

		fmt.Printf("Current status of cluster %s (%s): %s (message: %s)\n", cluster.ID, cluster.Name, c.Status.State, c.Status.Message)
		cluster = c
		return false, nil
	}, ctx.Done())
	if err != nil {
		return nil, cleanup, fmt.Errorf("cluster %s (%s) never became running -- last status: %s (message: %s): %s", cluster.ID, cluster.Name, cluster.Status.State, cluster.Status.Message, err)
	}
	fmt.Printf("Cluster %s (%s) is running\n", cluster.ID, cluster.Name)

	kubeConfig, _, err := client.Kubernetes.GetKubeConfig(ctx, cluster.ID)
	if err != nil {
		return nil, cleanup, fmt.Errorf("failed to get kubeconfig for cluster %s (%s): %s", cluster.ID, cluster.Name, err)
	}

	return kubeConfig, cleanup, nil
}

func deleteCluster(ctx context.Context, client *godo.Client, clusterID string) error {
	resp, err := client.Kubernetes.Delete(ctx, clusterID)
	if err != nil {
		if resp == nil || resp.StatusCode != http.StatusNotFound {
			return err
		}
	}

	return nil
}

// deployDriver invokes our deploy script with the right set of parameters.
func deployDriver(ctx context.Context, driverImage string, kubeconfigFile, token string) error {
	if token == "" {
		return tokenMissingErr
	}

	return runCommand(ctx, deployScriptPath, cmdParams{
		args: []string{"-y"},
		envs: []string{
			fmt.Sprintf("%s=%s", envVarDigitalOceanAccessToken, token),
			fmt.Sprintf("KUBECONFIG=%s", kubeconfigFile),
			fmt.Sprintf("DEV_IMAGE=%s", driverImage),
		},
		dir: filepath.Dir(deployScriptPath),
	})
}

// runE2ETests invokes our test container.
// It passes in bind-mount parameters for the kubeconfig and the location of the
// testdriver YAML files.
func runE2ETests(ctx context.Context, kubeVersion, runnerImage, testdriverFilename, focus, kubeconfigFile, token string, skipParallel, skipSequential bool) (retErr error) {
	testdriverDirectoryInContainer := "/testdrivers"
	testdriverFilenameInContainer := filepath.Join(testdriverDirectoryInContainer, filepath.Base(testdriverFilename))

	envs := []string{
		"KUBECONFIG=/root/.kube/config",
	}

	if skipParallel {
		envs = append(envs, fmt.Sprintf("%s=1", envVarSkipTestsParallel))
	}
	if skipSequential {
		envs = append(envs, fmt.Sprintf("%s=1", envVarSkipTestsSequential))
	}

	if token != "" {
		envs = append(envs, fmt.Sprintf("%s=%s", envVarDigitalOceanAccessToken, token))
	}

	p := containerParams{
		image: canonicalizeImage(runnerImage),
		cmd: []string{
			kubeVersion,
			testdriverFilenameInContainer,
		},
		env: envs,
		binds: map[string]string{
			kubeconfigFile:                  "/root/.kube/config",
			testdriverDirectoryAbsolutePath: testdriverDirectoryInContainer,
		},
		// ginkgo initiates graceful termination and cleanup of namespaces on
		// SIGINT.
		stopSignal:  "INT",
		stopTimeout: 1 * time.Minute,
	}

	return runContainer(ctx, p)
}

func canonicalizeImage(image string) string {
	if strings.Count(image, "/") < 2 {
		image = dockerHost + image
	}
	return image
}
