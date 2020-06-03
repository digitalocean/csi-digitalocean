package main

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	v1beta1snapshot "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/utils/pointer"
)

const dobsStorageClassName = "do-block-storage"

type createVolSnapshotClassOpts struct {
	snapshotter string
	isDefault   bool
}

func createVolSnapshotClass(opts createVolSnapshotClassOpts) *unstructured.Unstructured {
	if opts.snapshotter == "" {
		opts.snapshotter = dobsDriverNewStyle
	}

	annotations := map[string]interface{}{}
	if opts.isDefault {
		annotations["snapshot.storage.kubernetes.io/is-default-class"] = "true"
	}

	class := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "snapshot.storage.k8s.io/v1alpha1",
			"kind":       "VolumeSnapshotClass",
			"metadata": map[string]interface{}{
				"name":        dobsStorageClassName,
				"annotations": annotations,
			},
			"snapshotter": opts.snapshotter,
		},
	}

	return class
}

func TestWriteVolumeSnapshotObjectsClass(t *testing.T) {
	tests := []struct {
		name              string
		inVolumeSnapClass *unstructured.Unstructured
		wantErr           error
		wantDeletePolicy  string
	}{
		{
			name:              "VolumeSnapshotClass missing",
			inVolumeSnapClass: nil,
			wantErr:           errNoVolumeSnapshotClassFound,
		},
		{
			name: "new style snapshotter",
			inVolumeSnapClass: createVolSnapshotClass(createVolSnapshotClassOpts{
				snapshotter: dobsDriverNewStyle,
			}),
			wantErr: nil,
		},
		{
			name: "old style snapshotter",
			inVolumeSnapClass: createVolSnapshotClass(createVolSnapshotClassOpts{
				snapshotter: dobsDriverOldStyle,
			}),
			wantErr: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.name != "new style snapshotter" {
				t.Skip()
			}
			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme)

			if test.inVolumeSnapClass != nil {
				_, err := client.Resource(alphaSnapClassRes).Create(context.Background(), test.inVolumeSnapClass, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create VolumeSnapshotClass: %s", err)
				}
			}

			volWriter, err := newVolumeSnapshotWriter("")
			if err != nil {
				t.Fatalf("failed to create volume snapshot writer: %s", err)
			}

			err = writeVolumeSnapshotObjects(context.Background(), client, volWriter)
			if err != test.wantErr {
				t.Fatalf("got error %q, want error: %q", err, test.wantErr)
			}
		})
	}
}

func TestWriteVolumeSnapshotObjectsContent(t *testing.T) {
	tests := []struct {
		name                   string
		dobsIsDefaultClass     bool
		inAlphaVolSnapContents []*unstructured.Unstructured
		wantVolSnapContents    []v1beta1snapshot.VolumeSnapshotContent
	}{
		{
			name:               "no VolumeSnapshotContents",
			dobsIsDefaultClass: true,
		},
		{
			name:               "VolumeSnapshotContent from different default class",
			dobsIsDefaultClass: false,
			inAlphaVolSnapContents: []*unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"apiVersion": "snapshot.storage.k8s.io/v1alpha1",
						"kind":       "VolumeSnapshotContent",
						"metadata": map[string]interface{}{
							"name": "my-volumesnapshotcontent",
						},
						"spec": map[string]interface{}{
							"csiVolumeSnapshotSource": map[string]interface{}{
								"driver":         "other.driver",
								"snapshotHandle": "12cb8633-f3b2-422a-bc64-eef690c17f14",
							},
							"volumeSnapshotRef": map[string]interface{}{
								"name":      "my-volumesnapshot",
								"namespace": "default",
							},
							"persistentVolumeRef": map[string]interface{}{
								"name":      "my-persistentvolume",
								"namespace": "default",
							},
						},
					},
				},
			},
		},
		{
			name:               "VolumeSnapshotContent from different explicit class",
			dobsIsDefaultClass: true,
			inAlphaVolSnapContents: []*unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"apiVersion": "snapshot.storage.k8s.io/v1alpha1",
						"kind":       "VolumeSnapshotContent",
						"metadata": map[string]interface{}{
							"name": "my-volumesnapshotcontent",
						},
						"spec": map[string]interface{}{
							"csiVolumeSnapshotSource": map[string]interface{}{
								"driver":         "other.driver",
								"snapshotHandle": "12cb8633-f3b2-422a-bc64-eef690c17f14",
							},
							"volumeSnapshotRef": map[string]interface{}{
								"name":      "my-volumesnapshot",
								"namespace": "default",
							},
							"persistentVolumeRef": map[string]interface{}{
								"name":      "my-persistentvolume",
								"namespace": "default",
							},
							"snapshotClassName": "other-class",
						},
					},
				},
			},
		},
		{
			name:               "VolumeSnapshotContents found",
			dobsIsDefaultClass: true,
			inAlphaVolSnapContents: []*unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"apiVersion": "snapshot.storage.k8s.io/v1alpha1",
						"kind":       "VolumeSnapshotContent",
						"metadata": map[string]interface{}{
							"name": "my-volumesnapshotcontent1",
						},
						"spec": map[string]interface{}{
							"csiVolumeSnapshotSource": map[string]interface{}{
								"driver":         dobsDriverNewStyle,
								"snapshotHandle": "12cb8633-f3b2-422a-bc64-eef690c17f14",
							},
							"volumeSnapshotRef": map[string]interface{}{
								"name":      "my-volumesnapshot1",
								"namespace": "default",
							},
							"persistentVolumeRef": map[string]interface{}{
								"name":      "my-persistentvolume1",
								"namespace": "default",
							},
							"snapshotClassName": dobsStorageClassName,
						},
					},
				},
				{
					Object: map[string]interface{}{
						"apiVersion": "snapshot.storage.k8s.io/v1alpha1",
						"kind":       "VolumeSnapshotContent",
						"metadata": map[string]interface{}{
							"name": "my-volumesnapshotcontent2",
						},
						"spec": map[string]interface{}{
							"csiVolumeSnapshotSource": map[string]interface{}{
								"driver":         dobsDriverNewStyle,
								"snapshotHandle": "69e7bb48-c1a6-41fe-b090-6f17f6bfd862",
							},
							"volumeSnapshotRef": map[string]interface{}{
								"name":      "my-volumesnapshot2",
								"namespace": "default",
							},
							"persistentVolumeRef": map[string]interface{}{
								"name":      "my-persistentvolume2",
								"namespace": "default",
							},
						},
					},
				},
			},
			wantVolSnapContents: []v1beta1snapshot.VolumeSnapshotContent{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "VolumeSnapshotContent",
						APIVersion: v1beta1snapshot.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-volumesnapshotcontent1",
					},
					Spec: v1beta1snapshot.VolumeSnapshotContentSpec{
						VolumeSnapshotRef: corev1.ObjectReference{
							Name:      "my-volumesnapshot1",
							Namespace: "default",
						},
						Driver:                  dobsDriverNewStyle,
						VolumeSnapshotClassName: pointer.StringPtr(dobsStorageClassName),
						Source: v1beta1snapshot.VolumeSnapshotContentSource{
							SnapshotHandle: pointer.StringPtr("12cb8633-f3b2-422a-bc64-eef690c17f14"),
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "VolumeSnapshotContent",
						APIVersion: v1beta1snapshot.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-volumesnapshotcontent2",
					},
					Spec: v1beta1snapshot.VolumeSnapshotContentSpec{
						VolumeSnapshotRef: corev1.ObjectReference{
							Name:      "my-volumesnapshot2",
							Namespace: "default",
						},
						Driver: dobsDriverNewStyle,
						Source: v1beta1snapshot.VolumeSnapshotContentSource{
							SnapshotHandle: pointer.StringPtr("69e7bb48-c1a6-41fe-b090-6f17f6bfd862"),
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme)

			volumeSnapshotClass := createVolSnapshotClass(createVolSnapshotClassOpts{
				isDefault: test.dobsIsDefaultClass,
			})

			_, err := client.Resource(alphaSnapClassRes).Create(context.Background(), volumeSnapshotClass, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("failed to create VolumeSnapshotClass: %s", err)
			}

			for _, alphaVolSnapContent := range test.inAlphaVolSnapContents {
				_, err := client.Resource(alphaSnapContentRes).Create(context.Background(), alphaVolSnapContent, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create VolumeSnapshotContent: %s", err)
				}
			}

			volWriter, err := newVolumeSnapshotWriter("")
			if err != nil {
				t.Fatalf("failed to create volume snapshot writer: %s", err)
			}

			err = writeVolumeSnapshotObjects(context.Background(), client, volWriter)
			if err != nil {
				t.Fatalf("got writing error: %s", err)
			}

			gotVolSnapContents := volWriter.objBuffer.volSnapshotContents
			if diff := cmp.Diff(test.wantVolSnapContents, gotVolSnapContents); diff != "" {
				t.Fatalf("VolumeSnapshotContents mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWriteVolumeSnapshotObjects(t *testing.T) {
	tests := []struct {
		name                string
		dobsIsDefaultClass  bool
		inAlphaVolSnapshots []*unstructured.Unstructured
		wantVolSnapshots    []v1beta1snapshot.VolumeSnapshot
	}{
		{
			name:               "no VolumeSnapshots",
			dobsIsDefaultClass: true,
		},
		{
			name:               "VolumeSnapshot from different default class",
			dobsIsDefaultClass: false,
			inAlphaVolSnapshots: []*unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"apiVersion": "snapshot.storage.k8s.io/v1alpha1",
						"kind":       "VolumeSnapshot",
						"metadata": map[string]interface{}{
							"name": "my-volumesnapshot",
						},
						"spec": map[string]interface{}{
							"source": map[string]interface{}{
								"kind": "PersistentVolumeClaim",
								"name": "my-volumeclaim",
							},
							"snapshotContentName": "my-volumesnapshot",
						},
					},
				},
			},
		},
		{
			name:               "VolumeSnapshot from different explicit class",
			dobsIsDefaultClass: true,
			inAlphaVolSnapshots: []*unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"apiVersion": "snapshot.storage.k8s.io/v1alpha1",
						"kind":       "VolumeSnapshot",
						"metadata": map[string]interface{}{
							"name":      "my-volumesnapshot",
							"namespace": "backups",
						},
						"spec": map[string]interface{}{
							"source": map[string]interface{}{
								"kind": "PersistentVolumeClaim",
								"name": "my-volumeclaim",
							},
							"snapshotContentName": "my-volumesnapshot",
							"snapshotClassName":   "other-class",
						},
					},
				},
			},
		},
		{
			name:               "VolumeSnapshots found",
			dobsIsDefaultClass: true,
			inAlphaVolSnapshots: []*unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"apiVersion": "snapshot.storage.k8s.io/v1alpha1",
						"kind":       "VolumeSnapshot",
						"metadata": map[string]interface{}{
							"name": "my-volumesnapshot1",
						},
						"spec": map[string]interface{}{
							"snapshotContentName": "my-volumesnapshot1",
							"snapshotClassName":   dobsStorageClassName,
						},
					},
				},
				{
					Object: map[string]interface{}{
						"apiVersion": "snapshot.storage.k8s.io/v1alpha1",
						"kind":       "VolumeSnapshot",
						"metadata": map[string]interface{}{
							"name":      "my-volumesnapshot2",
							"namespace": "backups",
						},
						"spec": map[string]interface{}{
							"source": map[string]interface{}{
								"kind": "PersistentVolumeClaim",
								"name": "my-volumeclaim2",
							},
						},
					},
				},
			},
			wantVolSnapshots: []v1beta1snapshot.VolumeSnapshot{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "VolumeSnapshot",
						APIVersion: v1beta1snapshot.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-volumesnapshot1",
					},
					Spec: v1beta1snapshot.VolumeSnapshotSpec{
						Source: v1beta1snapshot.VolumeSnapshotSource{
							VolumeSnapshotContentName: pointer.StringPtr("my-volumesnapshot1"),
						},
						VolumeSnapshotClassName: pointer.StringPtr(dobsStorageClassName),
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "VolumeSnapshot",
						APIVersion: v1beta1snapshot.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-volumesnapshot2",
						Namespace: "backups",
					},
					Spec: v1beta1snapshot.VolumeSnapshotSpec{
						Source: v1beta1snapshot.VolumeSnapshotSource{
							PersistentVolumeClaimName: pointer.StringPtr("my-volumeclaim2"),
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme)

			volumeSnapshotClass := createVolSnapshotClass(createVolSnapshotClassOpts{
				isDefault: test.dobsIsDefaultClass,
			})

			_, err := client.Resource(alphaSnapClassRes).Create(context.Background(), volumeSnapshotClass, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("failed to create VolumeSnapshotClass: %s", err)
			}

			for _, alphaVolSnapshot := range test.inAlphaVolSnapshots {
				namespace, _, err := unstructured.NestedString(alphaVolSnapshot.Object, "metadata", "namespace")
				if err != nil {
					t.Fatalf("failed to get namespace from v1alpha1.Snapshot: %s", err)
				}

				_, err = client.Resource(alphaSnapshotRes).Namespace(namespace).Create(context.Background(), alphaVolSnapshot, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create VolumeSnapshot: %s", err)
				}
			}

			volWriter, err := newVolumeSnapshotWriter("")
			if err != nil {
				t.Fatalf("failed to create volume snapshot writer: %s", err)
			}

			err = writeVolumeSnapshotObjects(context.Background(), client, volWriter)
			if err != nil {
				t.Fatalf("got writing error: %s", err)
			}

			gotVolSnapshots := volWriter.objBuffer.volSnapshots
			if diff := cmp.Diff(test.wantVolSnapshots, gotVolSnapshots); diff != "" {
				t.Fatalf("VolumeSnapshots mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
