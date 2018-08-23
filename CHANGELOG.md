## unreleased

* Makefile improvements. Please check the GH link for more information.
  [[GH-66]](https://github.com/digitalocean/csi-digitalocean/pull/66)
* Validate volume capabilities during volume creation.
  [[GH-68]](https://github.com/digitalocean/csi-digitalocean/pull/68)

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
