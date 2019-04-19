// +build integration

package integration

import (
	"errors"
	"log"
	"os"
	"testing"
	"time"

	"github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1alpha1"
	snapclientset "github.com/kubernetes-csi/external-snapshotter/pkg/client/clientset/versioned"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// namespace defines the namespace the resources will be created for the CSI tests
	namespace = "csi-test"
)

var (
	client     kubernetes.Interface
	snapClient snapclientset.Interface
)

func TestMain(m *testing.M) {
	if err := setup(); err != nil {
		log.Fatalln(err)
	}

	// run the tests, don't call any defer yet as it'll fail due `os.Exit()
	exitStatus := m.Run()

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
					Name:  "my-csi-app",
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

	t.Log("Creating pod")
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

	t.Log("Creating pvc")
	_, err = client.CoreV1().PersistentVolumeClaims(namespace).Create(pvc)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting pod %q to be running ...\n", pod.Name)
	if err := waitForPod(client, pod.Name); err != nil {
		t.Error(err)
	}

	t.Log("Finished!")
}

func TestDeployment_Single_Volume(t *testing.T) {
	volumeName := "my-do-volume"
	claimName := "csi-deployment-pvc"
	appName := "my-csi-app"

	replicaCount := new(int32)
	*replicaCount = 1

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: appName,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: replicaCount,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": appName,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": appName,
					},
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
			},
		},
	}

	t.Log("Creating deployment")
	_, err := client.AppsV1().Deployments(namespace).Create(dep)
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

	t.Log("Creating pvc")
	_, err = client.CoreV1().PersistentVolumeClaims(namespace).Create(pvc)
	if err != nil {
		t.Fatal(err)
	}

	// get pod associated with the deployment
	selector, err := appSelector(appName)
	if err != nil {
		t.Fatal(err)
	}

	pods, err := client.CoreV1().Pods(namespace).
		List(metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		t.Fatal(err)
	}

	if len(pods.Items) != 1 || len(pods.Items) > 1 {
		t.Fatalf("expected to have a 1 pod, got %d pods for the given deployment", len(pods.Items))

	}
	pod := pods.Items[0]

	t.Logf("Waiting pod %q to be running ...\n", pod.Name)
	if err := waitForPod(client, pod.Name); err != nil {
		t.Error(err)
	}

	t.Log("Finished!")
}

func TestPod_Multi_Volume(t *testing.T) {
	volumeName1 := "my-do-volume-1"
	volumeName2 := "my-do-volume-2"
	claimName1 := "csi-pod-pvc-1"
	claimName2 := "csi-pod-pvc-2"
	appName := "my-multi-csi-app"

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: appName,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  appName,
					Image: "busybox",
					VolumeMounts: []v1.VolumeMount{
						{
							MountPath: "/data/pod-1/",
							Name:      volumeName1,
						},
						{
							MountPath: "/data/pod-2/",
							Name:      volumeName2,
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
					Name: volumeName1,
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: claimName1,
						},
					},
				},
				{
					Name: volumeName2,
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: claimName2,
						},
					},
				},
			},
		},
	}

	t.Log("Creating pod")
	_, err := client.CoreV1().Pods(namespace).Create(pod)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Creating pvc1")
	pvc1 := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: claimName1,
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
	_, err = client.CoreV1().PersistentVolumeClaims(namespace).Create(pvc1)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Creating pvc2")
	pvc2 := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: claimName2,
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
	_, err = client.CoreV1().PersistentVolumeClaims(namespace).Create(pvc2)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting pod %q to be running ...\n", pod.Name)
	if err := waitForPod(client, pod.Name); err != nil {
		t.Error(err)
	}

	t.Log("Finished!")
}

func TestSnapshot_Create(t *testing.T) {
	volumeName := "my-do-volume"
	pvcName := "csi-do-test-pvc"

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-csi-app-2",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "my-csi-app",
					Image: "busybox",
					VolumeMounts: []v1.VolumeMount{
						{
							MountPath: "/data",
							Name:      volumeName,
						},
					},
					Command: []string{
						"sh", "-c",
						"echo testcanary > /data/canary && sleep 1000000",
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: volumeName,
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}

	t.Log("Creating pod")
	_, err := client.CoreV1().Pods(namespace).Create(pod)
	if err != nil {
		t.Fatal(err)
	}

	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvcName,
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

	t.Log("Creating pvc")
	_, err = client.CoreV1().PersistentVolumeClaims(namespace).Create(pvc)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for pod %q to be running ...\n", pod.Name)
	if err := waitForPod(client, pod.Name); err != nil {
		t.Error(err)
	}

	snapshotName := "csi-do-test-snapshot"
	snapshot := &v1alpha1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name: snapshotName,
		},
		Spec: v1alpha1.VolumeSnapshotSpec{
			Source: &v1alpha1.TypedLocalObjectReference{
				Name: pvcName,
				Kind: "PersistentVolumeClaim",
			},
		},
	}

	t.Log("Creating snapshots")
	_, err = snapClient.VolumesnapshotV1alpha1().VolumeSnapshots(namespace).Create(snapshot)
	if err != nil {
		t.Fatal(err)
	}

	restorePVCName := "csi-do-test-pvc-restore"
	apiGroup := "snapshot.storage.k8s.io"

	restorePVC := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: restorePVCName,
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
			DataSource: &v1.TypedLocalObjectReference{
				APIGroup: &apiGroup,
				Kind:     "VolumeSnapshot",
				Name:     snapshotName,
			},
		},
	}

	t.Log("Restoring from snapshot using a new PVC")
	_, err = client.CoreV1().PersistentVolumeClaims(namespace).Create(restorePVC)
	if err != nil {
		t.Fatal(err)
	}

	restoredPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-csi-app-2-restored",
		},
		Spec: v1.PodSpec{
			// This init container verifies that the /data/canary file is present.
			// If it is not, then the volume was not properly restored.
			// waitForPod only waits for the pod to enter the running state, so will not
			// detect any failures after that, so this has to be an InitContainer so that
			// the pod never enters the running state if it fails.
			InitContainers: []v1.Container{
				{
					Name:  "my-csi",
					Image: "busybox",
					VolumeMounts: []v1.VolumeMount{
						{
							MountPath: "/data",
							Name:      volumeName,
						},
					},
					Command: []string{
						"cat",
						"/data/canary",
					},
				},
			},
			Containers: []v1.Container{
				{
					Name:  "my-csi-app",
					Image: "busybox",
					VolumeMounts: []v1.VolumeMount{
						{
							MountPath: "/data",
							Name:      volumeName,
						},
					},
					Command: []string{
						"sleep", "1000000",
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: volumeName,
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: restorePVCName,
						},
					},
				},
			},
		},
	}

	t.Log("Creating a new pod with the resotored snapshot")
	_, err = client.CoreV1().Pods(namespace).Create(restoredPod)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting pod %q to be running ...\n", restoredPod.Name)
	if err := waitForPod(client, restoredPod.Name); err != nil {
		t.Error(err)
	}

	t.Log("Finished!")
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

	snapClient, err = snapclientset.NewForConfig(config)
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

// appSelector returns a selector that selects deployed applications with the
// given name
func appSelector(appName string) (labels.Selector, error) {
	selector := labels.NewSelector()
	appRequirement, err := labels.NewRequirement("app", selection.Equals, []string{appName})
	if err != nil {
		return nil, err
	}

	selector = selector.Add(
		*appRequirement,
	)

	return selector, nil
}
