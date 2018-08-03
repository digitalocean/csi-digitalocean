## unplanned

* Fix passing an empty source to `IsMounted()` function during `NodeUnpublish`.
  This would prevent a pod to be deleted successfully in case of a dettached
  volume, because `NodeUnpublish` would never return success as `IsMounted()`
  was failing.
  [[GH-50]](https://github.com/digitalocean/csi-digitalocean/pull/50)

## v0.1.2 - August 2nd 2018

* Check if mounts are propagated (`MountPropagation` is enabled on the host) in
  Node plugin to prevent silent failing. 
  [[GH-46]](https://github.com/digitalocean/csi-digitalocean/pull/46)
* Fix `IsMounted()` for bind mounts where it was returning false positives.
  [[GH-46]](https://github.com/digitalocean/csi-digitalocean/pull/46)
* Log 422 errors for visibility in Controller publish/unpublish methods.
  [[GH-38]](https://github.com/digitalocean/csi-digitalocean/pull/38)

## v0.1.1 (alpha) - May 29th 2018

* Fix panicking on errors for nil response objects
  [[GH-34]](https://github.com/digitalocean/csi-digitalocean/pull/34)

## v0.1.0 (alpha) - May 15th 2018

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

## v0.0.1 (alpha) - May 10th 2018

* First release with all important methods of the CSI spec implemented
