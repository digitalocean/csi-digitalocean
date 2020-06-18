# migrate-snapshots

_tl;dr: for DigitalOcean Kubernetes Service (DOKS) users: you can ignore the instructions and the tool below -- DigitalOcean takes care of migrating your snapshots when upgrading your clusters from 1.17 to 1.18._

`migrate-snapshots` is a small tool that allows to migrate existing snapshots from the v1alpha1 to the v1beta1 API. This is necessary because no official migration path exists by the Kubernetes project.

The tool is focused on DigitalOcean's Block Storage snapshot driver only: snapshot objects belonging to a different `VolumeSnapshotClass` will be ignored.

`migrate-snapshots` is meant to be run against a cluster that still runs on v1alpha1 snapshots. After connecting to the cluster, it will iterate over all `VolumeSnapshotContent` and `VolumeSnapshot` objects and convert them to their respective v1beta1 structures. The converted objects are stored as YAML manifest files below a directory of choice.

It is assumed that the snapshots in the DigitalOcean storage system won't be deleted even if the snapshots objects in the Kubernetes cluster are, which can be achieved by changing the `deletionPolicy` on all `VolumeSnapshotContent` objects to `Retain`. Afterwards, the snapshots can be reimported by applying the YAML manifests to the upgraded or new cluster.

By default, `migrate-snapshots` reads out the `$HOME/.kube/config` file. A custom kubeconfig can be specified through the `KUBECONFIG` environment variable.

## Steps

Here are the proposed steps:

1. Run `migrate-snapshots` without the `-directory` parameter to see which objects would be persisted.
1. Run `migrate-snapshots -directory snapshot_objects` to store all snapshot-related objects below the `snapshot_objects` directory.
1. (optional) Change the deletion policy of the DigitalOcean Block Storage `VolumeSnapshotClass` to `Retain` to ensure that newly created snapshots are not removed: `kubectl patch volumesnapshotclass do-block-storage --patch 'deletionPolicy: Retain' --type merge`
1. Set all existing `VolumeSnapshotContent` objects to retain the snapshots on deletion: `kubectl get volumesnapshotcontent -o name --no-headers | xargs -n 1 kubectl patch --patch '{ "spec": { "deletionPolicy": "Retain" } }'`
1. Upgrade / re-create the cluster.
1. Ensure that all v1alpha1 snapshot CRDs and associated snapshot objects are removed. (It is not possible to run them concurrently with the v1beta1 objects.)
1. Ensure that the v1beta1 CRDs and DigitalOcean Block Storage `VolumeSnapshotClass` exist.
1. Re-import the previous snapshots: `kubectl apply -f snapshot_objects`.

## Limitations

No object metadata other than the name and namespace will be transferred over from v1alpha1 to v1beta1.
