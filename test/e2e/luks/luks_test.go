package luks_test

import (
	"context"
	"fmt"
	"ginkgo/framework"
	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"time"
)

var _ = Describe("LUKS", func() {
	f := framework.NewDefaultFramework("luks-e2e-test")

	var (
		client          clientset.Interface
		podName         string
		err             error
		deletePolicy    = metav1.DeletePropagationForeground
		secretName      = "luks-e2e-key"
		pvcName         = "luks-e2e-pvc"
		scName          = "luks-e2e-sc"
		scReclaimPolicy = v1.PersistentVolumeReclaimDelete
		volBindMode     = storagev1.VolumeBindingImmediate
	)

	BeforeEach(func() {
		client = f.ClientSet
	})

	AfterEach(func() {
		podClient := client.CoreV1().Pods(f.Namespace.Name)

		// Delete pod.
		if err := podClient.Delete(context.TODO(), podName, metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}); err != nil {
			By(fmt.Sprintf("Failed to delete pod, %s", err.Error()))
		} else {
			By(fmt.Sprintf("Deleted pod: %s", podName))
		}

		// Delete PVC.
		if err := client.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Delete(context.TODO(), pvcName, metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}); err != nil {
			By(fmt.Sprintf("Failed to delete PVC, %s", err.Error()))
		} else {
			By(fmt.Sprintf("Deleted PVC: %s", pvcName))
		}

		// Delete secret.
		if err := client.CoreV1().Secrets(f.Namespace.Name).Delete(context.TODO(), secretName, metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}); err != nil {
			By(fmt.Sprintf("Failed to delete secret, %s", err.Error()))
		} else {
			By(fmt.Sprintf("Deleted secret: %s", secretName))
		}

		// Delete storage class.
		if err := client.StorageV1().StorageClasses().Delete(context.TODO(), scName, metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}); err != nil {
			By(fmt.Sprintf("Failed to delete storage class, %s", err.Error()))
		} else {
			By(fmt.Sprintf("Deleted storage class: %s", scName))
		}
	})

	Describe("LUKS storage", func() {
		Context("One pod requesting one LUKS PVC", func() {
			It("Creating pod", func() {
				// LUKS secret.
				secretConfig := &v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: f.Namespace.Name,
						Name:      secretName,
					},
					Data: map[string][]byte{
						"luksKey": []byte("luks-key"),
					},
					Type: v1.SecretTypeOpaque,
				}

				_, err = client.CoreV1().Secrets(f.Namespace.Name).Create(context.TODO(), secretConfig, metav1.CreateOptions{})
				By(fmt.Sprintf("Created secret: %s", secretName))

				// Storage class.
				scConfig := &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: scName,
					},
					Provisioner:       f.DriverData.DriverInfo.Name,
					VolumeBindingMode: &volBindMode,
					ReclaimPolicy:     &scReclaimPolicy,
					Parameters: map[string]string{
						"csi.storage.k8s.io/node-stage-secret-name":      secretName,
						"csi.storage.k8s.io/node-stage-secret-namespace": f.Namespace.Name,
						"dobs.csi.digitalocean.com/luks-cipher":          "aes-xts-plain64",
						"dobs.csi.digitalocean.com/luks-encrypted":       "true",
						"dobs.csi.digitalocean.com/luks-key-size":        "512",
					},
				}

				_, err = client.StorageV1().StorageClasses().Create(context.TODO(), scConfig, metav1.CreateOptions{})
				if err != nil {
					Fail(err.Error())
				}

				By(fmt.Sprintf("Created storage class: %s", scName))

				// PVC.
				volumeSize := "1Gi"
				if f.DriverData.DriverInfo.SupportedSizeRange.Min != "" {
					volumeSize = f.DriverData.DriverInfo.SupportedSizeRange.Min
				}

				pvcConfig := &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: pvcName,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceStorage: resource.MustParse(volumeSize),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{
							"ReadWriteOnce",
						},
						StorageClassName: &scName,
					},
				}

				pvc, err := client.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Create(context.TODO(), pvcConfig, metav1.CreateOptions{})
				if err != nil {
					Fail(err.Error())
				}

				By(fmt.Sprintf("Created PVC: %s", pvcName))

				// Wait for PVC 30s.
				count := 0
				for pvc.Spec.VolumeName == "" && count < 30 {
					By(fmt.Sprintf("Wait for PVC...[%ds]", count))
					time.Sleep(1 * time.Second)
					pvc, err = client.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), pvcName, metav1.GetOptions{})
					if err != nil {
						Fail(err.Error())
					}
					count++
				}

				if pvc.Spec.VolumeName == "" {
					Fail(fmt.Sprintf("PVC %s can't be provisioned", pvc.Name))
				}

				// Pod.
				// @TODO: leave only one once docker image is ready.
				podCmd := "df -T /data | grep /dev/mapper/" + pvc.Spec.VolumeName
				//podCmd := "df -T /data | grep /dev/disk/by-id/scsi-0DO_Volume_" + pvc.Spec.VolumeName

				podConfig := &v1.Pod{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Pod",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "pvc-luks-tester-",
						Namespace:    f.Namespace.Name,
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:    "pod",
								Image:   "busybox:latest",
								Command: []string{"/bin/sh", "-c", podCmd},
								VolumeMounts: []v1.VolumeMount{
									{
										MountPath: "/data",
										Name:      "data",
									},
								},
							},
						},
						Volumes: []v1.Volume{
							{
								Name: "data",
								VolumeSource: v1.VolumeSource{
									PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
										ClaimName: pvcName,
										ReadOnly:  false,
									},
								},
							},
						},
						RestartPolicy: v1.RestartPolicyNever,
					},
				}

				podClient := client.CoreV1().Pods(f.Namespace.Name)

				pod, err := podClient.Create(context.TODO(), podConfig, metav1.CreateOptions{})
				if err != nil {
					Fail(err.Error())
				}

				podName = pod.GetObjectMeta().GetName()

				By(fmt.Sprintf("Created pod: %q", podName))

				pod, err = podClient.Get(context.TODO(), podName, metav1.GetOptions{})
				if err != nil {
					Fail(err.Error())
				}

				// Wait for pod 120s.
				count = 0
				for pod.Status.Phase == v1.PodPending && count < 24 {
					By(fmt.Sprintf("Wait for pod...[%ds]", count*5))
					time.Sleep(5 * time.Second)
					pod, err = podClient.Get(context.TODO(), podName, metav1.GetOptions{})
					if err != nil {
						Fail(err.Error())
					}
					count++
				}

				if pod.Status.Phase == v1.PodFailed {
					Fail(fmt.Sprintf("pod %q failed with status: %+v", podName, pod.Status))
				}

				By("Luks volume was created successfully! Cleaning up.")
			})
		})
	})
})
