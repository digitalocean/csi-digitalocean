package driver

import (
	"context"
	"errors"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"strings"
	"testing"
)

type TestPodVolume struct {
	ClaimName    string
	SizeGB       int
	StorageClass string
	LuksKey      string
}

type TestPodDescriptor struct {
	Kind    string
	Name    string
	Volumes []TestPodVolume
}

type DiskInfo struct {
	PVCName        string `json:"pvcName"`
	DeviceName     string `json:"deviceName"`
	DeviceSize     int    `json:"deviceSize"`
	Filesystem     string `json:"filesystem"`
	FilesystemUUID string `json:"filesystemUUID"`
	FilesystemSize int    `json:"filesystemSize"`
	DeviceSource   string `json:"deviceSource"`
	Luks           string `json:"luks,omitempty"`
	Cipher         string `json:"cipher,omitempty"`
	Keysize        int    `json:"keysize,omitempty"`
}

// creates a kubernetes pod from the given TestPodDescriptor
func TestCreateLuksVolume(t *testing.T) {
	tests := []struct {
		name           string
		listVolumesErr error
		getSnapshotErr error
	}{
		{
			name:           "listing volumes failing",
			listVolumesErr: errors.New("failed to list volumes"),
		},
		{
			name:           "fetching snapshot failing",
			getSnapshotErr: errors.New("failed to get snapshot"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := &Driver{
				storage: &fakeStorageDriver{
					listVolumesErr: test.listVolumesErr,
				},
				snapshots: &fakeSnapshotsDriver{
					getSnapshotErr: test.getSnapshotErr,
				},
				log: logrus.New().WithField("test_enabed", true),
			}

			_, err := d.CreateVolume(context.Background(), &csi.CreateVolumeRequest{
				Name: "name",
				Parameters: map[string]string{
					LuksCipherAttribute:    "cipher",
					LuksEncryptedAttribute: "true",
					LuksKeyAttribute:       "key",
					LuksKeySizeAttribute:   "23234",
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					&csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
				VolumeContentSource: &csi.VolumeContentSource{
					Type: &csi.VolumeContentSource_Snapshot{
						Snapshot: &csi.VolumeContentSource_SnapshotSource{
							SnapshotId: "snapshotId",
						},
					},
				},
			})

			var wantErr error
			switch {
			case test.listVolumesErr != nil:
				wantErr = test.listVolumesErr
			case test.getSnapshotErr != nil:
				wantErr = test.getSnapshotErr
			}

			if wantErr == nil && err != nil {
				t.Errorf("got error %q, want none", err)
			}
			if wantErr != nil && !strings.Contains(err.Error(), wantErr.Error()) {
				t.Errorf("want error %q to include %q", err, wantErr)
			}
		})
	}
}
