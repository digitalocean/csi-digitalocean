/*
Copyright 2026 DigitalOcean

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gotest.tools/v3/assert"
	"k8s.io/mount-utils"
)

// stageMounter stubs the Mounter calls NodeStageVolume makes and records the
// device each format-related call received, so the gRPC error mapping and the
// use of the resolved device can be tested without a block device.
type stageMounter struct {
	device         string
	waitErr        error
	attachErr      error
	formatted      bool
	isFormattedErr error
	formatErr      error

	waitPath          string
	attachedSource    string
	isFormattedSource string
	formatSource      string
	mountSource       string
}

func (m *stageMounter) WaitForDevice(_ context.Context, path string, _ time.Duration) (string, error) {
	m.waitPath = path
	if m.waitErr != nil {
		return "", m.waitErr
	}
	return m.device, nil
}

func (m *stageMounter) IsAttached(source string) error {
	m.attachedSource = source
	return m.attachErr
}

func (m *stageMounter) IsFormatted(source string) (bool, error) {
	m.isFormattedSource = source
	return m.formatted, m.isFormattedErr
}

func (m *stageMounter) Format(source, _ string) error {
	m.formatSource = source
	return m.formatErr
}

func (m *stageMounter) Mount(source, _, _ string, _ ...string) error {
	m.mountSource = source
	return nil
}

func (m *stageMounter) Unmount(string) error { return nil }

func (m *stageMounter) IsMounted(string) (bool, error) { return false, nil }

func (m *stageMounter) GetDeviceName(mount.Interface, string) (string, error) {
	return "", errors.New("not implemented")
}

func (m *stageMounter) GetStatistics(string) (volumeStatistics, error) {
	return volumeStatistics{}, errors.New("not implemented")
}

func (m *stageMounter) IsBlockDevice(string) (bool, error) {
	return false, errors.New("not implemented")
}

func TestNodeStageVolume(t *testing.T) {
	const (
		volumeName = "pvc-node-stage-test"
		byIDPath   = "/dev/disk/by-id/scsi-0DO_Volume_" + volumeName
		device     = "/dev/sdb"
	)

	tests := []struct {
		name               string
		mounter            *stageMounter
		validateAttachment bool
		volumeContext      map[string]string

		wantCode        codes.Code
		wantErr         string
		wantAttached    bool
		wantIsFormatted bool
		wantFormat      bool
		wantMount       bool
	}{
		{
			name:     "device wait timeout returns Unavailable",
			mounter:  &stageMounter{waitErr: errors.New("device did not become ready")},
			wantCode: codes.Unavailable,
			wantErr:  "waiting for device",
		},
		{
			name:               "attachment check failure returns Unavailable",
			mounter:            &stageMounter{device: device, attachErr: errors.New("not running")},
			validateAttachment: true,
			wantCode:           codes.Unavailable,
			wantErr:            "attachment status",
			wantAttached:       true,
		},
		{
			name:            "blkid failure returns Internal without formatting",
			mounter:         &stageMounter{device: device, isFormattedErr: errors.New("blkid exited with code 4")},
			wantCode:        codes.Internal,
			wantErr:         "could not determine whether volume",
			wantIsFormatted: true,
		},
		{
			name:            "non-empty device returns FailedPrecondition",
			mounter:         &stageMounter{device: device, formatErr: errDeviceNotEmpty},
			wantCode:        codes.FailedPrecondition,
			wantErr:         noFormatVolumeAttributes[0] + "\" in the PersistentVolume spec.csi.volumeAttributes",
			wantIsFormatted: true,
			wantFormat:      true,
		},
		{
			name:            "other format failure returns Internal",
			mounter:         &stageMounter{device: device, formatErr: errors.New("mkfs failed")},
			wantCode:        codes.Internal,
			wantErr:         "mkfs failed",
			wantIsFormatted: true,
			wantFormat:      true,
		},
		{
			name:               "blank device is formatted and mounted",
			mounter:            &stageMounter{device: device},
			validateAttachment: true,
			wantCode:           codes.OK,
			wantAttached:       true,
			wantIsFormatted:    true,
			wantFormat:         true,
			wantMount:          true,
		},
		{
			name:            "formatted device is mounted without formatting",
			mounter:         &stageMounter{device: device, formatted: true},
			wantCode:        codes.OK,
			wantIsFormatted: true,
			wantMount:       true,
		},
		{
			name:          "noformat volume attribute skips format checks",
			mounter:       &stageMounter{device: device},
			volumeContext: map[string]string{noFormatVolumeAttributes[0]: "true"},
			wantCode:      codes.OK,
			wantMount:     true,
		},
		{
			name:          "legacy noformat volume attribute skips format checks",
			mounter:       &stageMounter{device: device},
			volumeContext: map[string]string{noFormatVolumeAttributes[1]: "true"},
			wantCode:      codes.OK,
			wantMount:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Driver{
				name:                  DefaultDriverName,
				publishInfoVolumeName: DefaultDriverName + "/volume-name",
				log:                   logrus.New().WithField("test_enabled", true),
				mounter:               tt.mounter,
				validateAttachment:    tt.validateAttachment,
				deviceWaitTimeout:     time.Second,
			}
			req := &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-1",
				StagingTargetPath: t.TempDir(),
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{FsType: "ext4"},
					},
				},
				PublishContext: map[string]string{d.publishInfoVolumeName: volumeName},
				VolumeContext:  tt.volumeContext,
			}

			_, err := d.NodeStageVolume(context.Background(), req)

			assert.Equal(t, status.Code(err), tt.wantCode, "err: %v", err)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			}

			m := tt.mounter
			assert.Equal(t, m.waitPath, byIDPath)
			// Every format-related call must use the device WaitForDevice
			// resolved, not the by-id symlink.
			assert.Equal(t, m.attachedSource, ifWant(tt.wantAttached, device))
			assert.Equal(t, m.isFormattedSource, ifWant(tt.wantIsFormatted, device))
			assert.Equal(t, m.formatSource, ifWant(tt.wantFormat, device))
			assert.Equal(t, m.mountSource, ifWant(tt.wantMount, byIDPath))
		})
	}
}

func ifWant(want bool, s string) string {
	if want {
		return s
	}
	return ""
}
