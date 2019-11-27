// +build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1alpha1"
	snapclientset "github.com/kubernetes-csi/external-snapshotter/pkg/client/clientset/versioned"
	"golang.org/x/oauth2"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/wait"
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
	doClient   *godo.Client
	// testStorageClass defines the storage class to test. By default it's our
	// default storage class name, do-block-storage, but can be set to a
	// different value via the TEST_STORAGE_CLASS environment variable.
	testStorageClass = "do-block-storage"
	// skipCleanup can be set to true via the SKIP_CLEANUP environment variable
	// to have all resources left behind on test failure. This is useful for
	// investigating failures.
	skipCleanup = false

	tokenEnvVars = []string{
		"CSI_DIGITALOCEAN_ACCESS_TOKEN",
		"DIGITALOCEAN_ACCESS_TOKEN",
	}
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
			StorageClassName: strPtr(testStorageClass),
		},
	}

	t.Log("Creating pvc")
	_, err = client.CoreV1().PersistentVolumeClaims(namespace).Create(pvc)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting pod %q to be running ...\n", pod.Name)
	if _, err := waitForPod(client, pod.Name); err != nil {
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
			StorageClassName: strPtr(testStorageClass),
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
	if _, err := waitForPod(client, pod.Name); err != nil {
		t.Error(err)
	}

	t.Log("Finished!")
}

func TestPersistentVolume_Resize(t *testing.T) {
	volumeName := "my-do-volume"
	claimName := "csi-pvc-resizer"

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-csi-app-resizer",
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
			StorageClassName: strPtr(testStorageClass),
		},
	}

	t.Log("Creating pvc")
	_, err = client.CoreV1().PersistentVolumeClaims(namespace).Create(pvc)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for pod %q to be running ...", pod.Name)
	if _, err := waitForPod(client, pod.Name); err != nil {
		t.Error(err)
	}

	createdPVC, err := client.CoreV1().PersistentVolumeClaims(namespace).Get(claimName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	pvName := createdPVC.Spec.VolumeName
	pv, err := client.CoreV1().PersistentVolumes().Get(pvName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if pv.Spec.Capacity["storage"] != resource.MustParse("5Gi") {
		t.Fatalf("initial volume size (%v) is not equal to requested volume size (%v)", pv.Spec.Capacity["storage"], resource.MustParse("5Gi"))
	}

	t.Log("Updating pvc to request more size")
	createdPVC.Spec.Resources.Requests = v1.ResourceList{
		v1.ResourceStorage: resource.MustParse("6Gi"),
	}

	updatedPVC, err := client.CoreV1().PersistentVolumeClaims(namespace).Update(createdPVC)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for volume %q to be resized ...", pvName)
	resizedPv, err := waitForVolumeCapacityChange(client, pvName, pv.Spec.Capacity)
	if err != nil {
		t.Error(err)
	}

	if resizedPv.Spec.Capacity["storage"] != resource.MustParse("6Gi") {
		t.Fatalf("volume size (%v) is not equal to requested volume size (%v)", pv.Spec.Capacity["storage"], resource.MustParse("6Gi"))
	}

	t.Logf("Waiting for volume claim %q to be resized ...", claimName)
	resizedPVC, err := waitForVolumeClaimCapacityChange(client, claimName, updatedPVC.Status.Capacity)
	if err != nil {
		t.Error(err)
	}

	if resizedPVC.Status.Capacity["storage"] != resource.MustParse("6Gi") {
		t.Fatalf("claim capacity (%v) is not equal to requested capacity (%v)", resizedPVC.Status.Capacity["storage"], resource.MustParse("6Gi"))
	}
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
			StorageClassName: strPtr(testStorageClass),
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
			StorageClassName: strPtr(testStorageClass),
		},
	}
	_, err = client.CoreV1().PersistentVolumeClaims(namespace).Create(pvc2)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting pod %q to be running ...\n", pod.Name)
	if _, err := waitForPod(client, pod.Name); err != nil {
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
			// Write the data in an InitContainer so that we can guarantee
			// it's been written before we reach running in the main container.
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
						"sh", "-c",
						"echo testcanary > /data/canary && sync",
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
						"sh", "-c",
						"sleep 1000000",
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
			StorageClassName: strPtr(testStorageClass),
		},
	}

	t.Log("Creating pvc")
	_, err = client.CoreV1().PersistentVolumeClaims(namespace).Create(pvc)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for pod %q to be running ...\n", pod.Name)
	if _, err := waitForPod(client, pod.Name); err != nil {
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
			VolumeSnapshotClassName: strPtr(testStorageClass),
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
			StorageClassName: strPtr(testStorageClass),
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
	if _, err := waitForPod(client, restoredPod.Name); err != nil {
		t.Error(err)
	}

	t.Log("Finished!")
}

func TestUnpublishOnDetachedVolume(t *testing.T) {
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
			StorageClassName: strPtr(testStorageClass),
		},
	}

	t.Log("Creating pvc")
	_, err = client.CoreV1().PersistentVolumeClaims(namespace).Create(pvc)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for pod %q to be running", pod.Name)
	pod, err = waitForPod(client, pod.Name)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Discovering droplet ID from pod")

	// The chain of IDs we have to connect to get from the pod to the droplet ID
	// is as following:
	// 1. Fetch the PVC claim name from the pod.
	// 2. Find the PV corresponding to the claim name and fetch its name as well
	//    as the volume ID.
	// 3. Find the VolumeAttachment corresponding to the PV name and fetch the
	//    node name (i.e., the node that the volume is attached to).
	// 4. Fetch the provider ID from the node object and extract the droplet ID
	//    from it.

	var pvcName string
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil {
			pvcName = vol.PersistentVolumeClaim.ClaimName
			break
		}
	}
	if pvcName == "" {
		t.Fatal("no persistent volume claim found on pod")
	}

	pvs, err := client.CoreV1().PersistentVolumes().List(metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var pvName, volumeID string
	for _, pv := range pvs.Items {
		if pv.Spec.ClaimRef.Name == pvcName {
			pvName = pv.ObjectMeta.Name
			volumeID = pv.Spec.CSI.VolumeHandle
			break
		}
	}
	if pvName == "" {
		t.Fatalf("no persistent volume with claim reference %q found", pvcName)
	}
	if volumeID == "" {
		t.Fatal("volume ID should have been discovered together with persist volume name")
	}

	vaList, err := client.StorageV1().VolumeAttachments().List(metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var attachedNodeName string
	for _, va := range vaList.Items {
		if va.Spec.Source.PersistentVolumeName != nil && *va.Spec.Source.PersistentVolumeName == pvName {
			attachedNodeName = va.Spec.NodeName
			break
		}
	}
	if attachedNodeName == "" {
		t.Fatalf("no volume attachment with persistent volume name %q found (number of volume attachments found: %d)", pvName, len(vaList.Items))
	}

	nodes, err := client.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	var dropletID int
	for _, node := range nodes.Items {
		if node.Name == attachedNodeName {
			dropletIDStr := strings.TrimPrefix(node.Spec.ProviderID, "digitalocean://")
			var err error
			dropletID, err = strconv.Atoi(dropletIDStr)
			if err != nil {
				t.Fatalf("failed to convert integer part %s of provider ID %q to integer: %s", dropletIDStr, node.Spec.ProviderID, err)
			}
			break
		}
	}
	if dropletID == 0 {
		t.Fatalf("no node by name %q found", attachedNodeName)
	}

	t.Log("Detaching volume directly")
	action, _, err := doClient.StorageActions.DetachByDropletID(context.Background(), volumeID, dropletID)
	if err != nil {
		t.Fatal(err)
	}

	if action != nil {
		err := wait.PollImmediate(5*time.Second, 2*time.Minute, wait.ConditionFunc(func() (bool, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			action, _, err := doClient.StorageActions.Get(ctx, volumeID, action.ID)
			if err != nil {
				return false, err
			}

			return action.Status == godo.ActionCompleted, nil
		}))
		if err != nil {
			t.Fatal(err)
		}
	}

	err = client.CoreV1().Pods(namespace).Delete(pod.Name, &metav1.DeleteOptions{})
	if err != nil {
		t.Fatal(err)
	}
}

func setup() error {
	// Default storage class is "do-block-storage" but we can override it via an
	// environment variable.
	if storageClass := os.Getenv("TEST_STORAGE_CLASS"); storageClass != "" {
		testStorageClass = storageClass
	}

	if skip := os.Getenv("SKIP_CLEANUP"); skip != "" && skip != "false" {
		skipCleanup = true
	}

	// Create godo client.
	var doToken string
	for _, tokenEnvVar := range tokenEnvVars {
		if doToken = os.Getenv(tokenEnvVar); doToken != "" {
			break
		}
	}

	if doToken == "" {
		return fmt.Errorf("DO API token must be provided in one of the following environment variables: %s", strings.Join(tokenEnvVars, ", "))
	}

	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: doToken,
	})
	oauthClient := oauth2.NewClient(context.Background(), tokenSource)

	opts := []godo.ClientOpt{
		godo.SetUserAgent("csi-digitalocean/integration-tests"),
	}

	var err error
	doClient, err = godo.New(oauthClient, opts...)
	if err != nil {
		return fmt.Errorf("failed to create DigitalOcean client: %s", err)
	}

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
	if skipCleanup {
		return nil
	}

	// delete all test resources
	err := client.CoreV1().Namespaces().Delete(namespace, nil)
	if err != nil && !kubeerrors.IsNotFound(err) {
		return err
	}

	// Wait for namespace delete to complete so that subsequent test runs fired
	// off with little delay do not run into an error when the namespace still
	// exists (i.e., deletion is still in progress).
	return waitForNamespaceDelete(client, namespace)
}

func strPtr(s string) *string {
	return &s
}

func waitForNamespaceDelete(client kubernetes.Interface, name string) error {
	err := wait.PollImmediate(3*time.Second, 5*time.Minute, wait.ConditionFunc(func() (bool, error) {
		_, err := client.CoreV1().Namespaces().Get(name, metav1.GetOptions{})
		if kubeerrors.IsNotFound(err) {
			return true, nil
		}

		return false, err
	}))

	return err
}

// waitForPod waits for the given pod name to be running
func waitForPod(client kubernetes.Interface, name string) (*v1.Pod, error) {
	var resultPod *v1.Pod
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
					resultPod = pod
					close(stopCh)
					return
				}
			},
		})

	controller.Run(stopCh)
	return resultPod, err
}

// waitForVolumeCapacityChange waits for the given volume's capacity to be changed
func waitForVolumeCapacityChange(client kubernetes.Interface, name string, resourceList v1.ResourceList) (*v1.PersistentVolume, error) {
	var err error
	var pv *v1.PersistentVolume
	stopCh := make(chan struct{})

	go func() {
		select {
		case <-time.After(time.Minute * 5):
			err = errors.New("timing out waiting for pv capcity change")
			close(stopCh)
		case <-stopCh:
		}
	}()

	watchlist := cache.NewListWatchFromClient(client.CoreV1().RESTClient(),
		"persistentvolumes", v1.NamespaceAll, fields.Everything())
	_, controller := cache.NewInformer(watchlist, &v1.PersistentVolume{}, time.Second*1,
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(o, n interface{}) {
				volume := n.(*v1.PersistentVolume)
				if name != volume.Name {
					return
				}
				if volume.Status.Phase == v1.VolumeFailed {
					err = errors.New("Persistent volume status is Failed")
					close(stopCh)
					return
				}

				if volume.Status.Phase == v1.VolumeBound && volume.Spec.Capacity["storage"] != resourceList["storage"] {
					pv = volume
					close(stopCh)
					return
				}
			},
		})

	controller.Run(stopCh)
	return pv, err
}

// waitForVolumeClaimCapacityChange waits for the given volume claim's capacity to be changed
func waitForVolumeClaimCapacityChange(client kubernetes.Interface, name string, resourceList v1.ResourceList) (*v1.PersistentVolumeClaim, error) {
	var err error
	var pvc *v1.PersistentVolumeClaim
	stopCh := make(chan struct{})

	go func() {
		select {
		case <-time.After(time.Minute * 5):
			err = errors.New("timing out waiting for pvc capcity change")
			close(stopCh)
		case <-stopCh:
		}
	}()

	watchlist := cache.NewListWatchFromClient(client.CoreV1().RESTClient(),
		"persistentvolumeclaims", namespace, fields.Everything())
	_, controller := cache.NewInformer(watchlist, &v1.PersistentVolumeClaim{}, time.Second*1,
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(o, n interface{}) {
				claim := n.(*v1.PersistentVolumeClaim)
				if name != claim.Name {
					return
				}
				if claim.Status.Phase == v1.ClaimLost {
					err = errors.New("Persistent volume claim status is Lost")
					close(stopCh)
					return
				}
				if claim.Status.Phase == v1.ClaimBound && claim.Status.Capacity["storage"] != resourceList["storage"] {
					pvc = claim
					close(stopCh)
					return
				}
			},
		})

	controller.Run(stopCh)
	return pvc, err
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
