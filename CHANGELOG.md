## unreleased\n
## v0.4.2 - 2019.07.02

* Cherry-pick: Set a custom user agent for the godo client.
  [[GH-156]](https://github.com/digitalocean/csi-digitalocean/pull/156)

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
