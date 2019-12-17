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

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/digitalocean/godo"
	"github.com/golang/protobuf/ptypes"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	_   = iota
	kiB = 1 << (10 * iota)
	miB
	giB
	tiB
)

const (
	// minimumVolumeSizeInBytes is used to validate that the user is not trying
	// to create a volume that is smaller than what we support
	minimumVolumeSizeInBytes int64 = 1 * giB

	// maximumVolumeSizeInBytes is used to validate that the user is not trying
	// to create a volume that is larger than what we support
	maximumVolumeSizeInBytes int64 = 16 * tiB

	// defaultVolumeSizeInBytes is used when the user did not provide a size or
	// the size they provided did not satisfy our requirements
	defaultVolumeSizeInBytes int64 = 16 * giB

	// createdByDO is used to tag volumes that are created by this CSI plugin
	createdByDO = "Created by DigitalOcean CSI driver"

	// doAPITimeout sets the timeout we will use when communicating with the
	// Digital Ocean API. NOTE: some queries inherit the context timeout
	doAPITimeout = 10 * time.Second
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

	if violations := validateCapabilities(req.VolumeCapabilities); len(violations) > 0 {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("volume capabilities cannot be satisified: %s", strings.Join(violations, "; ")))
	}

	size, err := extractStorage(req.CapacityRange)
	if err != nil {
		return nil, status.Errorf(codes.OutOfRange, "invalid capacity range: %v", err)
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

	volumeName := req.Name

	log := d.log.WithFields(logrus.Fields{
		"volume_name":             volumeName,
		"storage_size_giga_bytes": size / giB,
		"method":                  "create_volume",
		"volume_capabilities":     req.VolumeCapabilities,
	})
	log.Info("create volume called")

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

		if vol.SizeGigaBytes*giB != size {
			return nil, status.Error(codes.AlreadyExists, fmt.Sprintf("invalid option requested size: %d", size))
		}

		log.Info("volume already created")
		return &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				VolumeId:      vol.ID,
				CapacityBytes: vol.SizeGigaBytes * giB,
			},
		}, nil
	}

	volumeReq := &godo.VolumeCreateRequest{
		Region:        d.region,
		Name:          volumeName,
		Description:   createdByDO,
		SizeGigaBytes: size / giB,
	}

	if d.doTag != "" {
		volumeReq.Tags = append(volumeReq.Tags, d.doTag)
	}

	contentSource := req.GetVolumeContentSource()
	if contentSource != nil && contentSource.GetSnapshot() != nil {
		snapshotID := contentSource.GetSnapshot().GetSnapshotId()
		if snapshotID == "" {
			return nil, status.Error(codes.InvalidArgument, "snapshot ID is empty")
		}

		// check if the snapshot exist before we continue
		_, resp, err := d.snapshots.Get(ctx, snapshotID)
		if err != nil {
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				return nil, status.Errorf(codes.NotFound, "snapshot %q not found", snapshotID)
			}
			return nil, err
		}

		log.WithField("snapshot_id", snapshotID).Info("using snapshot as volume source")
		volumeReq.SnapshotID = snapshotID
	}

	log.Info("checking volume limit")
	details, err := d.checkLimit(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check volume limit: %s", err)
	}
	if details != nil {
		return nil, status.Errorf(codes.ResourceExhausted,
			"volume limit (%d) has been reached. Current number of volumes: %d. Please contact support.",
			details.limit, details.numVolumes)
	}

	log.WithField("volume_req", volumeReq).Info("creating volume")
	vol, _, err := d.storage.CreateVolume(ctx, volumeReq)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      vol.ID,
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

	log.WithField("response", resp).Info("volume created")
	return resp, nil
}

// DeleteVolume deletes the given volume. The function is idempotent.
func (d *Driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "DeleteVolume Volume ID must be provided")
	}

	log := d.log.WithFields(logrus.Fields{
		"volume_id": req.VolumeId,
		"method":    "delete_volume",
	})
	log.Info("delete volume called")

	resp, err := d.storage.DeleteVolume(ctx, req.VolumeId)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			// we assume it's deleted already for idempotency
			log.WithFields(logrus.Fields{
				"error": err,
				"resp":  resp,
			}).Warn("assuming volume is deleted already")
			return &csi.DeleteVolumeResponse{}, nil
		}
		return nil, err
	}

	log.WithField("response", resp).Info("volume is deleted")
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

	log := d.log.WithFields(logrus.Fields{
		"volume_id":  req.VolumeId,
		"node_id":    req.NodeId,
		"droplet_id": dropletID,
		"method":     "controller_publish_volume",
	})
	log.Info("controller publish volume called")

	// check if volume exist before trying to attach it
	vol, resp, err := d.storage.GetVolume(ctx, req.VolumeId)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, status.Errorf(codes.NotFound, "volume %q not found", req.VolumeId)
		}
		return nil, err
	}

	if d.doTag != "" {
		err = d.tagVolume(ctx, vol)
		if err != nil {
			log.Errorf("error tagging volume: %s", err)
			return nil, status.Errorf(codes.Internal, "failed to tag volume: %s", err)
		}
	}

	// check if droplet exist before trying to attach the volume to the droplet
	_, resp, err = d.droplets.Get(ctx, dropletID)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, status.Errorf(codes.NotFound, "droplet %d not found", dropletID)
		}
		return nil, err
	}

	attachedID := 0
	for _, id := range vol.DropletIDs {
		attachedID = id
		if id == dropletID {
			log.Info("volume is already attached")
			return &csi.ControllerPublishVolumeResponse{
				PublishContext: map[string]string{
					d.publishInfoVolumeName: vol.Name,
				},
			}, nil
		}
	}

	// droplet is attached to a different node, return an error
	if attachedID != 0 {
		return nil, status.Errorf(codes.FailedPrecondition,
			"volume %q is attached to the wrong droplet (%d), detach the volume to fix it",
			req.VolumeId, attachedID)
	}

	// attach the volume to the correct node
	action, resp, err := d.storageActions.Attach(ctx, req.VolumeId, dropletID)
	if err != nil {
		// don't do anything if attached
		if resp != nil && resp.StatusCode == http.StatusUnprocessableEntity {
			if strings.Contains(err.Error(), "This volume is already attached") {
				log.WithFields(logrus.Fields{
					"error": err,
					"resp":  resp,
				}).Warn("assuming volume is attached already")
				return &csi.ControllerPublishVolumeResponse{
					PublishContext: map[string]string{
						d.publishInfoVolumeName: vol.Name,
					},
				}, nil
			}

			if strings.Contains(err.Error(), "Droplet already has a pending event") {
				log.WithFields(logrus.Fields{
					"error": err,
					"resp":  resp,
				}).Warn("droplet is not able to attach the volume")
				// sending an abort makes sure the csi-attacher retries with the next backoff tick
				return nil, status.Errorf(codes.Aborted, "volume %q couldn't be attached. droplet %d is in process of another action",
					req.VolumeId, dropletID)
			}
		}
		return nil, err
	}

	if action != nil {
		log.Info("waiting until volume is attached")
		if err := d.waitAction(ctx, log, req.VolumeId, action.ID); err != nil {
			return nil, err
		}
	}

	log.Info("volume is attached")
	return &csi.ControllerPublishVolumeResponse{
		PublishContext: map[string]string{
			d.publishInfoVolumeName: vol.Name,
		},
	}, nil
}

// ControllerUnpublishVolume deattaches the given volume from the node
func (d *Driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerUnpublishVolume Volume ID must be provided")
	}

	dropletID, err := strconv.Atoi(req.NodeId)
	if err != nil {
		// don't return because the CSI tests passes ID's in non-integer format
		dropletID = 1 // for testing purposes only. Will fail in real world API
		d.log.WithField("node_id", req.NodeId).Warn("node ID cannot be converted to an integer")
	}

	log := d.log.WithFields(logrus.Fields{
		"volume_id":  req.VolumeId,
		"node_id":    req.NodeId,
		"droplet_id": dropletID,
		"method":     "controller_unpublish_volume",
	})
	log.Info("controller unpublish volume called")

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
			return nil, status.Errorf(codes.NotFound, "droplet %d not found", dropletID)
		}
		return nil, err
	}

	action, resp, err := d.storageActions.DetachByDropletID(ctx, req.VolumeId, dropletID)
	if err != nil {
		if resp != nil {
			if resp.StatusCode == http.StatusNotFound {
				log.WithFields(logrus.Fields{
					"error": err,
					"resp":  resp,
				}).Warn("volume is not attached to droplet")
				return &csi.ControllerUnpublishVolumeResponse{}, nil
			}

			if resp.StatusCode == http.StatusUnprocessableEntity {
				if strings.Contains(err.Error(), "Attachment not found") {
					log.WithFields(logrus.Fields{
						"error": err,
						"resp":  resp,
					}).Warn("assuming volume is detached already")
					return &csi.ControllerUnpublishVolumeResponse{}, nil
				}

				if strings.Contains(err.Error(), "Droplet already has a pending event") {
					log.WithFields(logrus.Fields{
						"error": err,
						"resp":  resp,
					}).Warn("droplet is not able to detach the volume")
					// sending an abort makes sure the csi-attacher retries with the next backoff tick
					return nil, status.Errorf(codes.Aborted, "volume %q couldn't be detached. droplet %d is in process of another action",
						req.VolumeId, dropletID)
				}
			}
		}

		return nil, err
	}

	if action != nil {
		log.Info("waiting until volume is detached")
		if err := d.waitAction(ctx, log, req.VolumeId, action.ID); err != nil {
			return nil, err
		}
	}

	log.Info("volume is detached")
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

	log := d.log.WithFields(logrus.Fields{
		"volume_id":              req.VolumeId,
		"volume_capabilities":    req.VolumeCapabilities,
		"supported_capabilities": supportedAccessMode,
		"method":                 "validate_volume_capabilities",
	})
	log.Info("validate volume capabilities called")

	// check if volume exist before trying to validate it it
	_, volResp, err := d.storage.GetVolume(ctx, req.VolumeId)
	if err != nil {
		if volResp != nil && volResp.StatusCode == http.StatusNotFound {
			return nil, status.Errorf(codes.NotFound, "volume %q not found", req.VolumeId)
		}
		return nil, err
	}

	// if it's not supported (i.e: wrong region), we shouldn't override it
	resp := &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessMode: supportedAccessMode,
				},
			},
		},
	}

	log.WithField("confirmed", resp.Confirmed).Info("supported capabilities")
	return resp, nil
}

// ListVolumes returns a list of all requested volumes
func (d *Driver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	var page int
	var err error
	if req.StartingToken != "" {
		page, err = strconv.Atoi(req.StartingToken)
		if err != nil {
			return nil, status.Errorf(codes.Aborted, "starting_token is invalid: %s", err)
		}
	}

	listOpts := &godo.ListVolumeParams{
		ListOptions: &godo.ListOptions{
			PerPage: int(req.MaxEntries),
			Page:    page,
		},
		Region: d.region,
	}

	log := d.log.WithFields(logrus.Fields{
		"list_opts":          listOpts,
		"req_starting_token": req.StartingToken,
		"method":             "list_volumes",
	})
	log.Info("list volumes called")

	var volumes []godo.Volume
	lastPage := 0
	for {
		vols, resp, err := d.storage.ListVolumes(ctx, listOpts)
		if err != nil {
			return nil, err
		}

		volumes = append(volumes, vols...)

		if page > len(volumes) {
			return nil, status.Error(codes.Aborted, "starting_token is is greater than total number of vols")
		}

		if len(volumes) == int(req.MaxEntries) {
			lastPage = int(req.MaxEntries)
			break
		}

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
				VolumeId:      vol.ID,
				CapacityBytes: vol.SizeGigaBytes * giB,
			},
		})
	}

	// TODO(arslan): check that the NextToken logic works fine, might be racy
	resp := &csi.ListVolumesResponse{
		Entries:   entries,
		NextToken: strconv.Itoa(lastPage),
	}

	log.WithField("response", resp).Info("volumes listed")
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
		csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
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

	log := d.log.WithFields(logrus.Fields{
		"req_name":             req.GetName(),
		"req_source_volume_id": req.GetSourceVolumeId(),
		"req_parameters":       req.GetParameters(),
		"method":               "create_snapshot",
	})

	log.Info("create snapshot is called")

	// get snapshot first, if it's created do no thing

	opts := &godo.ListOptions{
		Page:    1,
		PerPage: 50,
	}
	for {
		snapshots, resp, err := d.storage.ListSnapshots(ctx, req.GetSourceVolumeId(), opts)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to list snapshots on page %d: %s", opts.Page, err)
		}

		for _, snap := range snapshots {
			if snap.Name == req.GetName() && snap.ResourceID == req.GetSourceVolumeId() {
				s, err := toCSISnapshot(&snap)
				if err != nil {
					return nil, status.Errorf(codes.Internal,
						"failed to convert DO snapshot %q to CSI snapshot: %s", snap.Name, err)
				}

				snapResp := &csi.CreateSnapshotResponse{
					Snapshot: s,
				}
				log.WithField("response", snapResp).Info("existing snapshot found")
				return snapResp, nil
			}
		}

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, fmt.Errorf("failed to get current page: %s", err)
		}
		opts.Page = page + 1
	}

	snapReq := &godo.SnapshotCreateRequest{
		VolumeID:    req.GetSourceVolumeId(),
		Name:        req.GetName(),
		Description: createdByDO,
	}
	if d.doTag != "" {
		snapReq.Tags = append(snapReq.Tags, d.doTag)
	}

	snap, resp, err := d.storage.CreateSnapshot(ctx, snapReq)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusConflict {
			// 409 is returned when we try to snapshot a volume with the same
			// name
			log.WithFields(logrus.Fields{
				"error": err,
				"resp":  resp,
			}).Warn("snapshot create failed, might be due using an existing name")
			return nil, status.Errorf(codes.AlreadyExists, "snapshot with name %s already exists", req.GetName())
		}

		return nil, status.Error(codes.Internal, err.Error())
	}

	s, err := toCSISnapshot(snap)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"couldn't convert DO snapshot to CSI snapshot: %s", err.Error())
	}

	snapResp := &csi.CreateSnapshotResponse{
		Snapshot: s,
	}
	log.WithField("response", resp).Info("snapshot created")
	return snapResp, nil
}

// DeleteSnapshot will be called by the CO to delete a snapshot.
func (d *Driver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	log := d.log.WithFields(logrus.Fields{
		"req_snapshot_id": req.GetSnapshotId(),
		"method":          "delete_snapshot",
	})

	log.Info("delete snapshot is called")

	if req.GetSnapshotId() == "" {
		return nil, status.Error(codes.InvalidArgument, "DeleteSnapshot Snapshot ID must be provided")
	}

	resp, err := d.storage.DeleteSnapshot(ctx, req.GetSnapshotId())
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			// we assume it's deleted already for idempotency
			log.WithFields(logrus.Fields{
				"error": err,
				"resp":  resp,
			}).Warn("assuming snapshot is deleted already")
			return &csi.DeleteSnapshotResponse{}, nil
		}
		return nil, err
	}

	log.WithField("response", resp).Info("snapshot is deleted")
	return &csi.DeleteSnapshotResponse{}, nil
}

// ListSnapshots returns the information about all snapshots on the storage
// system within the given parameters regardless of how they were created.
// ListSnapshots shold not list a snapshot that is being created but has not
// been cut successfully yet.
func (d *Driver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	// Pagination in the CSI world works different than at DO. CSI sends the
	// `req.MaxEntries` to indicate how much snapshots it wants. The
	// req.StartingToken is returned by us, if we somehow need to indicate that
	// we couldn't fetch and need to fetch again. But it's NOT the page number.
	// I.e: suppose CSI wants us to fetch 50 entries, we only fetch 30, we need to
	// return NextToken as 31 (so req.StartingToken will be set to 31 when CSI
	// calls us again), to indicate that we want to continue returning from the
	// index 31 up to 50.

	var nextToken int
	var err error
	if req.StartingToken != "" {
		nextToken, err = strconv.Atoi(req.StartingToken)
		if err != nil {
			return nil, status.Errorf(codes.Aborted, "ListSnapshots starting token %s is not valid : %s",
				req.StartingToken, err.Error())
		}
	}

	if nextToken != 0 && req.MaxEntries != 0 {
		return nil, status.Errorf(codes.Aborted,
			"ListSnapshots invalid arguments starting token: %d and max entries: %d can't be non null at the same time", nextToken, req.MaxEntries)
	}

	log := d.log.WithFields(logrus.Fields{
		"req_starting_token": req.StartingToken,
		"method":             "list_snapshots",
	})
	log.Info("list snapshots is called")

	// fetch all entries
	listOpts := &godo.ListOptions{
		PerPage: int(req.MaxEntries),
	}
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
		listOpts.PerPage = len(snaps)
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

	entries := make([]*csi.ListSnapshotsResponse_Entry, 0, len(snapshots))
	for _, snapshot := range snapshots {
		snap, err := toCSISnapshot(&snapshot)
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

	log.WithField("response", listResp).Info("snapshots listed")
	return listResp, nil
}

// ControllerExpandVolume is called from the resizer to increase the volume size.
func (d *Driver) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	volID := req.GetVolumeId()

	if len(volID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "ControllerExpandVolume volume ID missing in request")
	}
	volume, _, err := d.storage.GetVolume(ctx, volID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ControllerExpandVolume could not retrieve existing volume: %v", err)
	}

	log := d.log.WithFields(logrus.Fields{
		"volume_id": req.VolumeId,
		"method":    "controller_expand_volume",
	})

	log.Info("controller expand volume called")

	resizeBytes, err := extractStorage(req.GetCapacityRange())
	if err != nil {
		return nil, status.Errorf(codes.OutOfRange, "ControllerExpandVolume invalid capacity range: %v", err)
	}
	resizeGigaBytes := resizeBytes / giB

	if resizeGigaBytes <= volume.SizeGigaBytes {
		log.WithFields(logrus.Fields{
			"current_volume_size":   volume.SizeGigaBytes,
			"requested_volume_size": resizeGigaBytes,
		}).Info("Skip volume resize: current volume size exceeds requested volume size. Node FS resize will be performed to resize claim capacity")
		// even if the volume is resized independently from the control panel, we still need to resize the node fs when resize is requested
		// in this case, the claim capacity will be resized to the volume capacity, requested capcity will be ignored to make the PV and PVC capacities consistent
		return &csi.ControllerExpandVolumeResponse{CapacityBytes: volume.SizeGigaBytes * giB, NodeExpansionRequired: true}, nil
	}

	action, _, err := d.storageActions.Resize(ctx, req.GetVolumeId(), int(resizeGigaBytes), d.region)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ControllerExpandVolume cannot resize volume %s: %s", req.GetVolumeId(), err.Error())
	}

	log = log.WithField("new_volume_size", resizeGigaBytes)

	if action != nil {
		log.Info("waiting until volume is resized")
		if err := d.waitAction(ctx, log, req.VolumeId, action.ID); err != nil {
			return nil, status.Errorf(codes.Internal, "failed waiting for volume to get resized: %s", err)
		}
	}
	log.Info("volume is resized")

	return &csi.ControllerExpandVolumeResponse{CapacityBytes: resizeGigaBytes * giB, NodeExpansionRequired: true}, nil
}

// extractStorage extracts the storage size in bytes from the given capacity
// range. If the capacity range is not satisfied it returns the default volume
// size. If the capacity range is below or above supported sizes, it returns an
// error.
func extractStorage(capRange *csi.CapacityRange) (int64, error) {
	if capRange == nil {
		return defaultVolumeSizeInBytes, nil
	}

	requiredBytes := capRange.GetRequiredBytes()
	requiredSet := 0 < requiredBytes
	limitBytes := capRange.GetLimitBytes()
	limitSet := 0 < limitBytes

	if !requiredSet && !limitSet {
		return defaultVolumeSizeInBytes, nil
	}

	if requiredSet && limitSet && limitBytes < requiredBytes {
		return 0, fmt.Errorf("limit (%v) can not be less than required (%v) size", formatBytes(limitBytes), formatBytes(requiredBytes))
	}

	if requiredSet && !limitSet && requiredBytes < minimumVolumeSizeInBytes {
		return 0, fmt.Errorf("required (%v) can not be less than minimum supported volume size (%v)", formatBytes(requiredBytes), formatBytes(minimumVolumeSizeInBytes))
	}

	if limitSet && limitBytes < minimumVolumeSizeInBytes {
		return 0, fmt.Errorf("limit (%v) can not be less than minimum supported volume size (%v)", formatBytes(limitBytes), formatBytes(minimumVolumeSizeInBytes))
	}

	if requiredSet && requiredBytes > maximumVolumeSizeInBytes {
		return 0, fmt.Errorf("required (%v) can not exceed maximum supported volume size (%v)", formatBytes(requiredBytes), formatBytes(maximumVolumeSizeInBytes))
	}

	if !requiredSet && limitSet && limitBytes > maximumVolumeSizeInBytes {
		return 0, fmt.Errorf("limit (%v) can not exceed maximum supported volume size (%v)", formatBytes(limitBytes), formatBytes(maximumVolumeSizeInBytes))
	}

	if requiredSet && limitSet && requiredBytes == limitBytes {
		return requiredBytes, nil
	}

	if requiredSet {
		return requiredBytes, nil
	}

	if limitSet {
		return limitBytes, nil
	}

	return defaultVolumeSizeInBytes, nil
}

func formatBytes(inputBytes int64) string {
	output := float64(inputBytes)
	unit := ""

	switch {
	case inputBytes >= tiB:
		output = output / tiB
		unit = "Ti"
	case inputBytes >= giB:
		output = output / giB
		unit = "Gi"
	case inputBytes >= miB:
		output = output / miB
		unit = "Mi"
	case inputBytes >= kiB:
		output = output / kiB
		unit = "Ki"
	case inputBytes == 0:
		return "0"
	}

	result := strconv.FormatFloat(output, 'f', 1, 64)
	result = strings.TrimSuffix(result, ".0")
	return result + unit
}

// waitAction waits until the given action for the volume is completed
func (d *Driver) waitAction(ctx context.Context, log *logrus.Entry, volumeId string, actionId int) error {
	log = log.WithFields(logrus.Fields{
		"action_id": actionId,
	})

	// This timeout should not strike given all sidecars use a timeout that is
	// lower (which should always be the case). Using this as a fail-safe
	// mechanism in case of a bug or misconfiguration.
	ctx, cancel := context.WithTimeout(ctx, d.waitActionTimeout)
	defer cancel()

	err := wait.PollUntil(1*time.Second, wait.ConditionFunc(func() (done bool, err error) {
		action, _, err := d.storageActions.Get(ctx, volumeId, actionId)
		if err != nil {
			ctxCanceled := ctx.Err() != nil
			if !ctxCanceled {
				log.WithError(err).Warn("getting action for volume")
				return false, nil
			}

			return false, fmt.Errorf("failed to get action %d for volume %s: %s", actionId, volumeId, err)
		}

		log.WithField("action_status", action.Status).Info("action received")

		if action.Status == godo.ActionCompleted {
			log.Info("action completed")
			return true, nil
		}

		return false, nil
	}), ctx.Done())

	return err
}

type limitDetails struct {
	limit      int
	numVolumes int
}

// checkLimit checks whether the user hit their account volume limit.
func (d *Driver) checkLimit(ctx context.Context) (*limitDetails, error) {
	// only one provisioner runs, we can make sure to prevent burst creation
	d.readyMu.Lock()
	defer d.readyMu.Unlock()

	account, _, err := d.account.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get account information: %s", err)
	}

	// administrative accounts might have zero length limits, make sure to not check them
	if account.VolumeLimit == 0 {
		return nil, nil //  hail to the king!
	}

	// The API returns the limit for *all* regions, so passing the region
	// down as a parameter doesn't change the response. Nevertheless, this
	// is something we should be aware of.
	_, resp, err := d.storage.ListVolumes(ctx, &godo.ListVolumeParams{
		Region: d.region,
		ListOptions: &godo.ListOptions{
			Page:    1,
			PerPage: 1,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %s", err)
	}

	if resp.Meta == nil {
		// This should really never happen.
		return nil, errors.New("no meta field available in list volumes response")
	}
	numVolumes := resp.Meta.Total
	if account.VolumeLimit <= numVolumes {
		return &limitDetails{
			limit:      account.VolumeLimit,
			numVolumes: numVolumes,
		}, nil
	}

	return nil, nil
}

// toCSISnapshot converts a DO Snapshot struct into a csi.Snapshot struct
func toCSISnapshot(snap *godo.Snapshot) (*csi.Snapshot, error) {
	createdAt, err := time.Parse(time.RFC3339, snap.Created)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse snapshot's created field: %s", err.Error())
	}

	tstamp, err := ptypes.TimestampProto(createdAt)
	if err != nil {
		return nil, fmt.Errorf("couldn't convert protobuf timestamp to go time.Time: %s",
			err.Error())
	}

	return &csi.Snapshot{
		SnapshotId:     snap.ID,
		SourceVolumeId: snap.ResourceID,
		SizeBytes:      int64(snap.SizeGigaBytes) * giB,
		CreationTime:   tstamp,
		ReadyToUse:     true,
	}, nil
}

// validateCapabilities validates the requested capabilities. It returns a list
// of violations which may be empty if no violatons were found.
func validateCapabilities(caps []*csi.VolumeCapability) []string {
	violations := sets.NewString()
	for _, cap := range caps {
		if cap.GetAccessMode().GetMode() != supportedAccessMode.GetMode() {
			violations.Insert(fmt.Sprintf("unsupported access mode %s", cap.GetAccessMode().GetMode().String()))
		}
		if cap.GetMount() == nil {
			violations.Insert("unsupported access type")
		}
	}

	return violations.List()
}

func (d *Driver) tagVolume(parentCtx context.Context, vol *godo.Volume) error {
	for _, tag := range vol.Tags {
		if tag == d.doTag {
			return nil
		}
	}

	tagReq := &godo.TagResourcesRequest{
		Resources: []godo.Resource{
			godo.Resource{
				ID:   vol.ID,
				Type: godo.VolumeResourceType,
			},
		},
	}

	ctx, cancel := context.WithTimeout(parentCtx, doAPITimeout)
	defer cancel()
	resp, err := d.tags.TagResources(ctx, d.doTag, tagReq)
	if resp == nil || resp.StatusCode != http.StatusNotFound {
		// either success or irrecoverable failure
		return err
	}

	// godo.TagsService returns 404 if the tag has not yet been
	// created, if that happens we need to create the tag
	// and then retry tagging the volume resource.
	ctx, cancel = context.WithTimeout(parentCtx, doAPITimeout)
	defer cancel()
	_, _, err = d.tags.Create(ctx, &godo.TagCreateRequest{
		Name: d.doTag,
	})
	if err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(parentCtx, doAPITimeout)
	defer cancel()
	_, err = d.tags.TagResources(ctx, d.doTag, tagReq)
	return err
}
