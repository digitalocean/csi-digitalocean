# Importing an existing snapshot into the cluster

It is possible to import an existing snapshot that was created out-of-band (i.e., via the DigitalOcean cloud control panel or the API) into a cluster. This guide outlines the necessary steps.

It is based on the [official Kubernetes blog post introducing volume snapshots](https://kubernetes.io/blog/2018/10/09/introducing-volume-snapshot-alpha-for-kubernetes/).

## Prequisites

You need the ID of the snapshot that you want to import. It can be discovered via `doctl compute snapshot list` or the [API equivalent](https://developers.digitalocean.com/documentation/v2/#list-all-volume-snapshots) (but not via the Cloud control panel at this time of writing). For the sake of this guide, we will assume the snapshot ID to be `92b46522-4798-4544-9d33-8b74ea56adb7`.

## Steps

First we need to manually create a `VolumeSnapshotContent` (which can be viewed as the snapshot equivalent of a `PersistentVolume`). It should reference the snapshot ID in the `snapshotHandle` field:

```yaml
apiVersion: snapshot.storage.k8s.io/v1alpha1
kind: VolumeSnapshotContent
metadata:
  name: snapshotcontent-manual
spec:
  csiVolumeSnapshotSource:
    driver: dobs.csi.digitalocean.com
    snapshotHandle: 92b46522-4798-4544-9d33-8b74ea56adb7
  deletionPolicy: Retain
  volumeSnapshotRef:
    apiVersion: snapshot.storage.k8s.io/v1alpha1
    kind: VolumeSnapshot
    name: snapshot-manual
    namespace: default
```

Note how the `VolumeSnapshotContent` also references a `VolumeSnapshot` under `volumeSnapshotRef`. The `name` is arbitrary but must match the `VolumeSnapshot` we are going to create later.

Setting the `deletionPolicy` to `Retain` means that the the VolumeSnapshotContent -- and, transitively, the actual DigitalOcean snapshot resource -- will not be deleted when the VolumeSnapshot is deleted. Change the value to `Delete` if you would rather want automatic cleanup to happen.

Apply the `VolumeSnapshotContent`:

```shell
kubectl apply -f snapshotcontent.yaml
```

Just like a `PersistentVolume` needs a `PersistentVolumeClaim`, we similarly need to create a `VolumeSnapshot`:

```yaml
apiVersion: snapshot.storage.k8s.io/v1alpha1
kind: VolumeSnapshot
metadata:
  name: snapshot-manual
spec:
  snapshotContentName: snapshotcontent-manual
  snapshotClassName: do-block-storage
```

Make sure the name references between the `VolumeSnapshot` and the previously created `VolumeSnapshotContent` line up correctly.

Apply the `VolumeSnapshot`:

```shell
kubectl apply -f snapshot.yaml
```

Wait for snapshot to be imported successfully. The process completes once the `VolumeSnapshot`'s `status.readyToUse` field is set to `true`. Watch out for any events on the `VolumeSnapshotContent` and `VolumeSnapshot` that may surface errors or misconfigurations.

Once the snapshot is ready, you can source it from a regular `PersistentVolumeClaim`:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: pvc-manual-restore
spec:
  dataSource:
    name: snapshot-manual
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 5Gi
```

Note how `dataSource.name` references the previously created `VolumeSnapshot`.

Apply the `PersistentVolumeClaim`:

```shell
kubectl apply -f pvc.yaml
```

The created `PersistentVolumeClaim` will dynamically create a `PersistentVolume`.

The imported snapshot is now ready to be used as a volume in your workload.
