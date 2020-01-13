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
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/digitalocean/godo"
	"github.com/google/go-cmp/cmp"
	"github.com/magiconair/properties/assert"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"k8s.io/apimachinery/pkg/util/wait"
)

func TestTagger(t *testing.T) {
	tag := "k8s:my-cluster-id"
	tcs := []struct {
		name               string
		vol                *godo.Volume
		createTagFunc      func(ctx context.Context, req *godo.TagCreateRequest) (*godo.Tag, *godo.Response, error)
		tagResourcesFunc   func(context.Context, string, *godo.TagResourcesRequest) (*godo.Response, error)
		tagExists          bool
		expectCreates      int
		expectTagResources int
		expectError        bool
		expectTags         int
	}{
		{
			name:               "success existing tag",
			vol:                &godo.Volume{ID: "hello-world"},
			expectTagResources: 1,
			tagExists:          true,
			expectTags:         1,
		},
		{
			name:               "success with new tag",
			vol:                &godo.Volume{ID: "hello-world"},
			expectCreates:      1,
			expectTagResources: 2,
			expectTags:         1,
		},
		{
			name: "success already tagged",
			vol: &godo.Volume{
				ID:   "hello-world",
				Tags: []string{tag},
			},
			expectCreates:      0,
			expectTagResources: 0,
		},
		{
			name:               "failed first tag",
			vol:                &godo.Volume{ID: "hello-world"},
			expectCreates:      0,
			expectTagResources: 1,
			expectError:        true,
			tagResourcesFunc: func(context.Context, string, *godo.TagResourcesRequest) (*godo.Response, error) {
				return nil, errors.New("an error")
			},
		},
		{
			name:               "failed create tag",
			vol:                &godo.Volume{ID: "hello-world"},
			expectCreates:      1,
			expectTagResources: 1,
			expectError:        true,
			createTagFunc: func(ctx context.Context, req *godo.TagCreateRequest) (*godo.Tag, *godo.Response, error) {
				return nil, nil, errors.New("an error")
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {

			tagService := &fakeTagsDriver{
				createFunc:       tc.createTagFunc,
				tagResourcesFunc: tc.tagResourcesFunc,
				exists:           tc.tagExists,
			}
			driver := &Driver{
				doTag: tag,
				tags:  tagService,
			}

			err := driver.tagVolume(context.Background(), tc.vol)

			if err != nil && !tc.expectError {
				t.Errorf("expected success but got error %v", err)
			} else if tc.expectError && err == nil {
				t.Error("expected error but got success")
			}

			if tagService.createCount != tc.expectCreates {
				t.Errorf("createCount was %d, expected %d", tagService.createCount, tc.expectCreates)
			}
			if tagService.tagResourcesCount != tc.expectTagResources {
				t.Errorf("tagResourcesCount was %d, expected %d", tagService.tagResourcesCount, tc.expectTagResources)
			}
			if tc.expectTags != len(tagService.resources) {
				t.Errorf("expected %d tagged volume, %d found", tc.expectTags, len(tagService.resources))
			}
		})
	}
}

type fakeTagsDriver struct {
	createFunc        func(ctx context.Context, req *godo.TagCreateRequest) (*godo.Tag, *godo.Response, error)
	tagResourcesFunc  func(context.Context, string, *godo.TagResourcesRequest) (*godo.Response, error)
	exists            bool
	resources         []godo.Resource
	createCount       int
	tagResourcesCount int
}

func (*fakeTagsDriver) List(context.Context, *godo.ListOptions) ([]godo.Tag, *godo.Response, error) {
	panic("not implemented")
}

func (*fakeTagsDriver) Get(context.Context, string) (*godo.Tag, *godo.Response, error) {
	panic("not implemented")
}

func (f *fakeTagsDriver) Create(ctx context.Context, req *godo.TagCreateRequest) (*godo.Tag, *godo.Response, error) {
	f.createCount++
	if f.createFunc != nil {
		return f.createFunc(ctx, req)
	}
	f.exists = true
	return &godo.Tag{
		Name: req.Name,
	}, godoResponse(), nil
}

func (*fakeTagsDriver) Delete(context.Context, string) (*godo.Response, error) {
	panic("not implemented")
}

func (f *fakeTagsDriver) TagResources(ctx context.Context, tag string, req *godo.TagResourcesRequest) (*godo.Response, error) {
	f.tagResourcesCount++
	if f.tagResourcesFunc != nil {
		return f.tagResourcesFunc(ctx, tag, req)
	}
	if !f.exists {
		return &godo.Response{
			Response: &http.Response{StatusCode: 404},
			Rate:     godo.Rate{Limit: 10, Remaining: 10},
		}, errors.New("An error occured")
	}
	f.resources = append(f.resources, req.Resources...)
	return godoResponse(), nil
}

func (*fakeTagsDriver) UntagResources(context.Context, string, *godo.UntagResourcesRequest) (*godo.Response, error) {
	panic("not implemented")
}

func TestControllerExpandVolume(t *testing.T) {
	tcs := []struct {
		name string
		req  *csi.ControllerExpandVolumeRequest
		resp *csi.ControllerExpandVolumeResponse
		err  error
	}{
		{
			name: "request exceeds maximum supported size",
			req: &csi.ControllerExpandVolumeRequest{
				VolumeId: "volume-id",
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 20 * tiB,
				},
			},
			resp: nil,
			err:  status.Error(codes.OutOfRange, "ControllerExpandVolume invalid capacity range: required (20Ti) can not exceed maximum supported volume size (16Ti)"),
		},
		{
			name: "requested size less than minimum supported size",
			req: &csi.ControllerExpandVolumeRequest{
				VolumeId: "volume-id",
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 0.5 * giB,
				},
			},
			resp: nil,
			err:  status.Error(codes.OutOfRange, "ControllerExpandVolume invalid capacity range: required (512Mi) can not be less than minimum supported volume size (1Gi)"),
		},
		{
			name: "volume for corresponding volume id does not exist",
			req: &csi.ControllerExpandVolumeRequest{
				VolumeId: "non-existent-id",
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 20 * tiB,
				},
			},
			resp: nil,
			err:  status.Error(codes.Internal, "ControllerExpandVolume could not retrieve existing volume: volume not found"),
		},
		{
			name: "new volume size is less than old volume size",
			req: &csi.ControllerExpandVolumeRequest{
				VolumeId: "volume-id",
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 10 * giB,
				},
			},
			resp: &csi.ControllerExpandVolumeResponse{CapacityBytes: defaultVolumeSizeInBytes, NodeExpansionRequired: true},
			err:  nil,
		},
		{
			name: "new volume size is equal to old volume size",
			req: &csi.ControllerExpandVolumeRequest{
				VolumeId: "volume-id",
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 16 * giB,
				},
			},
			resp: &csi.ControllerExpandVolumeResponse{CapacityBytes: defaultVolumeSizeInBytes, NodeExpansionRequired: true},
			err:  nil,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			volume := &godo.Volume{
				ID:            "volume-id",
				SizeGigaBytes: (defaultVolumeSizeInBytes / giB),
			}
			driver := &Driver{
				region: "foo",
				storage: &fakeStorageDriver{
					volumes: map[string]*godo.Volume{
						"volume-id": volume,
					},
				},
				storageActions: &fakeStorageActionsDriver{
					volumes: map[string]*godo.Volume{
						"volume-id": volume,
					},
				},
				log: logrus.New().WithField("test_enabed", true),
			}
			resp, err := driver.ControllerExpandVolume(context.Background(), tc.req)
			if err != nil {
				assert.Equal(t, err, tc.err)
			} else {
				assert.Equal(t, tc.resp, resp)
				assert.Equal(t, (volume.SizeGigaBytes * giB), resp.CapacityBytes)
			}

		})
	}
}

func TestCreateVolume(t *testing.T) {
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

func TestCheckLimit(t *testing.T) {
	tests := []struct {
		name        string
		limit       int
		numVolumes  int
		wantErr     error
		wantDetails *limitDetails
	}{
		{
			name:       "limit insufficient",
			limit:      25,
			numVolumes: 30,
			wantDetails: &limitDetails{
				limit:      25,
				numVolumes: 30,
			},
		},
		{
			name:        "limit sufficient",
			limit:       100,
			numVolumes:  25,
			wantDetails: nil,
		},
		{
			name:        "administrative account",
			limit:       0,
			numVolumes:  1000,
			wantDetails: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			storage := &fakeStorageDriver{
				volumes: map[string]*godo.Volume{},
			}
			for i := 0; i < test.numVolumes; i++ {
				storage.volumes[strconv.Itoa(i)] = &godo.Volume{}
			}

			d := Driver{
				account: &fakeAccountDriver{
					volumeLimit: test.limit,
				},
				storage: storage,
			}

			gotDetails, err := d.checkLimit(context.Background())
			if err != nil {
				t.Fatalf("got error: %s", err)
			}

			if diff := cmp.Diff(gotDetails, test.wantDetails, cmp.AllowUnexported(limitDetails{})); diff != "" {
				t.Errorf("details mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

type fakeStorageAction struct {
	*fakeStorageActionsDriver
	storageGetValsFunc func(invocation int) (*godo.Action, *godo.Response, error)
	invocation         int
}

func (f *fakeStorageAction) Get(ctx context.Context, volumeID string, actionID int) (*godo.Action, *godo.Response, error) {
	defer func() {
		f.invocation++
	}()
	return f.storageGetValsFunc(f.invocation)
}

func TestWaitAction(t *testing.T) {
	tests := []struct {
		name               string
		storageGetValsFunc func(invocation int) (*godo.Action, *godo.Response, error)
		timeout            time.Duration
		wantErr            error
	}{
		{
			name: "timeout",
			storageGetValsFunc: func(int) (*godo.Action, *godo.Response, error) {
				return &godo.Action{
					Status: godo.ActionInProgress,
				}, nil, nil
			},
			timeout: 2 * time.Second,
			wantErr: wait.ErrWaitTimeout,
		},
		{
			name: "progressing to completion",
			storageGetValsFunc: func(invocation int) (*godo.Action, *godo.Response, error) {
				switch invocation {
				case 0:
					return nil, nil, errors.New("network disruption")
				case 1:
					return &godo.Action{
						Status: godo.ActionInProgress,
					}, &godo.Response{}, nil
				default:
					return &godo.Action{
						Status: godo.ActionCompleted,
					}, &godo.Response{}, nil
				}
			},
			timeout: 5 * time.Second, // We need three 1-second ticks for the fake storage action to complete.
			wantErr: nil,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			d := Driver{
				waitActionTimeout: test.timeout,
				storageActions: &fakeStorageAction{
					fakeStorageActionsDriver: &fakeStorageActionsDriver{},
					storageGetValsFunc:       test.storageGetValsFunc,
				},
				log: logrus.New().WithField("test_enabed", true),
			}

			err := d.waitAction(
				context.Background(),
				logrus.New().WithField("test_enabed", true),
				"volumeID",
				42,
			)
			if err != test.wantErr {
				t.Errorf("got error %q, want %q", err, test.wantErr)
			}
		})
	}
}
