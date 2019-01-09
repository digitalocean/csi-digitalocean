/*
Copyright 2018 DigitalOcean

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
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/kubernetes-csi/csi-test/pkg/sanity"
	"github.com/sirupsen/logrus"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type storageVolumeRoot struct {
	Volume *godo.Volume `json:"volume"`
	Links  *godo.Links  `json:"links,omitempty"`
}

type storageVolumesRoot struct {
	Volumes []godo.Volume `json:"volumes"`
	Links   *godo.Links   `json:"links"`
}

type dropletRoot struct {
	Droplet *godo.Droplet `json:"droplet"`
	Links   *godo.Links   `json:"links,omitempty"`
}

func TestDriverSuite(t *testing.T) {
	socket := "/tmp/csi.sock"
	endpoint := "unix://" + socket
	if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
		t.Fatalf("failed to remove unix domain socket file %s, error: %s", socket, err)
	}

	nodeID := 987654
	volumes := make(map[string]*godo.Volume, 0)
	snapshots := make(map[string]*godo.Snapshot, 0)
	droplets := map[int]*godo.Droplet{
		nodeID: {
			ID: nodeID,
		},
	}

	driver := &Driver{
		endpoint: endpoint,
		nodeId:   strconv.Itoa(nodeID),
		region:   "nyc3",
		mounter:  &fakeMounter{},
		log:      logrus.New().WithField("test_enabed", true),

		storage: &fakeStorageDriver{
			volumes:   volumes,
			snapshots: snapshots,
		},
		storageActions: &fakeStorageActionsDriver{
			volumes:  volumes,
			droplets: droplets,
		},
		droplets: &fakeDropletsDriver{
			droplets: droplets,
		},
		snapshots: &fakeSnapshotsDriver{
			snapshots: snapshots,
		},
		account: &fakeAccountDriver{},
	}
	defer driver.Stop()

	go driver.Run()

	mntDir, err := ioutil.TempDir("", "mnt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(mntDir)

	mntStageDir, err := ioutil.TempDir("", "mnt-stage")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(mntStageDir)

	cfg := &sanity.Config{
		StagingPath: mntStageDir,
		TargetPath:  mntDir,
		Address:     endpoint,
	}

	sanity.Test(t, cfg)
}

type fakeAccountDriver struct{}

func (f *fakeAccountDriver) Get(context.Context) (*godo.Account, *godo.Response, error) {
	return &godo.Account{}, godoResponse(), nil
}

type fakeStorageDriver struct {
	volumes   map[string]*godo.Volume
	snapshots map[string]*godo.Snapshot
}

func (f *fakeStorageDriver) ListVolumes(ctx context.Context, param *godo.ListVolumeParams) ([]godo.Volume, *godo.Response, error) {
	var volumes []godo.Volume

	for _, vol := range f.volumes {
		volumes = append(volumes, *vol)
	}

	if param.Name != "" {
		var filtered []godo.Volume
		for _, vol := range volumes {
			if vol.Name == param.Name {
				filtered = append(filtered, vol)
			}
		}

		return filtered, godoResponse(), nil
	}

	return volumes, godoResponse(), nil
}

func (f *fakeStorageDriver) GetVolume(ctx context.Context, id string) (*godo.Volume, *godo.Response, error) {
	resp := godoResponse()
	vol, ok := f.volumes[id]
	if !ok {
		resp.Response = &http.Response{
			StatusCode: http.StatusNotFound,
		}
		return nil, resp, errors.New("volume not found")
	}

	return vol, resp, nil
}

func (f *fakeStorageDriver) CreateVolume(ctx context.Context, req *godo.VolumeCreateRequest) (*godo.Volume, *godo.Response, error) {
	id := randString(10)
	vol := &godo.Volume{
		ID:            id,
		Region:        &godo.Region{Slug: req.Region},
		Name:          req.Name,
		Description:   req.Description,
		SizeGigaBytes: req.SizeGigaBytes,
	}

	f.volumes[id] = vol

	return vol, godoResponse(), nil
}

func (f *fakeStorageDriver) DeleteVolume(ctx context.Context, id string) (*godo.Response, error) {
	delete(f.volumes, id)
	return godoResponse(), nil
}

func (f *fakeStorageDriver) ListSnapshots(ctx context.Context, volumeID string, opts *godo.ListOptions) ([]godo.Snapshot, *godo.Response, error) {
	var snapshots []godo.Snapshot

	for _, snap := range f.snapshots {
		if snap.ResourceID == volumeID {
			snapshots = append(snapshots, *snap)
		}
	}

	return snapshots, godoResponse(), nil
}

func (f *fakeStorageDriver) GetSnapshot(ctx context.Context, id string) (*godo.Snapshot, *godo.Response, error) {
	resp := godoResponse()
	snap, ok := f.snapshots[id]
	if !ok {
		resp.Response = &http.Response{
			StatusCode: http.StatusNotFound,
		}
		return nil, resp, errors.New("volume not found")
	}

	return snap, resp, nil
}

func (f *fakeStorageDriver) CreateSnapshot(crx context.Context, req *godo.SnapshotCreateRequest) (*godo.Snapshot, *godo.Response, error) {

	resp := godoResponse()
	for _, s := range f.snapshots {
		if s.Name == req.Name {
			resp.Response = &http.Response{
				StatusCode: http.StatusConflict,
			}
			return nil, resp, errors.New("snapshot with the same name exist")
		}
	}

	id := randString(10)
	snap := &godo.Snapshot{
		ID:         id,
		Name:       req.Name,
		ResourceID: req.VolumeID,
		Created:    time.Now().UTC().Format(time.RFC3339),
	}

	f.snapshots[id] = snap

	return snap, resp, nil
}

func (f *fakeStorageDriver) DeleteSnapshot(ctx context.Context, id string) (*godo.Response, error) {
	delete(f.snapshots, id)
	return godoResponse(), nil
}

type fakeStorageActionsDriver struct {
	volumes  map[string]*godo.Volume
	droplets map[int]*godo.Droplet
}

func (f *fakeStorageActionsDriver) Attach(ctx context.Context, volumeID string, dropletID int) (*godo.Action, *godo.Response, error) {
	return nil, godoResponse(), nil
}

func (f *fakeStorageActionsDriver) DetachByDropletID(ctx context.Context, volumeID string, dropletID int) (*godo.Action, *godo.Response, error) {
	return nil, godoResponse(), nil
}

func (f *fakeStorageActionsDriver) Get(ctx context.Context, volumeID string, actionID int) (*godo.Action, *godo.Response, error) {
	return nil, godoResponse(), nil
}

func (f *fakeStorageActionsDriver) List(ctx context.Context, volumeID string, opt *godo.ListOptions) ([]godo.Action, *godo.Response, error) {
	return nil, godoResponse(), nil
}

func (f *fakeStorageActionsDriver) Resize(ctx context.Context, volumeID string, sizeGigabytes int, regionSlug string) (*godo.Action, *godo.Response, error) {
	return nil, godoResponse(), nil
}

type fakeDropletsDriver struct {
	droplets map[int]*godo.Droplet
}

func (f *fakeDropletsDriver) List(context.Context, *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
	panic("not implemented")
}

func (f *fakeDropletsDriver) ListByTag(context.Context, string, *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
	panic("not implemented")
}

func (f *fakeDropletsDriver) Get(ctx context.Context, dropletID int) (*godo.Droplet, *godo.Response, error) {
	resp := godoResponse()
	droplet, ok := f.droplets[dropletID]
	if !ok {
		resp.Response = &http.Response{
			StatusCode: http.StatusNotFound,
		}
		return nil, resp, errors.New("droplet not found")
	}

	return droplet, godoResponse(), nil
}

func (f *fakeDropletsDriver) Create(context.Context, *godo.DropletCreateRequest) (*godo.Droplet, *godo.Response, error) {
	panic("not implemented")
}

func (f *fakeDropletsDriver) CreateMultiple(context.Context, *godo.DropletMultiCreateRequest) ([]godo.Droplet, *godo.Response, error) {
	panic("not implemented")
}

func (f *fakeDropletsDriver) Delete(context.Context, int) (*godo.Response, error) {
	panic("not implemented")
}

func (f *fakeDropletsDriver) DeleteByTag(context.Context, string) (*godo.Response, error) {
	panic("not implemented")
}

func (f *fakeDropletsDriver) Kernels(context.Context, int, *godo.ListOptions) ([]godo.Kernel, *godo.Response, error) {
	panic("not implemented")
}

func (f *fakeDropletsDriver) Snapshots(context.Context, int, *godo.ListOptions) ([]godo.Image, *godo.Response, error) {
	panic("not implemented")
}

func (f *fakeDropletsDriver) Backups(context.Context, int, *godo.ListOptions) ([]godo.Image, *godo.Response, error) {
	panic("not implemented")
}

func (f *fakeDropletsDriver) Actions(context.Context, int, *godo.ListOptions) ([]godo.Action, *godo.Response, error) {
	panic("not implemented")
}

func (f *fakeDropletsDriver) Neighbors(context.Context, int) ([]godo.Droplet, *godo.Response, error) {
	panic("not implemented")
}

type fakeSnapshotsDriver struct {
	snapshots map[string]*godo.Snapshot
}

func (f *fakeSnapshotsDriver) List(context.Context, *godo.ListOptions) ([]godo.Snapshot, *godo.Response, error) {
	panic("not implemented")
}

func (f *fakeSnapshotsDriver) ListVolume(ctx context.Context, opts *godo.ListOptions) ([]godo.Snapshot, *godo.Response, error) {
	var snapshots []godo.Snapshot
	for _, snap := range f.snapshots {
		snapshots = append(snapshots, *snap)
	}

	return snapshots, godoResponse(), nil
}

func (f *fakeSnapshotsDriver) ListDroplet(context.Context, *godo.ListOptions) ([]godo.Snapshot, *godo.Response, error) {
	panic("not implemented")
}

func (f *fakeSnapshotsDriver) Get(context.Context, string) (*godo.Snapshot, *godo.Response, error) {
	panic("not implemented")
}

func (f *fakeSnapshotsDriver) Delete(context.Context, string) (*godo.Response, error) {
	panic("not implemented")
}

type fakeMounter struct{}

func (f *fakeMounter) Format(source string, fsType string) error {
	return nil
}

func (f *fakeMounter) Mount(source string, target string, fsType string, options ...string) error {
	return nil
}

func (f *fakeMounter) Unmount(target string) error {
	return nil
}

func (f *fakeMounter) IsFormatted(source string) (bool, error) {
	return true, nil
}
func (f *fakeMounter) IsMounted(target string) (bool, error) {
	return true, nil
}

func godoResponse() *godo.Response {
	return &godo.Response{
		Response: &http.Response{StatusCode: 200},
		Rate:     godo.Rate{Limit: 10, Remaining: 10},
	}
}

func randString(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
