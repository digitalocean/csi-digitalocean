// +build integration

package integration

import (
	"errors"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// namespace defines the namespace the resources will be created for the CSI tests
	namespace = "csi-test"
)

var (
	client kubernetes.Interface
)

func TestMain(m *testing.M) {
	fmt.Println("==> Setting up tests")
	if err := setup(); err != nil {
		log.Fatalln(err)
	}

	// run the tests, don't call any defer yet as it'll fail due `os.Exit()
	exitStatus := m.Run()

	fmt.Println("==> Tearing down tests")
	if err := teardown(); err != nil {
		// don't call log.Fatalln() as we exit with `m.Run()`'s exit status
		log.Println(err)
	}

	os.Exit(exitStatus)
}

func TestPod_Single_Volume(t *testing.T) {
	volumeName := "my-do-volume"
	claimName := "csi-pod-pvc"

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-csi-app",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "my-busybox",
					Image: "busybox",
					VolumeMounts: []v1.VolumeMount{
						{
							MountPath: "/data",
							Name:      volumeName,
						},
					},
					Command: []string{
						"sleep",
						"1000000",
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: volumeName,
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: claimName,
						},
					},
				},
			},
		},
	}

	fmt.Println("Creating pod")
	_, err := client.CoreV1().Pods(namespace).Create(pod)
	if err != nil {
		t.Fatal(err)
	}

	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: claimName,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadWriteOnce,
			},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse("5Gi"),
				},
			},
			StorageClassName: strPtr("do-block-storage"),
		},
	}

	fmt.Println("Creating pvc")
	_, err = client.CoreV1().PersistentVolumeClaims(namespace).Create(pvc)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("Waiting pod to be running ...")
	if err := waitForPod(client, pod.Name); err != nil {
		t.Error(err)
	}

	fmt.Println("Finished!")
}

func setup() error {
	// if you want to change the loading rules (which files in which order),
	// you can do so here
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	// if you want to change override values or bind them to flags, there are
	// methods to help you
	configOverrides := &clientcmd.ConfigOverrides{}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return err
	}

	// create the clientset
	client, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	// create test namespace
	_, err = client.CoreV1().Namespaces().Create(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	})

	if err != nil {
		return err
	}

	return nil
}

func teardown() error {
	// delete all test resources
	err := client.CoreV1().Namespaces().Delete(namespace, nil)
	if err != nil && !(kubeerrors.IsNotFound(err) || kubeerrors.IsAlreadyExists(err)) {
		return err
	}

	return nil
}

func strPtr(s string) *string {
	return &s
}

// waitForPod waits for the given pod name to be running
func waitForPod(client kubernetes.Interface, name string) error {
	var err error
	stopCh := make(chan struct{})

	go func() {
		select {
		case <-time.After(time.Minute * 5):
			err = errors.New("timing out waiting for pod state")
			close(stopCh)
		case <-stopCh:
		}
	}()

	watchlist := cache.NewListWatchFromClient(client.CoreV1().RESTClient(),
		"pods", namespace, fields.Everything())
	_, controller := cache.NewInformer(watchlist, &v1.Pod{}, time.Second*1,
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(o, n interface{}) {
				pod := n.(*v1.Pod)
				if name != pod.Name {
					return
				}

				if pod.Status.Phase == v1.PodFailed || pod.Status.Phase == v1.PodSucceeded {
					err = errors.New("pod status is Failed or in Succeeded status (terminated)")
					close(stopCh)
					return
				}

				if pod.Status.Phase == v1.PodRunning {
					close(stopCh)
					return
				}
			},
		})

	controller.Run(stopCh)
	return err
}
