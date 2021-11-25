package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	v1beta1snapshot "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	dobsDriverNewStyle = "dobs.csi.digitalocean.com"
	dobsDriverOldStyle = "com.digitalocean.csi.dobs"
)

var (
	alphaSnapClassRes   = schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1alpha1", Resource: "volumesnapshotclasses"}
	alphaSnapContentRes = schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1alpha1", Resource: "volumesnapshotcontents"}
	alphaSnapshotRes    = schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1alpha1", Resource: "volumesnapshots"}
)

var errNoVolumeSnapshotClassFound = fmt.Errorf("could not find v1alpha1/VolumeStorageClass for driver %q", dobsDriverNewStyle)

type snapshotClassDefinition struct {
	dobsDriver         string
	dobsSnapClassName  string
	dobsIsDefaultClass bool
}

type params struct {
	kubeconfig string
	server     string
	directory  string
}

var p params

func errExit(err error) {
	fmt.Fprintf(os.Stderr, "Failed to migrate snapshots: %s\n", err)
	os.Exit(1)
}

func main() {
	var defaultKubeConfigPath string
	home, err := os.UserHomeDir()
	if err == nil {
		defaultKubeConfigPath = filepath.Join(home, ".kube", "config")
	}

	flag.StringVar(&p.kubeconfig, "kubeconfig", defaultKubeConfigPath, "(optional) absolute path to the kubeconfig file")
	flag.StringVar(&p.server, "server", "", "(optional) address and port of the Kubernetes API server")
	flag.StringVar(&p.directory, "directory", "", "the top-level directory to write YAML-marshaled objects into, or stdout if omitted")

	flag.Parse()

	if envVar := os.Getenv("KUBECONFIG"); envVar != "" {
		p.kubeconfig = envVar
	}

	if p.server != "" {
		p.kubeconfig = ""
	}

	if err := run(p); err != nil {
		errExit(err)
	}
}

func run(p params) error {
	config, err := clientcmd.BuildConfigFromFlags(p.server, p.kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to create config from flags: %s", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client for v1alpha1 snapshots: %s", err)
	}

	volWriter, err := newVolumeSnapshotWriter(p.directory)
	if err != nil {
		return fmt.Errorf("failed to create volume snapshot writer: %s", err)
	}

	err = writeVolumeSnapshotObjects(context.TODO(), dynClient, volWriter)
	if err != nil {
		return fmt.Errorf("failed to write volume snapshot objects: %s", err)
	}

	return nil
}

func writeVolumeSnapshotObjects(ctx context.Context, client dynamic.Interface, volWriter *volumeSnapshotWriter) error {
	snapClassDef, err := analyzeVolumeSnapshotClass(ctx, client.Resource(alphaSnapClassRes))
	if err != nil {
		return err
	}
	fmt.Printf("Found VolumeSnapshotClass for DigitalOcean Block Storage (driver=%s, name=%s, default class? %t)\n", snapClassDef.dobsDriver, snapClassDef.dobsSnapClassName, snapClassDef.dobsIsDefaultClass)

	num, err := writeVolumeSnapshotContents(ctx, client.Resource(alphaSnapContentRes), snapClassDef, volWriter)
	if err != nil {
		return err
	}
	fmt.Printf("Converted %d VolumeSnapshotContent object(s)\n", num)

	num, err = writeVolumeSnapshots(ctx, client.Resource(alphaSnapshotRes), snapClassDef, volWriter)
	if err != nil {
		return err
	}

	fmt.Printf("Converted %d VolumeSnapshot object(s)\n", num)
	return nil
}

// analyzeVolumeSnapshotClass extracts metadata from the DigitalOcean Block
// Storage (DOBS) VolumeSnapshotClass needed to write other volume snapshot
// objects.
func analyzeVolumeSnapshotClass(ctx context.Context, client dynamic.NamespaceableResourceInterface) (*snapshotClassDefinition, error) {
	alphaSnapClasses, err := client.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list v1alpha1/VolumeSnapshotClass: %s", err)
	}

	for _, alphaSnapClass := range alphaSnapClasses.Items {
		name, _, err := unstructured.NestedString(alphaSnapClass.Object, "metadata", "name")
		if err != nil {
			return nil, fmt.Errorf("failed to get v1alpha1/VolumeSnapshotClass.metadata.name: %s", err)
		}

		snapshotter, found, err := unstructured.NestedString(alphaSnapClass.Object, "snapshotter")
		if err != nil {
			return nil, fmt.Errorf("failed to get v1alpha1/VolumeSnapshotClass.snapshotter for %s: %s", name, err)
		}
		if !found {
			return nil, fmt.Errorf("v1alpha1 VolumeSnapshotClass.snapshotter does not exist for %s", name)
		}
		if snapshotter == dobsDriverNewStyle || snapshotter == dobsDriverOldStyle {
			dobsDriver := snapshotter
			dobsSnapClassName := name

			isDefaultClass, _, err := unstructured.NestedString(alphaSnapClass.Object, "metadata", "annotations", "snapshot.storage.kubernetes.io/is-default-class")
			if err != nil {
				return nil, fmt.Errorf("failed to get v1alpha1/VolumeSnapshotClass.metadataannotation.'snapshot.storage.kubernetes.io/is-default-class' for %s: %s", name, err)
			}
			dobsIsDefaultClass := isDefaultClass == "true"

			return &snapshotClassDefinition{dobsDriver, dobsSnapClassName, dobsIsDefaultClass}, nil
		}
	}

	return nil, errNoVolumeSnapshotClassFound
}

// writeVolumeSnapshotContents converts v1alpha1 VolumeSnapshotContent objects
// into their v1beta1 equivalents and writes them out.
func writeVolumeSnapshotContents(ctx context.Context, client dynamic.NamespaceableResourceInterface, snapClassDef *snapshotClassDefinition, volWriter *volumeSnapshotWriter) (num int, err error) {
	alphaSnapContents, err := client.List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to list v1alpha1/VolumeSnapshotContents: %s", err)
	}

	for _, alphaSnapContent := range alphaSnapContents.Items {
		name, _, err := unstructured.NestedString(alphaSnapContent.Object, "metadata", "name")
		if err != nil {
			return 0, fmt.Errorf("failed get v1alpha1/VolumeSnapshotContent.metadata.name: %s", err)
		}

		volSnapClassNameStr, found, err := unstructured.NestedString(alphaSnapContent.Object, "spec", "snapshotClassName")
		if err != nil {
			return 0, fmt.Errorf("failed get v1alpha1/VolumeSnapshotContent.spec.snapshotClassName for %s: %s", name, err)
		}
		if !found && !snapClassDef.dobsIsDefaultClass {
			fmt.Printf("Skipping v1alpha1/VolumeSnapshotContent %q because spec.snapshotClassName is missing and DOBS is not the default driver\n", name)
			continue
		}

		var volSnapClassName *string
		if found {
			if volSnapClassNameStr != snapClassDef.dobsSnapClassName {
				fmt.Printf("Skipping v1alpha1/VolumeSnapshotContent %q because object is managed by non-DOBS driver %q\n", name, volSnapClassNameStr)
				continue
			}

			volSnapClassName = &volSnapClassNameStr
		}

		volSnapRefName, _, err := unstructured.NestedString(alphaSnapContent.Object, "spec", "volumeSnapshotRef", "name")
		if err != nil {
			return 0, fmt.Errorf("failed get v1alpha1/VolumeSnapshotContent.spec.volumeSnapshotRef.name for %s: %s", name, err)
		}
		volSnapRefNamespace, _, err := unstructured.NestedString(alphaSnapContent.Object, "spec", "volumeSnapshotRef", "namespace")
		if err != nil {
			return 0, fmt.Errorf("failed get v1alpha1/VolumeSnapshotContent.spec.volumeSnapshotRef.namespace for %s: %s", name, err)
		}

		deletionPolicyPtr, _, err := unstructured.NestedFieldNoCopy(alphaSnapContent.Object, "spec", "deletionPolicy")
		if err != nil {
			return 0, fmt.Errorf("failed get v1alpha1/VolumeSnapshotContent.spec.deletionPolicy for %s: %s", name, err)
		}
		// Delete is the default policy.
		deletionPolicy := "Delete"
		if deletionPolicyPtr != nil {
			deletionPolicyStrPtr, ok := deletionPolicyPtr.(*string)
			if !ok {
				return 0, fmt.Errorf("v1alpha1/VolumeSnapshotContent.spec.deletionPolicy is type %T; expected *string", deletionPolicyPtr)
			}
			deletionPolicy = *deletionPolicyStrPtr
		}

		csiVolSnapSourceSnapHandleStr, found, err := unstructured.NestedString(alphaSnapContent.Object, "spec", "csiVolumeSnapshotSource", "snapshotHandle")
		if err != nil {
			return 0, fmt.Errorf("failed get v1alpha1/VolumeSnapshotContent.spec.csiVolumeSnapshotSource.snapshotHandle for %s: %s", name, err)
		}
		var csiVolSnapSourceSnapHandle *string
		if found {
			csiVolSnapSourceSnapHandle = &csiVolSnapSourceSnapHandleStr
		}

		betaSnapContent := v1beta1snapshot.VolumeSnapshotContent{
			TypeMeta: metav1.TypeMeta{
				Kind:       "VolumeSnapshotContent",
				APIVersion: v1beta1snapshot.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: v1beta1snapshot.VolumeSnapshotContentSpec{
				VolumeSnapshotRef: corev1.ObjectReference{
					Name:      volSnapRefName,
					Namespace: volSnapRefNamespace,
				},
				DeletionPolicy:          v1beta1snapshot.DeletionPolicy(deletionPolicy),
				Driver:                  snapClassDef.dobsDriver,
				VolumeSnapshotClassName: volSnapClassName,
				Source: v1beta1snapshot.VolumeSnapshotContentSource{
					SnapshotHandle: csiVolSnapSourceSnapHandle,
				},
			},
		}

		fmt.Printf("--- Writing VolumeSnapshotContent %q\n", name)
		err = volWriter.writeVolumeSnapshotContent(betaSnapContent)
		if err != nil {
			return 0, fmt.Errorf("failed to write VolumeSnapshotContent %q: %s", name, err)
		}

		num++
	}

	return num, nil
}

// writeVolumeSnapshots converts v1alpha1 VolumeSnapshot objects into their
// v1beta1 equivalents and writes them out.
func writeVolumeSnapshots(ctx context.Context, client dynamic.NamespaceableResourceInterface, snapClassDef *snapshotClassDefinition, volWriter *volumeSnapshotWriter) (num int, err error) {
	alphaSnapshots, err := client.List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to list v1alpha1/VolumeSnapshots: %s", err)
	}

	for _, alphaSnapshot := range alphaSnapshots.Items {
		name, _, err := unstructured.NestedString(alphaSnapshot.Object, "metadata", "name")
		if err != nil {
			return 0, fmt.Errorf("failed get v1alpha1/VolumeSnapshot.metadata.name: %s", err)
		}

		volSnapClassNameStr, found, err := unstructured.NestedString(alphaSnapshot.Object, "spec", "snapshotClassName")
		if err != nil {
			return 0, fmt.Errorf("failed get v1alpha1/VolumeSnapshot.spec.snapshotClassName for %s: %s", name, err)
		}
		if !found && !snapClassDef.dobsIsDefaultClass {
			fmt.Printf("Skipping v1alpha1/VolumeSnapshot %q because spec.snapshotClassName is missing and DOBS is not the default driver\n", name)
			continue
		}

		var volSnapClassName *string
		if found {
			if volSnapClassNameStr != snapClassDef.dobsSnapClassName {
				fmt.Printf("Skipping v1alpha1/VolumeSnapshot %q because object is managed by non-DOBS driver %q\n", name, volSnapClassNameStr)
				continue
			}

			volSnapClassName = &volSnapClassNameStr
		}

		namespace, _, err := unstructured.NestedString(alphaSnapshot.Object, "metadata", "namespace")
		if err != nil {
			return 0, fmt.Errorf("failed get v1alpha1/VolumeSnapshot.metadata.namespace for %s: %s", name, err)
		}

		var persistentVolumeClaimName *string
		persistentVolumeClaimNameStr, found, err := unstructured.NestedString(alphaSnapshot.Object, "spec", "source", "name")
		if err != nil {
			return 0, fmt.Errorf("failed get v1alpha1/VolumeSnapshot.spec.source.name for %s: %s", name, err)
		}
		if found {
			persistentVolumeClaimName = &persistentVolumeClaimNameStr
		}

		var snapshotContentName *string
		snapshotContentNameStr, found, err := unstructured.NestedString(alphaSnapshot.Object, "spec", "snapshotContentName")
		if err != nil {
			return 0, fmt.Errorf("failed get v1alpha1/VolumeSnapshot.spec.snapshotContentName for %s: %s", name, err)
		}
		if found {
			snapshotContentName = &snapshotContentNameStr
		}

		betaSnapshot := v1beta1snapshot.VolumeSnapshot{
			TypeMeta: metav1.TypeMeta{
				Kind:       "VolumeSnapshot",
				APIVersion: v1beta1snapshot.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: v1beta1snapshot.VolumeSnapshotSpec{
				Source: v1beta1snapshot.VolumeSnapshotSource{
					VolumeSnapshotContentName: snapshotContentName,
				},
				VolumeSnapshotClassName: volSnapClassName,
			},
		}

		if snapshotContentName == nil {
			// The VolumeSnapshot might not be bound to a VolumeSnapshotContent
			// yet (either due to delay or pathological reasons). Associate any
			// PersistentVolumeClaim name we may have instead. (Note that
			// exactly one of VolumeSnapshotContentName and
			// PersistentVolumeClaimName may be defined on a VolumeSnapshot at
			// any given time.)
			betaSnapshot.Spec.Source.PersistentVolumeClaimName = persistentVolumeClaimName
		}

		fmt.Printf("--- Writing VolumeSnapshot %q\n", name)
		err = volWriter.writeVolumeSnapshot(betaSnapshot)
		if err != nil {
			return 0, fmt.Errorf("failed to write VolumeSnapshot %q: %s", name, err)
		}

		num++
	}

	return num, nil
}
