/*
Copyright 2022 DigitalOcean

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
	"fmt"
	"net/http"
	"net/url"
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
	defaultVolume := &godo.Volume{
		ID:            "volume-id",
		SizeGigaBytes: (defaultVolumeSizeInBytes / giB),
	}
	oversizeErr := &godo.ErrorResponse{
		Response: &http.Response{
			Request: &http.Request{
				Method: http.MethodPost,
				URL:    &url.URL{},
			},
			StatusCode: http.StatusUnprocessableEntity,
		},
		Message: "error resizing volume: invalid size specified",
	}
	tcs := []struct {
		name              string
		req               *csi.ControllerExpandVolumeRequest
		resp              *csi.ControllerExpandVolumeResponse
		err               error
		volume            *godo.Volume
		resizeErr         error
		resizeErrResponse *godo.Response
	}{
		{
			name: "request exceeds maximum supported size",
			req: &csi.ControllerExpandVolumeRequest{
				VolumeId: "volume-id",
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 20 * tiB,
				},
			},
			resp:      nil,
			resizeErr: oversizeErr,
			resizeErrResponse: &godo.Response{
				Response: &http.Response{
					StatusCode: http.StatusUnprocessableEntity,
				},
			},
			err: status.Errorf(codes.OutOfRange, "requested size exceeds maximum supported volume size: %v", oversizeErr),
		},
		{
			name: "requested size less than minimum supported size returns the default minimum volume size",
			volume: &godo.Volume{
				ID:            "volume-id",
				SizeGigaBytes: 1,
			},
			req: &csi.ControllerExpandVolumeRequest{
				VolumeId: "volume-id",
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 0.5 * giB,
				},
			},
			resp: &csi.ControllerExpandVolumeResponse{CapacityBytes: minimumVolumeSizeInBytes, NodeExpansionRequired: true},
			err:  nil,
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
			if tc.volume == nil {
				tc.volume = defaultVolume
			}
			driver := &Driver{
				region: "foo",
				storage: &fakeStorageDriver{
					volumes: map[string]*godo.Volume{
						"volume-id": tc.volume,
					},
				},
				storageActions: &fakeStorageActionsDriver{
					volumes: map[string]*godo.Volume{
						"volume-id": tc.volume,
					},
					resizeErr:         tc.resizeErr,
					resizeErrResponse: tc.resizeErrResponse,
				},
				log: logrus.New().WithField("test_enabled", true),
			}
			resp, err := driver.ControllerExpandVolume(context.Background(), tc.req)
			if err != nil {
				assert.Equal(t, err, tc.err)
			} else {
				assert.Equal(t, tc.resp, resp)
				assert.Equal(t, (tc.volume.SizeGigaBytes * giB), resp.CapacityBytes)
			}

		})
	}
}

func TestCreateVolume(t *testing.T) {
	snapshotId := "snapshotId"

	tests := []struct {
		name                    string
		listVolumesErr          error
		getSnapshotErr          error
		snapshots               map[string]*godo.Snapshot
		wantErr                 error
		createVolumeErr         error
		createVolumeResponseErr *godo.Response
	}{
		{
			name:           "listing volumes failing",
			listVolumesErr: errors.New("failed to list volumes"),
		},
		{
			name:           "fetching snapshot failing",
			getSnapshotErr: errors.New("failed to get snapshot"),
		},
		{
			name: "volume limit has been reached",
			snapshots: map[string]*godo.Snapshot{
				snapshotId: {
					ID: snapshotId,
				},
			},
			createVolumeErr: &godo.ErrorResponse{
				Response: &http.Response{
					Request: &http.Request{
						Method: http.MethodPost,
						URL:    &url.URL{},
					},
					StatusCode: http.StatusForbidden,
				},
				Message: "failed to create volume: volume/snapshot capacity limit exceeded",
			},
			createVolumeResponseErr: &godo.Response{
				Response: &http.Response{
					StatusCode: http.StatusForbidden,
				},
			},
			wantErr: errors.New("volume limit has been reached. Please contact support"),
		},
		{
			name: "volume limit has been reached (429)",
			snapshots: map[string]*godo.Snapshot{
				snapshotId: {
					ID: snapshotId,
				},
			},
			createVolumeErr: &godo.ErrorResponse{
				Response: &http.Response{
					Request: &http.Request{
						Method: http.MethodPost,
						URL:    &url.URL{},
					},
					StatusCode: http.StatusTooManyRequests,
				},
				Message: "failed to create volume: volume/snapshot capacity limit exceeded",
			},
			createVolumeResponseErr: &godo.Response{
				Response: &http.Response{
					StatusCode: http.StatusTooManyRequests,
				},
			},
			wantErr: errors.New("volume limit has been reached. Please contact support"),
		},
		{
			name: "requested size exceeds maximum supported size",
			snapshots: map[string]*godo.Snapshot{
				snapshotId: {
					ID: snapshotId,
				},
			},
			createVolumeErr: &godo.ErrorResponse{
				Response: &http.Response{
					Request: &http.Request{
						Method: http.MethodPost,
						URL:    &url.URL{},
					},
					StatusCode: http.StatusUnprocessableEntity,
				},
				Message: "failed to create volume: invalid size specified",
			},
			createVolumeResponseErr: &godo.Response{
				Response: &http.Response{
					StatusCode: http.StatusUnprocessableEntity,
				},
			},
			wantErr: errors.New("requested size exceeds maximum supported volume size"),
		},
		{
			name: "error occurred when creating a volume",
			snapshots: map[string]*godo.Snapshot{
				snapshotId: {
					ID: snapshotId,
				},
			},
			createVolumeErr: &godo.ErrorResponse{
				Response: &http.Response{
					Request: &http.Request{
						Method: http.MethodPost,
						URL:    &url.URL{},
					},
					StatusCode: http.StatusInternalServerError,
				},
				Message: "internal server error",
			},
			createVolumeResponseErr: &godo.Response{
				Response: &http.Response{
					StatusCode: http.StatusInternalServerError,
				},
			},
			wantErr: errors.New("internal server error"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := &Driver{
				storage: &fakeStorageDriver{
					listVolumesErr:          test.listVolumesErr,
					createVolumeErr:         test.createVolumeErr,
					createVolumeErrResponse: test.createVolumeResponseErr,
				},
				snapshots: &fakeSnapshotsDriver{
					getSnapshotErr: test.getSnapshotErr,
					snapshots:      test.snapshots,
				},
				log: logrus.New().WithField("test_enabled", true),
			}

			_, err := d.CreateVolume(context.Background(), &csi.CreateVolumeRequest{
				Name: "name",
				VolumeCapabilities: []*csi.VolumeCapability{
					{
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
			case test.createVolumeErr != nil:
				wantErr = test.wantErr
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

func TestControllerPublishVolume(t *testing.T) {
	const (
		volumeID   = "vol-1"
		volumeName = "pvc-name"
		nodeID     = "42"
		dropletID  = 42
		publishKey = "test.csi.digitalocean.com/volume-name"
		doTag      = "k8s:cluster"
	)

	notFoundErr := errors.New("volume or droplet was not found")
	limitErr := errors.New(godo.ErrPerDropletVolumeCountLimit)
	legacyLimitErr := errors.New(maxVolumesPerDropletErrorLegacyMessage)
	regionErr := errors.New("volume and droplet are in different regions")
	suspendedErr := errors.New("volume is suspended")
	destroyedErr := errors.New("droplet has been destroyed")
	alreadyAttachedErr := errors.New(godo.ErrVolumeAlreadyAttached)
	pendingEventErr := errors.New(godo.ErrDropletPendingEvent)

	capability := &csi.VolumeCapability{
		AccessType: &csi.VolumeCapability_Mount{
			Mount: &csi.VolumeCapability_MountVolume{},
		},
		AccessMode: &csi.VolumeCapability_AccessMode{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
	}

	tests := []struct {
		name                 string
		attachVolumeDirectly bool
		missingVolume        bool
		missingDroplet       bool
		dropletIDs           []int
		doTag                string
		attachStatus         int
		attachErr            error
		actionStatus         string
		wantCode             codes.Code
		wantMessage          string
		wantRaw              error
		wantAttachCalls      int
		wantGetCalls         int
		wantTagCalls         int
	}{
		{
			name:                 "completed attach returns the volume name",
			attachVolumeDirectly: true,
			actionStatus:         godo.ActionCompleted,
			wantAttachCalls:      1,
			wantGetCalls:         1,
		},
		{
			name:                 "stale droplet id for this node still attaches",
			attachVolumeDirectly: true,
			dropletIDs:           []int{dropletID},
			actionStatus:         godo.ActionCompleted,
			wantAttachCalls:      1,
			wantGetCalls:         1,
		},
		{
			name:                 "stale droplet id for another node still attaches",
			attachVolumeDirectly: true,
			dropletIDs:           []int{99},
			actionStatus:         godo.ActionCompleted,
			wantAttachCalls:      1,
			wantGetCalls:         1,
		},
		{
			name:                 "volume does not exist",
			attachVolumeDirectly: true,
			missingVolume:        true,
			wantCode:             codes.NotFound,
			wantMessage:          fmt.Sprintf("volume %q does not exist", volumeID),
		},
		{
			name:                 "attach not found",
			attachVolumeDirectly: true,
			attachStatus:         http.StatusNotFound,
			attachErr:            notFoundErr,
			wantCode:             codes.NotFound,
			wantMessage:          fmt.Sprintf("failed to attach volume %q to droplet %d: %s", volumeID, dropletID, notFoundErr),
			wantAttachCalls:      1,
		},
		{
			name:                 "per droplet volume limit",
			attachVolumeDirectly: true,
			attachStatus:         http.StatusUnprocessableEntity,
			attachErr:            limitErr,
			wantCode:             codes.ResourceExhausted,
			wantMessage:          limitErr.Error(),
			wantAttachCalls:      1,
		},
		{
			name:                 "legacy per droplet volume limit",
			attachVolumeDirectly: true,
			attachStatus:         http.StatusUnprocessableEntity,
			attachErr:            legacyLimitErr,
			wantCode:             codes.ResourceExhausted,
			wantMessage:          legacyLimitErr.Error(),
			wantAttachCalls:      1,
		},
		{
			name:                 "already attached message is not treated as success",
			attachVolumeDirectly: true,
			attachStatus:         http.StatusUnprocessableEntity,
			attachErr:            alreadyAttachedErr,
			wantRaw:              alreadyAttachedErr,
			wantAttachCalls:      1,
		},
		{
			name:                 "pending event message is not aborted",
			attachVolumeDirectly: true,
			attachStatus:         http.StatusUnprocessableEntity,
			attachErr:            pendingEventErr,
			wantRaw:              pendingEventErr,
			wantAttachCalls:      1,
		},
		{
			name:                 "volume in another region",
			attachVolumeDirectly: true,
			attachStatus:         http.StatusConflict,
			attachErr:            regionErr,
			wantCode:             codes.FailedPrecondition,
			wantMessage:          fmt.Sprintf("failed to attach volume %q to droplet %d: %s", volumeID, dropletID, regionErr),
			wantAttachCalls:      1,
		},
		{
			name:                 "volume suspended",
			attachVolumeDirectly: true,
			attachStatus:         http.StatusForbidden,
			attachErr:            suspendedErr,
			wantCode:             codes.FailedPrecondition,
			wantMessage:          fmt.Sprintf("failed to attach volume %q to droplet %d: %s", volumeID, dropletID, suspendedErr),
			wantAttachCalls:      1,
		},
		{
			name:                 "destroyed droplet passes through",
			attachVolumeDirectly: true,
			attachStatus:         http.StatusBadRequest,
			attachErr:            destroyedErr,
			wantRaw:              destroyedErr,
			wantAttachCalls:      1,
		},
		{
			name:                 "errored action is a failed precondition",
			attachVolumeDirectly: true,
			actionStatus:         actionStatusErrored,
			wantCode:             codes.FailedPrecondition,
			wantMessage:          fmt.Sprintf("volume %q may be attached to another droplet", volumeID),
			wantAttachCalls:      1,
			wantGetCalls:         1,
		},
		{
			name:                 "tags the volume before attaching",
			attachVolumeDirectly: true,
			doTag:                doTag,
			actionStatus:         godo.ActionCompleted,
			wantAttachCalls:      1,
			wantGetCalls:         1,
			wantTagCalls:         1,
		},
		{
			name:                 "legacy: completed attach returns the volume name",
			attachVolumeDirectly: false,
			actionStatus:         godo.ActionCompleted,
			wantAttachCalls:      1,
			wantGetCalls:         1,
		},
		{
			name:                 "legacy: already attached short-circuits without attach",
			attachVolumeDirectly: false,
			dropletIDs:           []int{dropletID},
			wantAttachCalls:      0,
		},
		{
			name:                 "legacy: attached to another droplet is a failed precondition",
			attachVolumeDirectly: false,
			dropletIDs:           []int{99},
			wantCode:             codes.FailedPrecondition,
			wantMessage:          fmt.Sprintf("volume %q is attached to the wrong droplet (%d), detach the volume to fix it", volumeID, 99),
		},
		{
			name:                 "legacy: missing droplet is not found",
			attachVolumeDirectly: false,
			missingDroplet:       true,
			wantCode:             codes.NotFound,
			wantMessage:          fmt.Sprintf("droplet %d does not exist", dropletID),
		},
		{
			name:                 "legacy: already attached message is treated as success",
			attachVolumeDirectly: false,
			attachStatus:         http.StatusUnprocessableEntity,
			attachErr:            alreadyAttachedErr,
			wantAttachCalls:      1,
		},
		{
			name:                 "legacy: pending event is aborted",
			attachVolumeDirectly: false,
			attachStatus:         http.StatusUnprocessableEntity,
			attachErr:            pendingEventErr,
			wantCode:             codes.Aborted,
			wantMessage:          fmt.Sprintf("cannot attach because droplet %d has pending action for volume %q", dropletID, volumeID),
			wantAttachCalls:      1,
		},
		{
			name:                 "legacy: per droplet volume limit",
			attachVolumeDirectly: false,
			attachStatus:         http.StatusUnprocessableEntity,
			attachErr:            limitErr,
			wantCode:             codes.ResourceExhausted,
			wantMessage:          limitErr.Error(),
			wantAttachCalls:      1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			volumes := map[string]*godo.Volume{}
			if !tc.missingVolume {
				volumes[volumeID] = &godo.Volume{
					ID:         volumeID,
					Name:       volumeName,
					DropletIDs: tc.dropletIDs,
				}
			}

			droplets := map[int]*godo.Droplet{}
			if !tc.missingDroplet {
				droplets[dropletID] = &godo.Droplet{ID: dropletID}
			}

			tags := &fakeTagsDriver{exists: true}
			actions := &stubPublishStorageActions{
				attachStatus: tc.attachStatus,
				attachErr:    tc.attachErr,
				actionStatus: tc.actionStatus,
			}
			if tc.doTag != "" {
				actions.onAttach = func() {
					if tags.tagResourcesCount == 0 {
						t.Errorf("Attach called before the volume was tagged")
					}
				}
			}

			d := &Driver{
				publishInfoVolumeName: publishKey,
				doTag:                 tc.doTag,
				attachVolumeDirectly:  tc.attachVolumeDirectly,
				storage:               &fakeStorageDriver{volumes: volumes},
				storageActions:        actions,
				droplets:              &fakeDropletsDriver{droplets: droplets},
				tags:                  tags,
				log:                   logrus.New().WithField("test_enabled", true),
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			resp, err := d.ControllerPublishVolume(ctx, &csi.ControllerPublishVolumeRequest{
				VolumeId:         volumeID,
				NodeId:           nodeID,
				VolumeCapability: capability,
			})

			if actions.attachCalls != tc.wantAttachCalls {
				t.Errorf("attach calls: got %d, want %d", actions.attachCalls, tc.wantAttachCalls)
			}
			if actions.getCalls != tc.wantGetCalls {
				t.Errorf("action get calls: got %d, want %d", actions.getCalls, tc.wantGetCalls)
			}
			if tags.tagResourcesCount != tc.wantTagCalls {
				t.Errorf("tag calls: got %d, want %d", tags.tagResourcesCount, tc.wantTagCalls)
			}

			if tc.wantRaw != nil {
				if err != tc.wantRaw {
					t.Fatalf("got error %v, want %v", err, tc.wantRaw)
				}
				return
			}
			if tc.wantCode != codes.OK {
				st, ok := status.FromError(err)
				if !ok {
					t.Fatalf("got error %v, want status %s", err, tc.wantCode)
				}
				if st.Code() != tc.wantCode || st.Message() != tc.wantMessage {
					t.Errorf("got code %s message %q, want code %s message %q", st.Code(), st.Message(), tc.wantCode, tc.wantMessage)
				}
				return
			}
			if err != nil {
				t.Fatalf("got error %v, want success", err)
			}
			gotName := resp.GetPublishContext()[publishKey]
			if gotName != volumeName {
				t.Errorf("publish context volume name: got %q, want %q", gotName, volumeName)
			}
		})
	}
}

// stubPublishStorageActions returns scripted attach and action-get results.
type stubPublishStorageActions struct {
	attachStatus int
	attachErr    error
	actionStatus string
	onAttach     func()
	attachCalls  int
	getCalls     int
}

func (s *stubPublishStorageActions) Attach(ctx context.Context, volumeID string, dropletID int) (*godo.Action, *godo.Response, error) {
	s.attachCalls++
	if s.onAttach != nil {
		s.onAttach()
	}
	if s.attachErr != nil {
		return nil, &godo.Response{Response: &http.Response{StatusCode: s.attachStatus}}, s.attachErr
	}
	return &godo.Action{ID: 7, Status: godo.ActionInProgress}, godoResponse(), nil
}

func (s *stubPublishStorageActions) Get(ctx context.Context, volumeID string, actionID int) (*godo.Action, *godo.Response, error) {
	s.getCalls++
	actionStatus := s.actionStatus
	if actionStatus == "" {
		actionStatus = godo.ActionCompleted
	}
	return &godo.Action{ID: actionID, Status: actionStatus}, godoResponse(), nil
}

func (s *stubPublishStorageActions) DetachByDropletID(context.Context, string, int) (*godo.Action, *godo.Response, error) {
	panic("not implemented")
}

func (s *stubPublishStorageActions) List(context.Context, string, *godo.ListOptions) ([]godo.Action, *godo.Response, error) {
	panic("not implemented")
}

func (s *stubPublishStorageActions) Resize(context.Context, string, int, string) (*godo.Action, *godo.Response, error) {
	panic("not implemented")
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
			name: "errored",
			storageGetValsFunc: func(invocation int) (*godo.Action, *godo.Response, error) {
				if invocation != 0 {
					return nil, nil, fmt.Errorf("polled %d times after errored status", invocation+1)
				}
				return &godo.Action{
					Status: actionStatusErrored,
				}, &godo.Response{}, nil
			},
			timeout: 5 * time.Second,
			wantErr: errActionErrored,
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
				storageActions: &fakeStorageAction{
					fakeStorageActionsDriver: &fakeStorageActionsDriver{},
					storageGetValsFunc:       test.storageGetValsFunc,
				},
				log: logrus.New().WithField("test_enabled", true),
			}

			ctx, _ := context.WithTimeout(context.Background(), test.timeout)
			err := d.waitAction(
				ctx,
				logrus.New().WithField("test_enabled", true),
				"volumeID",
				42,
			)
			if err != test.wantErr {
				t.Errorf("got error %q, want %q", err, test.wantErr)
			}
		})
	}
}

func TestListSnapshot(t *testing.T) {
	createID := func(id int) string {
		return fmt.Sprintf("%03d", id)
	}

	tests := []struct {
		name             string
		inNumSnapshots   int
		maxEntries       int32
		startingToken    int
		wantNumSnapshots int
		wantNextToken    int
	}{
		{
			name:             "no constraints",
			inNumSnapshots:   10,
			wantNumSnapshots: 10,
		},
		{
			name:             "max entries set",
			inNumSnapshots:   10,
			maxEntries:       5,
			wantNumSnapshots: 5,
			wantNextToken:    6,
		},
		{
			name:             "starting token lower than number of snapshots",
			inNumSnapshots:   10,
			startingToken:    8,
			wantNumSnapshots: 3,
		},
		{
			name:             "starting token larger than number of snapshots",
			inNumSnapshots:   10,
			startingToken:    50,
			wantNumSnapshots: 0,
		},
		{
			name:             "starting token and max entries set with extra snapshots available",
			inNumSnapshots:   10,
			maxEntries:       5,
			startingToken:    4,
			wantNumSnapshots: 5,
			wantNextToken:    9,
		},
		{
			name:             "starting token and max entries set with no extra snapshots available",
			inNumSnapshots:   10,
			maxEntries:       15,
			startingToken:    8,
			wantNumSnapshots: 3,
		},
		{
			name:             "single paging with extra snapshots available",
			inNumSnapshots:   50,
			maxEntries:       12,
			startingToken:    30,
			wantNumSnapshots: 12,
			wantNextToken:    42,
		},
		{
			name:             "single paging with no extra snapshots available",
			inNumSnapshots:   32,
			maxEntries:       12,
			startingToken:    30,
			wantNumSnapshots: 3,
		},
		{
			name:             "multi-paging with extra snapshots available",
			inNumSnapshots:   50,
			maxEntries:       30,
			startingToken:    12,
			wantNumSnapshots: 30,
			wantNextToken:    42,
		},
		{
			name:             "multi-paging with exact fit",
			inNumSnapshots:   42,
			maxEntries:       30,
			startingToken:    13,
			wantNumSnapshots: 30,
		},
		{
			name:             "maxEntries exceeding maximum page size limit",
			inNumSnapshots:   300,
			maxEntries:       250,
			wantNumSnapshots: 250,
			wantNextToken:    251,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshots := map[string]*godo.Snapshot{}
			for i := 1; i <= test.inNumSnapshots; i++ {
				id := createID(i)
				snap := createGodoSnapshot(id, fmt.Sprintf("snapshot-%d", i), "")
				snapshots[id] = snap
			}

			d := Driver{
				snapshots: &fakeSnapshotsDriver{
					snapshots: snapshots,
				},
				log: logrus.New().WithField("test_enabled", true),
			}

			resp, err := d.ListSnapshots(context.Background(), &csi.ListSnapshotsRequest{
				MaxEntries:    test.maxEntries,
				StartingToken: strconv.Itoa(test.startingToken),
			})
			if err != nil {
				t.Fatalf("got error: %s", err)
			}

			if len(resp.Entries) != test.wantNumSnapshots {
				t.Errorf("got %d snapshot(s), want %d", len(resp.Entries), test.wantNumSnapshots)
			} else {
				runningID := test.startingToken
				if runningID == 0 {
					runningID = 1
				}
				for i, entry := range resp.Entries {
					wantID := createID(runningID)
					gotID := entry.Snapshot.GetSnapshotId()
					if gotID != wantID {
						t.Errorf("got snapshot ID %q at position %d, want %q", gotID, i, wantID)
					}
					runningID++
				}
			}

			if test.wantNextToken > 0 {
				wantNextTokenStr := strconv.Itoa(test.wantNextToken)
				if resp.NextToken != wantNextTokenStr {
					t.Errorf("got next token %q, want %q", resp.NextToken, wantNextTokenStr)
				}
			} else if resp.NextToken != "" {
				t.Errorf("got non-empty next token %q", resp.NextToken)
			}
		})
	}
}
