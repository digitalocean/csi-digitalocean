package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	v1beta1snapshot "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	"sigs.k8s.io/yaml"
)

const (
	fileVolumeSnapshotClassDeletionPolicy = "volumesnapshotclass_deletion-policy"
	subDirectoryVolumeSnapshotContents    = "01-volumesnapshotcontents"
	subDirectoryVolumeSnapshots           = "02-volumesnapshots"
)

type objectBuffer struct {
	classDeletionPolicy string
	volSnapshotContents []v1beta1snapshot.VolumeSnapshotContent
	volSnapshots        []v1beta1snapshot.VolumeSnapshot
}

type volumeSnapshotWriter struct {
	directory string
	objBuffer objectBuffer
}

func newVolumeSnapshotWriter(directory string) (*volumeSnapshotWriter, error) {
	if directory != "" {
		var err error
		directory, err = filepath.Abs(directory)
		if err != nil {
			return nil, err
		}

		if err := veryifyDirectoryPath(directory); err != nil {
			return nil, err
		}

	}

	return &volumeSnapshotWriter{
		directory: directory,
	}, nil
}

func (w *volumeSnapshotWriter) writeVolumeSnapshotContent(obj v1beta1snapshot.VolumeSnapshotContent) error {
	data, err := yaml.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal v1beta1/VolumeSnapshotContent into YAML: %s", err)
	}

	if w.directory == "" {
		w.objBuffer.volSnapshotContents = append(w.objBuffer.volSnapshotContents, obj)
		fmt.Println(string(data))
		return nil
	}

	dir := filepath.Join(w.directory, subDirectoryVolumeSnapshotContents)
	return w.writeFile(dir, obj.ObjectMeta.Name, data)
}

func (w *volumeSnapshotWriter) writeVolumeSnapshot(obj v1beta1snapshot.VolumeSnapshot) error {
	data, err := yaml.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal v1beta1/VolumeSnapshot into YAML: %s", err)
	}

	if w.directory == "" {
		w.objBuffer.volSnapshots = append(w.objBuffer.volSnapshots, obj)
		fmt.Println(string(data))
		return nil
	}

	dir := filepath.Join(w.directory, subDirectoryVolumeSnapshots)
	return w.writeFile(dir, obj.ObjectMeta.Name, data)
}

func (w *volumeSnapshotWriter) writeFile(directory, filename string, data []byte) error {
	err := os.MkdirAll(directory, 0755)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(directory, filename)+".yaml", data, 0644)
}

func veryifyDirectoryPath(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// This is okay because we invoke MkdirAll later.
			return nil
		}
		return err
	}

	if !fi.Mode().IsDir() {
		return fmt.Errorf("given path %q is not a directory", path)
	}

	return nil
}
