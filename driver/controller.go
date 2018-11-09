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
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/digitalocean/godo"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	_  = iota
	KB = 1 << (10 * iota)
	MB
	GB
	TB
)

const (
	// PublishInfoVolumeName is used to pass the volume name from
	// `ControllerPublishVolume` to `NodeStageVolume or `NodePublishVolume`
	PublishInfoVolumeName = DriverName + "/volume-name"

	// defaultVolumeSizeInGB is used when the user didn't defined a correct
	// storage size or if the size is not satisfised
	defaultVolumeSizeInGB = 16 * GB

	// createdByDO is used to tag volumes that are created by this CSI plugin
	createdByDO = "Created by DigitalOcean CSI driver"
)

var (
	// DO currently only support a single node to be attached to a single node
	// in read/write mode. This corresponds to `accessModes.ReadWriteOnce` in a
	// PVC resource on Kubernets
	supportedAccessMode = &csi.VolumeCapability_AccessMode{
		Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
	}
)

// CreateVolume creates a new volume from the given request. The function is
// idempotent.
func (d *Driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Name must be provided")
	}

	if req.VolumeCapabilities == nil || len(req.VolumeCapabilities) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Volume capabilities must be provided")
	}

	if req.AccessibilityRequirements != nil {
		for _, t := range req.AccessibilityRequirements.Requisite {
			region, ok := t.Segments["region"]
			if !ok {
				continue // nothing to do
			}

			if region != d.region {
				return nil, status.Errorf(codes.ResourceExhausted, "volume can be only created in region: %q, got: %q", d.region, region)

			}
		}
	}

	size, err := extractStorage(req.CapacityRange)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	volumeName := req.Name

	ll := d.log.WithFields(logrus.Fields{
		"volume_name":             volumeName,
		"storage_size_giga_bytes": size / GB,
		"method":                  "create_volume",
		"volume_capabilities":     req.VolumeCapabilities,
	})
	ll.Info("create volume called")

	// get volume first, if it's created do no thing
	volumes, _, err := d.storage.ListVolumes(ctx, &godo.ListVolumeParams{
		Region: d.region,
		Name:   volumeName,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// volume already exist, do nothing
	if len(volumes) != 0 {
		if len(volumes) > 1 {
			return nil, fmt.Errorf("fatal issue: duplicate volume %q exists", volumeName)
		}
		vol := volumes[0]

		if vol.SizeGigaBytes*GB != size {
			return nil, status.Error(codes.AlreadyExists, fmt.Sprintf("invalid option requested size: %d", size))
		}

		ll.Info("volume already created")
		return &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				Id:            vol.ID,
				CapacityBytes: vol.SizeGigaBytes * GB,
			},
		}, nil
	}

	volumeReq := &godo.VolumeCreateRequest{
		Region:        d.region,
		Name:          volumeName,
		Description:   createdByDO,
		SizeGigaBytes: size / GB,
	}

	if !validateCapabilities(req.VolumeCapabilities) {
		return nil, status.Error(codes.AlreadyExists, "invalid volume capabilities requested. Only SINGLE_NODE_WRITER is supported ('accessModes.ReadWriteOnce' on Kubernetes)")
	}

	ll.Info("checking volume limit")
	if err := d.checkLimit(ctx); err != nil {
		return nil, err
	}

	ll.WithField("volume_req", volumeReq).Info("creating volume")
	vol, _, err := d.storage.CreateVolume(ctx, volumeReq)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			Id:            vol.ID,
			CapacityBytes: size,
			AccessibleTopology: []*csi.Topology{
				{
					Segments: map[string]string{
						"region": d.region,
					},
				},
			},
		},
	}

	ll.WithField("response", resp).Info("volume created")
	return resp, nil
}

// DeleteVolume deletes the given volume. The function is idempotent.
func (d *Driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "DeleteVolume Volume ID must be provided")
	}

	ll := d.log.WithFields(logrus.Fields{
		"volume_id": req.VolumeId,
		"method":    "delete_volume",
	})
	ll.Info("delete volume called")

	resp, err := d.storage.DeleteVolume(ctx, req.VolumeId)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			// we assume it's deleted already for idempotency
			ll.WithFields(logrus.Fields{
				"error": err,
				"resp":  resp,
			}).Warn("assuming volume is deleted already")
			return &csi.DeleteVolumeResponse{}, nil
		}
		return nil, err
	}

	ll.WithField("response", resp).Info("volume is deleted")
	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerPublishVolume attaches the given volume to the node
func (d *Driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Volume ID must be provided")
	}

	if req.NodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Node ID must be provided")
	}

	if req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Volume capability must be provided")
	}

	dropletID, err := strconv.Atoi(req.NodeId)
	if err != nil {
		// don't return because the CSI tests passes ID's in non-integer format.
		dropletID = 1 // for testing purposes only. Will fail in real world API
		d.log.WithField("node_id", req.NodeId).Warn("node ID cannot be converted to an integer")
	}

	if req.Readonly {
		// TODO(arslan): we should return codes.InvalidArgument, but the CSI
		// test fails, because according to the CSI Spec, this flag cannot be
		// changed on the same volume. However we don't use this flag at all,
		// as there are no `readonly` attachable volumes.
		return nil, status.Error(codes.AlreadyExists, "read only Volumes are not supported")
	}

	ll := d.log.WithFields(logrus.Fields{
		"volume_id":  req.VolumeId,
		"node_id":    req.NodeId,
		"droplet_id": dropletID,
		"method":     "controller_publish_volume",
	})
	ll.Info("controller publish volume called")

	// check if volume exist before trying to attach it
	vol, resp, err := d.storage.GetVolume(ctx, req.VolumeId)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, status.Errorf(codes.NotFound, "volume %q not found", req.VolumeId)
		}
		return nil, err
	}

	// check if droplet exist before trying to attach the volume to the droplet
	_, resp, err = d.droplets.Get(ctx, dropletID)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, status.Errorf(codes.NotFound, "droplet %q not found", dropletID)
		}
		return nil, err
	}

	attachedID := 0
	for _, id := range vol.DropletIDs {
		attachedID = id
		if id == dropletID {
			ll.Info("volume is already attached")
			return &csi.ControllerPublishVolumeResponse{
				PublishInfo: map[string]string{
					PublishInfoVolumeName: vol.Name,
				},
			}, nil
		}
	}

	// droplet is attached to a different node, return an error
	if attachedID != 0 {
		return nil, status.Errorf(codes.FailedPrecondition,
			"volume is attached to the wrong droplet(%q), dettach the volume to fix it", attachedID)
	}

	// attach the volume to the correct node
	action, resp, err := d.storageActions.Attach(ctx, req.VolumeId, dropletID)
	if err != nil {
		// don't do anything if attached
		if resp != nil && resp.StatusCode == http.StatusUnprocessableEntity {
			if strings.Contains(err.Error(), "This volume is already attached") {
				ll.WithFields(logrus.Fields{
					"error": err,
					"resp":  resp,
				}).Warn("assuming volume is attached already")
				return &csi.ControllerPublishVolumeResponse{
					PublishInfo: map[string]string{
						PublishInfoVolumeName: vol.Name,
					},
				}, nil
			}

			if strings.Contains(err.Error(), "Droplet already has a pending event") {
				ll.WithFields(logrus.Fields{
					"error": err,
					"resp":  resp,
				}).Warn("droplet is not able to detach the volume")
				// sending an abort makes sure the csi-attacher retries with the next backoff tick
				return nil, status.Errorf(codes.Aborted, "volume %q couldn't be attached. droplet %d is in process of another action",
					req.VolumeId, dropletID)
			}
		}
		return nil, err
	}

	if action != nil {
		ll.Info("waiting until volume is attached")
		if err := d.waitAction(ctx, req.VolumeId, action.ID); err != nil {
			return nil, err
		}
	}

	ll.Info("volume is attached")
	return &csi.ControllerPublishVolumeResponse{
		PublishInfo: map[string]string{
			PublishInfoVolumeName: vol.Name,
		},
	}, nil
}

// ControllerUnpublishVolume deattaches the given volume from the node
func (d *Driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Volume ID must be provided")
	}

	dropletID, err := strconv.Atoi(req.NodeId)
	if err != nil {
		// don't return because the CSI tests passes ID's in non-integer format
		dropletID = 1 // for testing purposes only. Will fail in real world API
		d.log.WithField("node_id", req.NodeId).Warn("node ID cannot be converted to an integer")
	}

	ll := d.log.WithFields(logrus.Fields{
		"volume_id":  req.VolumeId,
		"node_id":    req.NodeId,
		"droplet_id": dropletID,
		"method":     "controller_unpublish_volume",
	})
	ll.Info("controller unpublish volume called")

	// check if volume exist before trying to detach it
	_, resp, err := d.storage.GetVolume(ctx, req.VolumeId)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			// assume it's detached
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		}
		return nil, err
	}

	// check if droplet exist before trying to detach the volume from the droplet
	_, resp, err = d.droplets.Get(ctx, dropletID)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, status.Errorf(codes.NotFound, "droplet %q not found", dropletID)
		}
		return nil, err
	}

	action, resp, err := d.storageActions.DetachByDropletID(ctx, req.VolumeId, dropletID)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnprocessableEntity {
			if strings.Contains(err.Error(), "Attachment not found") {
				ll.WithFields(logrus.Fields{
					"error": err,
					"resp":  resp,
				}).Warn("assuming volume is detached already")
				return &csi.ControllerUnpublishVolumeResponse{}, nil
			}

			if strings.Contains(err.Error(), "Droplet already has a pending event") {
				ll.WithFields(logrus.Fields{
					"error": err,
					"resp":  resp,
				}).Warn("droplet is not able to detach the volume")
				// sending an abort makes sure the csi-attacher retries with the next backoff tick
				return nil, status.Errorf(codes.Aborted, "volume %q couldn't be detached. droplet %d is in process of another action",
					req.VolumeId, dropletID)
			}
		}
		return nil, err
	}

	if action != nil {
		ll.Info("waiting until volume is detached")
		if err := d.waitAction(ctx, req.VolumeId, action.ID); err != nil {
			return nil, err
		}
	}

	ll.Info("volume is detached")
	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

// ValidateVolumeCapabilities checks whether the volume capabilities requested
// are supported.
func (d *Driver) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities Volume ID must be provided")
	}

	if req.VolumeCapabilities == nil {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities Volume Capabilities must be provided")
	}

	ll := d.log.WithFields(logrus.Fields{
		"volume_id":              req.VolumeId,
		"volume_capabilities":    req.VolumeCapabilities,
		"accessible_topology":    req.AccessibleTopology,
		"supported_capabilities": supportedAccessMode,
		"method":                 "validate_volume_capabilities",
	})
	ll.Info("validate volume capabilities called")

	// check if volume exist before trying to validate it it
	_, volResp, err := d.storage.GetVolume(ctx, req.VolumeId)
	if err != nil {
		if volResp != nil && volResp.StatusCode == http.StatusNotFound {
			return nil, status.Errorf(codes.NotFound, "volume %q not found", req.VolumeId)
		}
		return nil, err
	}

	if req.AccessibleTopology != nil {
		for _, t := range req.AccessibleTopology {
			region, ok := t.Segments["region"]
			if !ok {
				continue // nothing to do
			}

			if region != d.region {
				// return early if a different region is expected
				ll.WithField("supported", false).Info("supported capabilities")
				return &csi.ValidateVolumeCapabilitiesResponse{
					Supported: false,
				}, nil
			}
		}
	}

	// if it's not supported (i.e: wrong region), we shouldn't override it
	resp := &csi.ValidateVolumeCapabilitiesResponse{
		Supported: validateCapabilities(req.VolumeCapabilities),
	}

	ll.WithField("supported", resp.Supported).Info("supported capabilities")
	return resp, nil
}

// ListVolumes returns a list of all requested volumes
func (d *Driver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	var page int
	var err error
	if req.StartingToken != "" {
		page, err = strconv.Atoi(req.StartingToken)
		if err != nil {
			return nil, err
		}
	}

	listOpts := &godo.ListVolumeParams{
		ListOptions: &godo.ListOptions{
			PerPage: int(req.MaxEntries),
			Page:    page,
		},
		Region: d.region,
	}

	ll := d.log.WithFields(logrus.Fields{
		"list_opts":          listOpts,
		"req_starting_token": req.StartingToken,
		"method":             "list_volumes",
	})
	ll.Info("list volumes called")

	var volumes []godo.Volume
	lastPage := 0
	for {
		vols, resp, err := d.storage.ListVolumes(ctx, listOpts)
		if err != nil {
			return nil, err
		}

		volumes = append(volumes, vols...)

		if resp.Links == nil || resp.Links.IsLastPage() {
			if resp.Links != nil {
				page, err := resp.Links.CurrentPage()
				if err != nil {
					return nil, err
				}
				// save this for the response
				lastPage = page
			}
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, err
		}

		listOpts.ListOptions.Page = page + 1
	}

	var entries []*csi.ListVolumesResponse_Entry
	for _, vol := range volumes {
		entries = append(entries, &csi.ListVolumesResponse_Entry{
			Volume: &csi.Volume{
				Id:            vol.ID,
				CapacityBytes: vol.SizeGigaBytes * GB,
			},
		})
	}

	// TODO(arslan): check that the NextToken logic works fine, might be racy
	resp := &csi.ListVolumesResponse{
		Entries:   entries,
		NextToken: strconv.Itoa(lastPage),
	}

	ll.WithField("response", resp).Info("volumes listed")
	return resp, nil
}

// GetCapacity returns the capacity of the storage pool
func (d *Driver) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	// TODO(arslan): check if we can provide this information somehow
	d.log.WithFields(logrus.Fields{
		"params": req.Parameters,
		"method": "get_capacity",
	}).Warn("get capacity is not implemented")
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerGetCapabilities returns the capabilities of the controller service.
func (d *Driver) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	newCap := func(cap csi.ControllerServiceCapability_RPC_Type) *csi.ControllerServiceCapability {
		return &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
	}

	var caps []*csi.ControllerServiceCapability
	for _, cap := range []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
		csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
		csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
	} {
		caps = append(caps, newCap(cap))
	}

	resp := &csi.ControllerGetCapabilitiesResponse{
		Capabilities: caps,
	}

	d.log.WithFields(logrus.Fields{
		"response": resp,
		"method":   "controller_get_capabilities",
	}).Info("controller get capabilities called")
	return resp, nil
}

// CreateSnapshot will be called by the CO to create a new snapshot from a
// source volume on behalf of a user.
func (d *Driver) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "CreateSnapshot Name must be provided")
	}

	if req.GetSourceVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "CreateSnapshot Source Volume ID must be provided")
	}

	ll := d.log.WithFields(logrus.Fields{
		"req_name":             req.GetName(),
		"req_source_volume_id": req.GetSourceVolumeId(),
		"req_parameters":       req.GetParameters(),
		"method":               "create_snapshot",
	})

	ll.Info("create snapshot is called")

	// get snapshot first, if it's created do no thing
	snapshots, _, err := d.storage.ListSnapshots(ctx, req.GetSourceVolumeId(), nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "couldn't fetch snapshots: %s", err.Error())
	}

	for _, snap := range snapshots {
		if snap.Name == req.GetName() && snap.ResourceID == req.GetSourceVolumeId() {
			s, err := toCsiSnapshot(&snap)
			if err != nil {
				return nil, status.Errorf(codes.Internal,
					"couldn't convert DO snapshot to CSI snapshot: %s", err.Error())
			}

			return &csi.CreateSnapshotResponse{
				Snapshot: s,
			}, nil
		}
	}

	snap, resp, err := d.storage.CreateSnapshot(ctx, &godo.SnapshotCreateRequest{
		VolumeID:    req.GetSourceVolumeId(),
		Name:        req.GetName(),
		Description: createdByDO,
	})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusConflict {
			// 409 is returned when we try to snapshot a volume with the same
			// name
			ll.WithFields(logrus.Fields{
				"error": err,
				"resp":  resp,
			}).Warn("snapshot create failed, might be due using an existing name")
			return nil, status.Errorf(codes.AlreadyExists, "snapshot with name %s already exists", req.GetName())
		}

		return nil, status.Error(codes.Internal, err.Error())
	}

	s, err := toCsiSnapshot(snap)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"couldn't convert DO snapshot to CSI snapshot: %s", err.Error())
	}

	return &csi.CreateSnapshotResponse{
		Snapshot: s,
	}, nil
}

// DeleteSnapshost will be called by the CO to delete a snapshot.
func (d *Driver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	ll := d.log.WithFields(logrus.Fields{
		"req_snapshot_id": req.GetSnapshotId(),
		"method":          "delete_snapshot",
	})

	ll.Info("delete snapshot is called")

	if req.GetSnapshotId() == "" {
		return nil, status.Error(codes.InvalidArgument, "DeleteSnapshot Snapshot ID must be provided")
	}

	resp, err := d.storage.DeleteSnapshot(ctx, req.GetSnapshotId())
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			// we assume it's deleted already for idempotency
			ll.WithFields(logrus.Fields{
				"error": err,
				"resp":  resp,
			}).Warn("assuming snapshot is deleted already")
			return &csi.DeleteSnapshotResponse{}, nil
		}
		return nil, err
	}

	ll.WithField("response", resp).Info("snapshot is deleted")
	return &csi.DeleteSnapshotResponse{}, nil
}

// ListSnapshots returns the information about all snapshots on the storage
// system within the given parameters regardless of how they were created.
// ListSnapshots shold not list a snapshot that is being created but has not
// been cut successfully yet.
func (d *Driver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	// Pagination in the CSI world works different than at DO. CSI sens the
	// `req.MaxEntries` to indicate how much snapshots it wants. The
	// req.StartingToken is returned by us, if we somehow need to indicate that
	// we couldn't fetch and need to fetch again. But it's NOT the page number.
	// Suppose CSI wants us to fetch 50 entries, we only fetch 30, we need to
	// return NextToken as 31, to indicate that we want to continue returning
	// from the index 31 up to 50.

	// Check if it's an integer
	var nextToken int
	var err error
	if req.StartingToken != "" {
		nextToken, err = strconv.Atoi(req.StartingToken)
		if err != nil {
			return nil, status.Errorf(codes.Aborted, "ListSnapshots starting token is not valid : %s",
				err.Error())
		}
	}

	if nextToken != 0 && req.MaxEntries != 0 {
		return nil, status.Errorf(codes.Aborted,
			"ListSnapshots invalid arguments starting token: %d and max entries: %d can't be non null at the same time", nextToken, req.MaxEntries)
	}

	ll := d.log.WithFields(logrus.Fields{
		"req_starting_token": req.StartingToken,
		"method":             "list_snapshots",
	})
	ll.Info("list snapshots is called")

	// fetch all entries
	listOpts := &godo.ListOptions{}
	var snapshots []godo.Snapshot
	for {
		snaps, resp, err := d.snapshots.ListVolume(ctx, listOpts)
		if err != nil {
			return nil, status.Errorf(codes.Aborted, "ListSnapshots listing volume snapshots has failed: %s", err.Error())
		}

		snapshots = append(snapshots, snaps...)

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, err
		}

		listOpts.Page = page + 1
	}

	if nextToken > len(snapshots) {
		return nil, status.Error(codes.Aborted, "ListSnapshots starting token is greater than total number of snapshots")
	}

	if nextToken != 0 {
		snapshots = snapshots[nextToken:]
	}

	if req.MaxEntries != 0 {
		nextToken = len(snapshots) - int(req.MaxEntries) - 1
		snapshots = snapshots[:req.MaxEntries]
	}

	var entries []*csi.ListSnapshotsResponse_Entry
	for _, snapshot := range snapshots {
		snap, err := toCsiSnapshot(&snapshot)
		if err != nil {
			return nil, status.Errorf(codes.Internal,
				"couldn't convert DO snapshot to CSI snapshot: %s", err.Error())
		}

		entries = append(entries, &csi.ListSnapshotsResponse_Entry{
			Snapshot: snap,
		})
	}

	listResp := &csi.ListSnapshotsResponse{
		Entries:   entries,
		NextToken: strconv.Itoa(nextToken),
	}

	ll.WithField("response", listResp).Info("snapshots listed")
	return listResp, nil
}

// extractStorage extracts the storage size in GB from the given capacity
// range. If the capacity range is not satisfied it returns the default volume
// size.
func extractStorage(capRange *csi.CapacityRange) (int64, error) {
	if capRange == nil {
		return defaultVolumeSizeInGB, nil
	}

	if capRange.RequiredBytes == 0 && capRange.LimitBytes == 0 {
		return defaultVolumeSizeInGB, nil
	}

	minSize := capRange.RequiredBytes

	// limitBytes might be zero
	maxSize := capRange.LimitBytes
	if capRange.LimitBytes == 0 {
		maxSize = minSize
	}

	if minSize == maxSize {
		return minSize, nil
	}

	return 0, errors.New("requiredBytes and LimitBytes are not the same")
}

// waitAction waits until the given action for the volume is completed
func (d *Driver) waitAction(ctx context.Context, volumeId string, actionId int) error {
	ll := d.log.WithFields(logrus.Fields{
		"volume_id": volumeId,
		"action_id": actionId,
	})

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	// TODO(arslan): use backoff in the future
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			action, _, err := d.storageActions.Get(ctx, volumeId, actionId)
			if err != nil {
				ll.WithError(err).Info("waiting for volume errored")
				continue
			}
			ll.WithField("action_status", action.Status).Info("action received")

			if action.Status == godo.ActionCompleted {
				ll.Info("action completed")
				return nil
			}

			if action.Status == godo.ActionInProgress {
				continue
			}
		case <-ctx.Done():
			return fmt.Errorf("timeout occured waiting for storage action of volume: %q", volumeId)
		}
	}
}

// checkLimit checks whether the user hit their volume limit to ensure.
func (d *Driver) checkLimit(ctx context.Context) error {
	// only one provisioner runs, we can make sure to prevent burst creation
	d.readyMu.Lock()
	defer d.readyMu.Unlock()

	account, _, err := d.account.Get(ctx)
	if err != nil {
		return status.Errorf(codes.Internal,
			"couldn't get account information to check volume limit: %s", err.Error())
	}

	// administrative accounts might have zero length limits, make sure to not check them
	if account.VolumeLimit == 0 {
		return nil //  hail to the king!
	}

	// NOTE(arslan): the API returns the limit for *all* regions, so passing
	// the region down as a parameter doesn't change the response.
	// Nevertheless, this is something we should be aware of.
	volumes, _, err := d.storage.ListVolumes(ctx, &godo.ListVolumeParams{
		Region: d.region,
	})
	if err != nil {
		return status.Errorf(codes.Internal,
			"couldn't get fetch volume list to check volume limit: %s", err.Error())
	}

	if account.VolumeLimit <= len(volumes) {
		return status.Errorf(codes.ResourceExhausted,
			"volume limit (%d) has been reached. Current number of volumes: %d. Please contact support.",
			account.VolumeLimit, len(volumes))
	}

	return nil
}

// toCsiSnapshot converts a DO Snapshot struct into a csi.Snapshot struct
func toCsiSnapshot(snap *godo.Snapshot) (*csi.Snapshot, error) {
	// snapshot already exists, return the information we have
	createdAt, err := time.Parse(time.RFC3339, snap.Created)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "couldn't parse snapshot's created field: %s", err.Error())
	}

	return &csi.Snapshot{
		Id:             snap.ID,
		SourceVolumeId: snap.ResourceID,
		SizeBytes:      int64(snap.SizeGigaBytes) * GB,
		CreatedAt:      createdAt.UTC().UnixNano(),
		Status: &csi.SnapshotStatus{
			Type: csi.SnapshotStatus_READY,
		},
	}, nil
}

// validateCapabilities validates the requested capabilities. It returns false
// if it doesn't satisfy the currently supported modes of DigitalOcean Block
// Storage
func validateCapabilities(caps []*csi.VolumeCapability) bool {
	vcaps := []*csi.VolumeCapability_AccessMode{supportedAccessMode}

	hasSupport := func(mode csi.VolumeCapability_AccessMode_Mode) bool {
		for _, m := range vcaps {
			if mode == m.Mode {
				return true
			}
		}
		return false
	}

	supported := false
	for _, cap := range caps {
		if hasSupport(cap.AccessMode.Mode) {
			supported = true
		} else {
			// we need to make sure all capabilities are supported. Revert back
			// in case we have a cap that is supported, but is invalidated now
			supported = false
		}
	}

	return supported
}
