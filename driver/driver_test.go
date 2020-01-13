/*
Copyright 2020 DigitalOcean

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
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/kubernetes-csi/csi-test/pkg/sanity"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestDriverSuite(t *testing.T) {
	socket := "/tmp/csi.sock"
	endpoint := "unix://" + socket
	if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
		t.Fatalf("failed to remove unix domain socket file %s, error: %s", socket, err)
	}

	nodeID := 987654
	doTag := "k8s:cluster-id"
	volumes := make(map[string]*godo.Volume)
	snapshots := make(map[string]*godo.Snapshot)
	droplets := map[int]*godo.Droplet{
		nodeID: {
			ID: nodeID,
		},
	}

	driver := &Driver{
		name:              DefaultDriverName,
		endpoint:          endpoint,
		hostID:            strconv.Itoa(nodeID),
		doTag:             doTag,
		region:            "nyc3",
		waitActionTimeout: defaultWaitActionTimeout,
		mounter: &fakeMounter{
			mounted: map[string]string{},
		},
		log: logrus.New().WithField("test_enabed", true),

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
		tags:    &fakeTagsDriver{},
	}

	ctx, cancel := context.WithCancel(context.Background())

	var eg errgroup.Group
	eg.Go(func() error {
		return driver.Run(ctx)
	})

	cfg := &sanity.Config{
		TargetPath:  os.TempDir() + "/csi-target",
		StagingPath: os.TempDir() + "/csi-staging",
		Address:     endpoint,
	}
	sanity.Test(t, cfg)

	cancel()
	if err := eg.Wait(); err != nil {
		t.Errorf("driver run failed: %s", err)
	}
}

type fakeAccountDriver struct {
	volumeLimit int
}

func (f *fakeAccountDriver) Get(context.Context) (*godo.Account, *godo.Response, error) {
	return &godo.Account{
		VolumeLimit: f.volumeLimit,
	}, godoResponse(), nil
}

type fakeStorageDriver struct {
	volumes        map[string]*godo.Volume
	snapshots      map[string]*godo.Snapshot
	listVolumesErr error
}

func (f *fakeStorageDriver) ListVolumes(ctx context.Context, param *godo.ListVolumeParams) ([]godo.Volume, *godo.Response, error) {
	if f.listVolumesErr != nil {
		return nil, nil, f.listVolumesErr
	}

	var volumes []godo.Volume

	for _, vol := range f.volumes {
		volumes = append(volumes, *vol)
	}

	if param != nil && param.ListOptions != nil && param.ListOptions.PerPage != 0 {
		perPage := param.ListOptions.PerPage
		chunkSize := perPage
		if len(volumes) < perPage {
			chunkSize = len(volumes)
		}
		vols := volumes[:chunkSize]
		for _, vol := range vols {
			delete(f.volumes, vol.ID)
		}

		return vols, godoResponseWithMeta(len(volumes)), nil
	}

	if param.Name != "" {
		var filtered []godo.Volume
		for _, vol := range volumes {
			if vol.Name == param.Name {
				filtered = append(filtered, vol)
			}
		}

		return filtered, godoResponseWithMeta(len(filtered)), nil
	}

	return volumes, godoResponseWithMeta(len(volumes)), nil
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

	return snapshots, godoResponseWithMeta(len(snapshots)), nil
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
	return nil, godoResponseWithMeta(0), nil
}

func (f *fakeStorageActionsDriver) Resize(ctx context.Context, volumeID string, sizeGigabytes int, regionSlug string) (*godo.Action, *godo.Response, error) {
	volume := f.volumes[volumeID]
	volume.SizeGigaBytes = int64(sizeGigabytes)
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
	snapshots      map[string]*godo.Snapshot
	getSnapshotErr error
}

func (f *fakeSnapshotsDriver) List(context.Context, *godo.ListOptions) ([]godo.Snapshot, *godo.Response, error) {
	panic("not implemented")
}

func (f *fakeSnapshotsDriver) ListVolume(ctx context.Context, opts *godo.ListOptions) ([]godo.Snapshot, *godo.Response, error) {
	var snapshots []godo.Snapshot
	for _, snap := range f.snapshots {
		snapshots = append(snapshots, *snap)
	}

	return snapshots, godoResponseWithMeta(len(snapshots)), nil
}

func (f *fakeSnapshotsDriver) ListDroplet(context.Context, *godo.ListOptions) ([]godo.Snapshot, *godo.Response, error) {
	panic("not implemented")
}

func (f *fakeSnapshotsDriver) Get(ctx context.Context, id string) (*godo.Snapshot, *godo.Response, error) {
	if f.getSnapshotErr != nil {
		return nil, nil, f.getSnapshotErr
	}

	resp := godoResponse()
	snap, ok := f.snapshots[id]
	if !ok {
		resp.Response = &http.Response{
			StatusCode: http.StatusNotFound,
		}
		return nil, resp, errors.New("snapshot not found")
	}

	return snap, resp, nil
}

func (f *fakeSnapshotsDriver) Delete(context.Context, string) (*godo.Response, error) {
	panic("not implemented")
}

type fakeMounter struct {
	mounted map[string]string
}

func (f *fakeMounter) Format(source string, fsType string) error {
	return nil
}

func (f *fakeMounter) Mount(source string, target string, fsType string, options ...string) error {
	f.mounted[target] = source
	return nil
}

func (f *fakeMounter) Unmount(target string) error {
	delete(f.mounted, target)
	return nil
}

func (f *fakeMounter) IsFormatted(source string) (bool, error) {
	return true, nil
}
func (f *fakeMounter) IsMounted(target string) (bool, error) {
	_, ok := f.mounted[target]
	return ok, nil
}

func (f *fakeMounter) GetStatistics(volumePath string) (volumeStatistics, error) {
	return volumeStatistics{
		availableBytes: 3 * giB,
		totalBytes:     10 * giB,
		usedBytes:      7 * giB,

		availableInodes: 3000,
		totalInodes:     10000,
		usedInodes:      7000,
	}, nil
}

func (f *fakeMounter) IsBlockDevice(volumePath string) (bool, error) {
	return false, nil
}

func godoResponse() *godo.Response {
	return godoResponseWithMeta(0)
}

func godoResponseWithMeta(total int) *godo.Response {
	return &godo.Response{
		Response: &http.Response{StatusCode: 200},
		Rate:     godo.Rate{Limit: 10, Remaining: 10},
		Meta: &godo.Meta{
			Total: total,
		},
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
