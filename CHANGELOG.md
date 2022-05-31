## unreleased

* Guarantee that action IDs are logged
  [[GH-433]](https://github.com/digitalocean/csi-digitalocean/pull/433)
* Make volumes page size customizable
  [[GH-431]](https://github.com/digitalocean/csi-digitalocean/pull/431)
* Upgrade sidecars
  [[GH-432]](https://github.com/digitalocean/csi-digitalocean/pull/432)
* Remove unnecessary waitActionTimeout
  [[GH-429]](https://github.com/digitalocean/csi-digitalocean/pull/429)
* Update to use Go 1.17.7
  [[GH-426]](https://github.com/digitalocean/csi-digitalocean/pull/426)

## v4.0.0 - 2022.02.07

* Update Kubernetes dependency to 1.22.5 and add support for 1.22 e2e tests
  [[GH-420]](https://github.com/digitalocean/csi-digitalocean/pull/420)
* Support snapshots v1
  [[GH-397]](https://github.com/digitalocean/csi-digitalocean/pull/397)

## v3.0.0 - 2022.01.07

* Mark CSI driver as default container
  [[GH-415]](https://github.com/digitalocean/csi-digitalocean/pull/415)
* Update snapshot CRDs to v3.0.3 from upstream
  [[GH-414]](https://github.com/digitalocean/csi-digitalocean/pull/414)
* Update snapshot controller to v3.0.3
  [[GH-412]](https://github.com/digitalocean/csi-digitalocean/pull/412)
* Update CSI spec to 1.5.0
  [[GH-411]](https://github.com/digitalocean/csi-digitalocean/pull/411)
* Populate published node IDs in ListVolumesResponse
  [[GH-410]](https://github.com/digitalocean/csi-digitalocean/pull/410)
* Add snapshot validation webhook
  [[GH-407]](https://github.com/digitalocean/csi-digitalocean/pull/407)
* Update registrar sidecar to v2.4.0
  [[GH-408]](https://github.com/digitalocean/csi-digitalocean/pull/408)
* Bump github.com/containerd/containerd from 1.5.2 to 1.5.8
  [[GH-402]](https://github.com/digitalocean/csi-digitalocean/pull/402)
* Add xfsprogs-extra to csi-do-plugin container to support xfs grows
  [[GH-405]](https://github.com/digitalocean/csi-digitalocean/pull/405)
* Update Go to 1.17.2
  [[GH-398]](https://github.com/digitalocean/csi-digitalocean/pull/398)
* Update sidecar dependencies
  [[GH-396]](https://github.com/digitalocean/csi-digitalocean/pull/396)
* Add Kubernetes 1.22 support
  [[GH-395]](https://github.com/digitalocean/csi-digitalocean/pull/395)
* Use k8s mount library to implement NodeUnpublishVolume
  [[GH-393]](https://github.com/digitalocean/csi-digitalocean/pull/393)

## v2.1.2 - 2021.09.07

* Update Kubernetes dependency to 1.21.3
  [[GH-389]](https://github.com/digitalocean/csi-digitalocean/pull/389)
* Refactor metadata service access
  [[GH-385]](https://github.com/digitalocean/csi-digitalocean/pull/385)

## v2.1.1 - 2020.10.30

* Fix fstype usage
  [[GH-363]](https://github.com/digitalocean/csi-digitalocean/pull/363)
* Match csi-provisioner RBAC rules with upstream
  [[GH-360]](https://github.com/digitalocean/csi-digitalocean/pull/360)

## v2.1.0 - 2020.10.07

* Surface health check failures
  [[GH-355]](https://github.com/digitalocean/csi-digitalocean/pull/355)
* Update csi-provisioner to v2.0.2 and csi-snapshotter to v3.0.0
  [[GH-353]](https://github.com/digitalocean/csi-digitalocean/pull/353)
* Run e2e tests for 1.19
  [[GH-349]](https://github.com/digitalocean/csi-digitalocean/pull/349)
* Update Go to 1.15.2
  [[GH-344]](https://github.com/digitalocean/csi-digitalocean/pull/344)
* Fix built-in release version
  [[GH-342]](https://github.com/digitalocean/csi-digitalocean/pull/342)
* Update Kubernetes dependency to 1.19.2 and sidecars to latest
  [[GH-341]](https://github.com/digitalocean/csi-digitalocean/pull/341)

## v2.0.0 - 2020.06.22

* Add snapshot migration tool
  [[GH-325]](https://github.com/digitalocean/csi-digitalocean/pull/325)
* Update csi-resizer sidecar to v0.5.0
  [[GH-324]](https://github.com/digitalocean/csi-digitalocean/pull/324)
* Support v1beta1 snapshots
  [[GH-322]](https://github.com/digitalocean/csi-digitalocean/pull/322)
* Upgrade Kubernetes dependencies to 1.18.3
  [[GH-318]](https://github.com/digitalocean/csi-digitalocean/pull/318)

## v1.3.0 - 2020.05.05

* Fix ListVolumes paging
  [[GH-310]](https://github.com/digitalocean/csi-digitalocean/pull/310)
* Upgrade Kubernetes dependencies to 1.17
  [[GH-307]](https://github.com/digitalocean/csi-digitalocean/pull/307)
* Add initContainer to delete automount udev rule file
  [[GH-305]](https://github.com/digitalocean/csi-digitalocean/pull/305)
* Handle per-node volume limit exceeding error during ControllerPublishVolume
  [[GH-303]](https://github.com/digitalocean/csi-digitalocean/pull/303)
* Build using Go 1.14
  [[GH-302]](https://github.com/digitalocean/csi-digitalocean/pull/302)
* Fix ListSnapshots paging
  [[GH-300]](https://github.com/digitalocean/csi-digitalocean/pull/300)
* Support filtering snapshots by ID
  [[GH-299]](https://github.com/digitalocean/csi-digitalocean/pull/299)
* Return minimum disk size field from snapshot response
  [[GH-298]](https://github.com/digitalocean/csi-digitalocean/pull/298)
* Improve debug HTTP server usage
  [[GH-281]](https://github.com/digitalocean/csi-digitalocean/pull/281)

## v1.2.0 - 2020.01.15

* Update csi-snapshotter to v1.2.2
  [[GH-266]](https://github.com/digitalocean/csi-digitalocean/pull/266)
* Support raw block volume mode
  [[GH-249]](https://github.com/digitalocean/csi-digitalocean/pull/249)
* Add support for running upstream storage end-to-end tests
  [[GH-248]](https://github.com/digitalocean/csi-digitalocean/pull/248)
* Include error details in failure to tag volumes
  [[GH-245]](https://github.com/digitalocean/csi-digitalocean/pull/245)
* Fix and improve logging
  [[GH-244]](https://github.com/digitalocean/csi-digitalocean/pull/244)
* Use WARN log level for non-critical failures to get an action
  [[GH-241]](https://github.com/digitalocean/csi-digitalocean/pull/241)
* Check all snapshots for existence
  [[GH-240]](https://github.com/digitalocean/csi-digitalocean/pull/240)
* Implement graceful shutdown
  [[GH-238]](https://github.com/digitalocean/csi-digitalocean/pull/238)
* Update sidecars
  [[GH-236]](https://github.com/digitalocean/csi-digitalocean/pull/236)
* Support checkLimit for multiple pages
  [[GH-235]](https://github.com/digitalocean/csi-digitalocean/pull/235)
* Log snapshot responses
  [[GH-234]](https://github.com/digitalocean/csi-digitalocean/pull/234)
* Return error when fetching the snapshot fails
  [[GH-233]](https://github.com/digitalocean/csi-digitalocean/pull/233)
* Refactor waitAction
  [[GH-229]](https://github.com/digitalocean/csi-digitalocean/pull/229)
* Assume detached state on 404 during ControllerUnpublishVolume
  [[GH-221]](https://github.com/digitalocean/csi-digitalocean/pull/221)
* Bump Kubernetes dependencies to 1.15
  [[GH-212]](https://github.com/digitalocean/csi-digitalocean/pull/212)
* Add health check endpoint
  [[GH-210]](https://github.com/digitalocean/csi-digitalocean/pull/210)
* Implement NodeGetVolumeStats RPC
  [[GH-197]](https://github.com/digitalocean/csi-digitalocean/pull/197)
* Build using Go 1.13
  [[GH-194]](https://github.com/digitalocean/csi-digitalocean/pull/194)
* Implement ControllerExpandVolume and NodeExpandVolume to resize volumes automatically
  [[GH-193]](https://github.com/digitalocean/csi-digitalocean/pull/193)

## v1.1.2 - 2019.09.17

* Improve error messages for incorrectly attached volumes
  [[GH-176]](https://github.com/digitalocean/csi-digitalocean/pull/176)
  [[GH-177]](https://github.com/digitalocean/csi-digitalocean/pull/177)
* Allow for custom driver names, to help with upgrades from Kubernetes 1.11
  [[GH-179]](https://github.com/digitalocean/csi-digitalocean/pull/179)
  [[GH-182]](https://github.com/digitalocean/csi-digitalocean/pull/182)

## v1.1.1 - 2019.07.02

* Set a custom user agent for the godo client. [[GH-156](https://github.com/digitalocean/csi-digitalocean/pull/156)]
* Include `xfsprogs` in the base Docker image so that XFS can be used

## v1.1.0 - 2019.04.29

**IMPORTANT**: This release is only compatible with Kubernetes **`v1.14.+`**

* Update CSI Spec to `v1.1.0`. This includes many new changes, make sure
  to read the Github PR for more information
  [[GH-144]](https://github.com/digitalocean/csi-digitalocean/pull/144)
* Update csi-test library to `v2.0.0`.
  [[GH-144]](https://github.com/digitalocean/csi-digitalocean/pull/144)
* Updated sidecars to the following versions:
  [[GH-144]](https://github.com/digitalocean/csi-digitalocean/pull/144)

```text
quay.io/k8scsi/csi-provisioner:v1.1.0
quay.io/k8scsi/csi-attacher:v1.1.1
quay.io/k8scsi/csi-snapshotter:v1.1.0
quay.io/k8scsi/csi-node-driver-registrar:v1.1.0
```

* Deprecated `cluster-driver-registrar` sidecar. The `CSIDriver` CRD is now
  part of Kubernetes since `v1.14.0`
  [[GH-144]](https://github.com/digitalocean/csi-digitalocean/pull/144)
* Added a new custom `CSIDriver` with the name `dobs.csi.digitalocean.com`
  [[GH-144]](https://github.com/digitalocean/csi-digitalocean/pull/144)
* Fix checking volume snapshot existency before proceeding creating a snapshot
  based volume
  [[GH-144]](https://github.com/digitalocean/csi-digitalocean/pull/144)
* Return proper error messages for unimplemented CSI methods
  [[GH-154]](https://github.com/digitalocean/csi-digitalocean/pull/154)

## v1.0.1 - 2019.04.25

* Add tagging support for Volumes via the new `--do-tag` flag
  [[GH-130]](https://github.com/digitalocean/csi-digitalocean/pull/130)
* Fix support for volume snapshots by setting snapshot id on volume creation
  [[GH-129]](https://github.com/digitalocean/csi-digitalocean/pull/129)
* Goreportcard fixes (typos, exported variables, etc..)
  [[GH-121]](https://github.com/digitalocean/csi-digitalocean/pull/121)
* Rename the cluster role bindings for the `node-driver-registrar` to be
  consistent with the other role bindings.
  [[GH-118]](https://github.com/digitalocean/csi-digitalocean/pull/118)
* Remove the `--token` flag for the `csi-do-node` driver. Drivers running on
  the node don't need the token anymore.
  [[GH-118]](https://github.com/digitalocean/csi-digitalocean/pull/118)
* Don't check the volume limits on the worker nodes (worker nodes are not able
  to talk to DigitalOcean API)
  [[GH-142]](https://github.com/digitalocean/csi-digitalocean/pull/142)
* Update `godo` (DigitalOcean API package) version to v1.13.0
  [[GH-143]](https://github.com/digitalocean/csi-digitalocean/pull/143)
* Fix race in snapshot integration test.
  [[GH-146]](https://github.com/digitalocean/csi-digitalocean/pull/146)
* Add tagging support for Volume snapshots via the new `--do-tag` flag
  [[GH-145]](https://github.com/digitalocean/csi-digitalocean/pull/145)

## v1.0.0 - 2018.12.19

* Add support for CSI SPEC `v1.0.0`. This includes various new changes and
  additions in the driver and is intended to be used with Kubernetes `v1.13+`
  [[GH-113]](https://github.com/digitalocean/csi-digitalocean/pull/113)
* Add priorityClassName to controller and node plugin to prevent the CSI
  components from being evicted in favor of user workloads
  [[GH-115]](https://github.com/digitalocean/csi-digitalocean/pull/115)

## v0.4.1 - 2019.04.26

* Cherry-pick: Add tagging support for Volumes via the new `--do-tag` flag
  [[GH-130]](https://github.com/digitalocean/csi-digitalocean/pull/130)
* Cherry-pick: Fix support for volume snapshots by setting snapshot id on volume creation
  [[GH-129]](https://github.com/digitalocean/csi-digitalocean/pull/129)
* Cherry-pick: Goreportcard fixes (typos, exported variables, etc..)
  [[GH-121]](https://github.com/digitalocean/csi-digitalocean/pull/121)
* Cherry-pick: Rename the cluster role bindings for the `node-driver-registrar` to be
  consistent with the other role bindings.
  [[GH-118]](https://github.com/digitalocean/csi-digitalocean/pull/118)
* Cherry-pick: Remove the `--token` flag for the `csi-do-node` driver. Drivers running on
  the node don't need the token anymore.
  [[GH-118]](https://github.com/digitalocean/csi-digitalocean/pull/118)
* Cherry-pick: Don't check the volume limits on the worker nodes (worker nodes are not able
  to talk to DigitalOcean API)
  [[GH-142]](https://github.com/digitalocean/csi-digitalocean/pull/142)
* Cherry-pick: Update `godo` (DigitalOcean API package) version to v1.13.0
  [[GH-143]](https://github.com/digitalocean/csi-digitalocean/pull/143)
* Cherry-pick: Fix race in snapshot integration test.
  [[GH-146]](https://github.com/digitalocean/csi-digitalocean/pull/146)
* Cherry-pick: Add tagging support for Volume snapshots via the new `--do-tag` flag
  [[GH-145]](https://github.com/digitalocean/csi-digitalocean/pull/145)

## v0.4.0 - 2018.11.26

* Add CSI Snapshots functionality
  [[GH-103]](https://github.com/digitalocean/csi-digitalocean/pull/103)
* Add csi-snapshotter sidecars and associated RBAC rules
  [[GH-104]](https://github.com/digitalocean/csi-digitalocean/pull/104)
* Add VolumeSnapshot CRD's to simplify the driver installation
  [[GH-108]](https://github.com/digitalocean/csi-digitalocean/pull/108)
* Revisit existing RBAC rules for the attacher, provisioner and
  driver-registrar. We no longer use the system cluster-role bindings as those
  will be deleted in v1.13
  [[GH-104]](https://github.com/digitalocean/csi-digitalocean/pull/104)
* Fix inconsistent usage of the driver name
  [[GH-100]](https://github.com/digitalocean/csi-digitalocean/pull/100)
* Use publish_info in ControllerPublishVolume for storing and accessing the
  volume name on Node plugins. This allows us to do all Node related operations
  without relying on the DO API.
  [[GH-99]](https://github.com/digitalocean/csi-digitalocean/pull/99)
* Improve creating volumes by validating the storage size requirements stricter
  and returning more human friendly errors.
  [[GH-101]](https://github.com/digitalocean/csi-digitalocean/pull/101)

## v0.3.1 - 2018.10.31

* Fix driver name in CSIDriver, StorageClass and GetNodeInfo()
  [[GH-96]](https://github.com/digitalocean/csi-digitalocean/pull/96)

## v0.3.0 - 2018.10.29

* This release is intended to be used with Kubernetes `v1.12.x` and is not compatible with older versions of Kubernetes. The latest CSI changes in v1.12.x are not compatible with older version unfortunately, therefore going forward we will not support older version anymore. The requirements also has changed, please make sure to read the README.md to see what kubelet and kube-apiserver flags needs to be enabled.
  [[GH-95]](https://github.com/digitalocean/csi-digitalocean/pull/95)
* Two new `CRD's` are installed: `CSINodeInfo` and `CSIDriver` to simplify node and driver discovery in Kubernetes.
* Add a [tutorial](examples/kubernetes/pod-single-existing-volume/README.md) on how to re-use an existing volume. Also a new option is introduced to prevent formatting an existing volume.
  [[GH-87]](https://github.com/digitalocean/csi-digitalocean/pull/87)
* Handle case if a volume is already attached to a droplet
  [[GH-87]](https://github.com/digitalocean/csi-digitalocean/pull/87)
* Switch to Go modules for dependency management
  [[GH-94]](https://github.com/digitalocean/csi-digitalocean/pull/94)

## v0.2.0 - 2018.09.05

* Add support to CSI Spec `v0.3.0`. This includes many new changes, make sure
  to read the Github PR for more information
  [[GH-72]](https://github.com/digitalocean/csi-digitalocean/pull/72)
* Check volume limits before provisioning calls
  [[GH-73]](https://github.com/digitalocean/csi-digitalocean/pull/73)
* Rename resource (DaemonSet, StatefulSet, containers, etc..) names and combine the
  attacher and provisioner into a single Statefulset.
  [[GH-74]](https://github.com/digitalocean/csi-digitalocean/pull/74)

**IMPORTANT**:This release contains breaking changes, mainly about how thing
are deployed. The minimum Kubernetes version needs to be now **v1.10.5**.
To upgrade from a prior `v0.1.x` versions please remove the old CSI plugin
completely and re-install the new one:

```sh
# delete old version, i.e: v0.1.5
kubectl delete -f https://raw.githubusercontent.com/digitalocean/csi-digitalocean/master/deploy/kubernetes/releases/csi-digitalocean-v0.1.5.yaml

# install v0.2.0
kubectl apply -f https://raw.githubusercontent.com/digitalocean/csi-digitalocean/master/deploy/kubernetes/releases/csi-digitalocean-v0.2.0.yaml
```

## v0.1.5 - 2018.08.27

* Makefile improvements. Please check the GH link for more information.
  [[GH-66]](https://github.com/digitalocean/csi-digitalocean/pull/66)
* Validate volume capabilities during volume creation.
  [[GH-68]](https://github.com/digitalocean/csi-digitalocean/pull/68)
* Add version information to logs
  [[GH-65]](https://github.com/digitalocean/csi-digitalocean/pull/65)

## v0.1.4 - 2018.08.23

* Add logs to mount operations
  [[GH-55]](https://github.com/digitalocean/csi-digitalocean/pull/55)
* Remove description to allow users to reuse volumes that were created by the
  UI/API
  [[GH-59]](https://github.com/digitalocean/csi-digitalocean/pull/59)
* Handle edge cases from external action, such as Volume deletion via UI more
  gracefully. We're not very strict anymore in cases we don't need to be, but
  we're also returning a better error in cases we need to be.
  [[GH-60]](https://github.com/digitalocean/csi-digitalocean/pull/60)
* Fix attaching multiple volumes to a single pod
  [[GH-61]](https://github.com/digitalocean/csi-digitalocean/pull/61)

## v0.1.3 - 2018.08.03

* Fix passing an empty source to `IsMounted()` function during `NodeUnpublish`.
  This would prevent a pod to be deleted successfully in case of a dettached
  volume, because `NodeUnpublish` would never return success as `IsMounted()`
  was failing.
  [[GH-50]](https://github.com/digitalocean/csi-digitalocean/pull/50)

## v0.1.2 - 2018.08.02

* Check if mounts are propagated (`MountPropagation` is enabled on the host) in
  Node plugin to prevent silent failing.
  [[GH-46]](https://github.com/digitalocean/csi-digitalocean/pull/46)
* Fix `IsMounted()` for bind mounts where it was returning false positives.
  [[GH-46]](https://github.com/digitalocean/csi-digitalocean/pull/46)
* Log 422 errors for visibility in Controller publish/unpublish methods.
  [[GH-38]](https://github.com/digitalocean/csi-digitalocean/pull/38)

## v0.1.1 (alpha) - 2018.05.29

* Fix panicking on errors for nil response objects
  [[GH-34]](https://github.com/digitalocean/csi-digitalocean/pull/34)

## v0.1.0 (alpha) - 2018.05.15

* Add method names to each log entry
  [[GH-22]](https://github.com/digitalocean/csi-digitalocean/pull/22)
* Kubernetes deployment uses the `kube-system` namespace instead of the prior
  `default` namespace. Please make sure to delete and re-deploy the plugin.
  [[GH-21]](https://github.com/digitalocean/csi-digitalocean/pull/21)
* Change secret name from `dotoken` to `digitalocean`. Please make sure to
  update your keys (delete old secret and create new secret with name
  `digitalocean`). Checkout the README for instructions if needed.
  [[GH-21]](https://github.com/digitalocean/csi-digitalocean/pull/21)
* Make DigitalOcean API configurable via the new `--url` flag
  [[GH-27]](https://github.com/digitalocean/csi-digitalocean/pull/27)

## v0.0.1 (alpha) - 2018.05.10

* First release with all important methods of the CSI spec implemented
