package framework

import (
	"context"
	"flag"
	"fmt"
	"github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
)

var (
	f              Framework
	TestDriverPath string
)

type Framework struct {
	BaseName   string
	Namespace  *v1.Namespace
	ClientSet  clientset.Interface
	DriverData *DriverDefinition
}

func init() {
	flag.StringVar(&TestDriverPath, "storage.testdriver", "", "The testdriver yaml file")
}

func NewDefaultFramework(baseName string) *Framework {
	return NewFramework(baseName, nil)
}

func NewFramework(baseName string, client clientset.Interface) *Framework {
	f := &Framework{
		BaseName:  baseName,
		ClientSet: client,
	}

	ginkgo.BeforeEach(f.BeforeEach)

	return f
}

func (f *Framework) BeforeEach(ctx context.Context) {
	ginkgo.DeferCleanup(f.AfterEach)

	driver, err := readTestDriverFile()
	if err != nil {
		ginkgo.Fail(fmt.Sprintf("Not able to read testdriver yaml file, %s", err.Error()))
	}

	f.DriverData = driver

	var kubeconfig *string

	// Try to get kube config either from user home directory or absolute path.
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	// Override with KUBECONFIG env variable.
	kubeconfigEnv := os.Getenv("KUBECONFIG")
	if kubeconfigEnv != "" {
		*kubeconfig = kubeconfigEnv
	}

	ginkgo.By(fmt.Sprintf("Creating a kubernetes client, basename %s", f.BaseName))

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		ginkgo.Fail(err.Error())
	}

	f.ClientSet, err = clientset.NewForConfig(config)
	if err != nil {
		ginkgo.Fail(err.Error())
	}

	ns, err := f.CreateNamespace(ctx)

	f.Namespace = ns
}

func (f *Framework) AfterEach(ctx context.Context) {
	defer func() {
		ginkgo.By(fmt.Sprintf("Destroying namespace: %s", f.Namespace.Name))

		if err := f.ClientSet.CoreV1().Namespaces().Delete(ctx, f.Namespace.Name, metav1.DeleteOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				ginkgo.By(fmt.Sprintf("Could not delete namespace: %s", f.Namespace.Name))
			} else {
				ginkgo.By(fmt.Sprintf("Namespace was already deleted: %s", f.Namespace.Name))
			}
		}

		f.Namespace = nil
		f.ClientSet = nil
	}()
}

func (f *Framework) CreateNamespace(ctx context.Context) (*v1.Namespace, error) {
	nsSpec := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: f.BaseName,
		},
		Status: v1.NamespaceStatus{},
	}

	ginkgo.By(fmt.Sprintf("Create namespace: %s", f.BaseName))

	ns, err := f.ClientSet.CoreV1().Namespaces().Create(ctx, nsSpec, metav1.CreateOptions{})

	if err != nil {
		ginkgo.By(fmt.Sprintf("Failed to create namespace: %v", err))

		return nil, err
	}

	return ns, nil
}

func readTestDriverFile() (*DriverDefinition, error) {
	data, err := os.ReadFile(TestDriverPath)
	if err != nil {
		return nil, err
	}

	driverData := &DriverDefinition{}

	if err := runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), data, driverData); err != nil {
		return nil, err
	}

	if driverData.DriverInfo.Name == "" {
		return nil, fmt.Errorf("DriverInfo.Name is not set in file: %s", TestDriverPath)
	}

	return driverData, nil
}

type DriverDefinition struct {
	DriverInfo struct {
		Name               string
		SupportedSizeRange struct {
			Max string
			Min string
		}
	}
}

func (d *DriverDefinition) GetObjectKind() schema.ObjectKind {
	return nil
}

func (d *DriverDefinition) DeepCopyObject() runtime.Object {
	return nil
}
