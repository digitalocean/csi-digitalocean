# Use an existing volume

Below you will find the instruction on how to use an existing DigitalOcean Block
Storage with your Kubernetes cluster.

## Known issues

* Deleting a `PVC` will delete the volume if the reclaim policy is set
to `Delete`, however the bound `PV` will be not deleted. This seems to be a
bug in K8S.
* Using an existing, attached volume might cause confusion if the `POD` is
scheduled to a different node. It's advised to use only existing **detached**
volumes. If you have an attached volume, please make sure to detach it, so
Kubernetes attaches it to the correct droplet. The CSI plugin will return an
error if the volume is attached to a wrong droplet.

## Example

To use an existing volume, we have to create manually a `PersistentVolume` (PV)
resource. Here is an example `PersistenVolume` resource for an existing volume:

```yaml
kind: PersistentVolume
apiVersion: v1
metadata:
  name: volume-nyc1-01
  annotations:
    # fake it by indicating this is provisioned dynamically, so the system
    # works properly
    pv.kubernetes.io/provisioned-by: dobs.csi.digitalocean.com
spec:
  storageClassName: do-block-storage
  # by default, the volume will be not deleted if you delete the PVC, change to
  # "Delete" if you wish the volume to be deleted automatically with the PVC
  persistentVolumeReclaimPolicy: Delete
  capacity:
    storage: 5Gi
  accessModes:
    - ReadWriteOnce
  csi:
    driver: dobs.csi.digitalocean.com
    fsType: ext4
    volumeHandle: 1952d58a-c714-11e8-bc0c-0a58ac14421e
    volumeAttributes:
      com.digitalocean.csi/noformat: "true"
```

Couple of things to note,

* `volumeHandle` is the volume ID you want to reuse. Make sure it matches exactly the volume you're targeting. You can list the ID's of your volumes via doctl: `doctl compute volume list`
* `volumeAttributes` has a special, csi-digitalocean specific annotation called `com.digitalocean.csi/noformat`. If you add this key, the CSI plugin makes sure to **not format** the volume. If you don't add this, it'll be formatted.
* `storage` make sure it's set to the same storage size as your existing DigitalOcean Block Storage volume.

Create a file with this content, naming it `pv.yaml` and deploying it:

```
kubectl create -f pv.yaml
```

View information about the `PersistentVolume`:

```
$ kubectl get pv volume-nyc1-01

NAME             CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS      CLAIM     STORAGECLASS       REASON    AGE
volume-nyc1-01   5Gi        RWO            Delete           Available             do-block-storage             15s
```

The status is `Available`. This means it has not yet been bound to a
PersistentVolumeClaim. Now we can proceed to create our PVC:


```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: csi-pod-pvc
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 5Gi
  storageClassName: do-block-storage
```

This is the same (just like our other examples). When you create `PVC`,
Kubernetes will try to match it to an existing `PV`. Because the
storageClassName and the storage size matches our `PV` descriptions, kubernetes
will bind this `PVC` to our manually create `PV`. CSI **will not** create a new
volume because of the existing `PV`.

Create the PersistentVolumeClaim:

```
kubectl create -f pvc.yaml
```

Now look at the PersistentVolumeClaim (PVC):

```
kubectl get pvc task-pv-claim
NAME          STATUS    VOLUME           CAPACITY   ACCESS MODES   STORAGECLASS       AGE
csi-pod-pvc   Bound     volume-nyc1-01   5Gi        RWO            do-block-storage   5s
```

As you see, the output shows that the PVC is bound to our PersistentVolume, `volume-nyc1-01`.

Finally, define your pod that refers to this PVC:

```yaml
kind: Pod
apiVersion: v1
metadata:
  name: my-csi-app
spec:
  containers:
    - name: my-frontend
      image: busybox
      volumeMounts:
      - mountPath: "/data"
        name: my-do-volume
      command: [ "sleep", "1000000" ]
  volumes:
    - name: my-do-volume
      persistentVolumeClaim:
        claimName: csi-pod-pvc 
```


Check if the pod is running successfully:


```
$ kubectl describe pods/my-csi-app
```

Write inside the app container:

```
$ kubectl exec -ti my-csi-app /bin/sh
/ # touch /data/hello-world
/ # exit
$ kubectl exec -ti my-csi-app /bin/sh
/ # ls /data
hello-world
```
