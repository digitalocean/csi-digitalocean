//go:build linux && loopdevice

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

// These tests exercise the real mounter against a loop device. They require
// root, losetup, blkid and mkfs.ext4. CI runs them via the "Run loop device
// tests" step in .github/workflows/test.yaml. Locally, e.g.:
//
//	docker run --rm --privileged -v "$PWD":/app -w /app golang:1.27-alpine sh -c \
//	  'apk add blkid e2fsprogs util-linux && go test -mod=vendor -tags loopdevice -run Loop -v ./driver/'

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func setupLoopDevice(t *testing.T) string {
	t.Helper()

	if os.Geteuid() != 0 {
		t.Fatal("loopdevice tests must run as root to attach loop devices")
	}

	img := filepath.Join(t.TempDir(), "vol.img")
	assert.NilError(t, exec.Command("truncate", "-s", "64M", img).Run())

	out, err := exec.Command("losetup", "-f", "--show", img).CombinedOutput()
	assert.NilError(t, err, "losetup: %s", out)
	dev := strings.TrimSpace(string(out))
	t.Cleanup(func() { _ = exec.Command("losetup", "-d", dev).Run() })
	return dev
}

func fsUUID(t *testing.T, dev string) string {
	t.Helper()
	out, err := exec.Command("blkid", "-p", "-s", "UUID", "-o", "value", dev).Output()
	assert.NilError(t, err)
	return strings.TrimSpace(string(out))
}

func TestLoop_FormatOnlyBlankDevice(t *testing.T) {
	dev := setupLoopDevice(t)
	m := newMounter(logrus.NewEntry(logrus.New()))

	formatted, err := m.IsFormatted(dev)
	assert.NilError(t, err)
	assert.Equal(t, formatted, false, "blank device %s", dev)

	assert.NilError(t, m.Format(dev, "ext4"))

	formatted, err = m.IsFormatted(dev)
	assert.NilError(t, err)
	assert.Equal(t, formatted, true, "device %s after format", dev)

	// CON-14735: even if the formatted check is wrong, Format must not
	// overwrite an existing filesystem.
	before := fsUUID(t, dev)
	err = m.Format(dev, "ext4")
	assert.Assert(t, errors.Is(err, errDeviceNotEmpty), "got %v, want errDeviceNotEmpty", err)
	assert.Equal(t, fsUUID(t, dev), before, "filesystem UUID changed, device was reformatted")
}

func TestLoop_MissingByIDPath(t *testing.T) {
	m := newMounter(logrus.NewEntry(logrus.New()))
	missing := "/dev/disk/by-id/scsi-0DO_Volume_does-not-exist"

	formatted, err := m.IsFormatted(missing)
	assert.ErrorContains(t, err, "no such file or directory")
	assert.Equal(t, formatted, false)

	_, err = m.WaitForDevice(context.Background(), missing, 500*time.Millisecond)
	assert.ErrorContains(t, err, "did not become ready")
}

func TestLoop_WaitForDeviceResolvesSymlink(t *testing.T) {
	dev := setupLoopDevice(t)
	m := newMounter(logrus.NewEntry(logrus.New()))

	link := filepath.Join(t.TempDir(), "scsi-0DO_Volume_test")
	go func() {
		time.Sleep(time.Second)
		_ = os.Symlink(dev, link)
	}()

	got, err := m.WaitForDevice(context.Background(), link, 10*time.Second)
	assert.NilError(t, err)
	assert.Equal(t, got, dev)
}
