# csi-digitalocean [![Build Status](https://travis-ci.org/digitalocean/csi-digitalocean.svg?branch=master)](https://travis-ci.org/digitalocean/csi-digitalocean)
A Container Storage Interface ([CSI](https://github.com/container-storage-interface/spec)) Driver for DigitalOcean Block Storage. The CSI plugin allows you to use DigitalOcean Block Storage with your preferred Container Orchestrator.

The DigitalOcean CSI plugin is mostly tested on Kubernetes. Theoretically it
should also work on other Container Orchestrator's, such as Mesos or
Cloud Foundry. Feel free to test it on other CO's and give us a feedback.

## Releases

The DigitalOcean CSI plugin follows [semantic versioning](https://semver.org/).
The current version is: **`v0.4.3`**. This means that the project is still
under active development and may not be production ready. The plugin will be
bumped to **`v1.0.0`** once the [DigitalOcean Kubernetes
product](https://www.digitalocean.com/products/kubernetes/) is released and
will continue following the rules below:

* Bug fixes will be released as a `PATCH` update.
* New features (such as CSI spec bumps) will be released as a `MINOR` update.
* Significant breaking changes makes a `MAJOR` update.


## Installing to Kubernetes

### Kubernetes Compatibility

| DigitalOcean CSI Driver\Kubernetes Version | 1.10.5 - 1.11 | 1.12+ | 1.13+ | 
|--------------------------------------|---------------|------|-------|
| v0.1.0 - v0.2.x                      | yes           | no   | no    |
| v0.3.0 - v0.4.x                      | no            | yes  | no    |
| dev                                  | no            | no   | no    |


Note: The [`DigitalOcean Kubernetes`](https://www.digitalocean.com/products/kubernetes/) products comes
with the CSI driver pre-installed and no further steps are required.

**Requirements:**

* `--allow-privileged` flag must be set to true for both the API server and the kubelet
* `--feature-gates=VolumeSnapshotDataSource=true,KubeletPluginsWatcher=true,CSINodeInfo=true,CSIDriverRegistry=true` feature gate flags must be set to true for both the API server and the kubelet
* Mount Propagation needs to be enabled. If you use Docker, the Docker daemon of the cluster nodes must allow shared mounts.


#### 1. Create a secret with your DigitalOcean API Access Token:

Replace the placeholder string starting with `a05...` with your own secret and
save it as `secret.yml`: 

```
apiVersion: v1
kind: Secret
metadata:
  name: digitalocean
  namespace: kube-system
stringData:
  access-token: "a05dd2f26b9b9ac2asdas__REPLACE_ME____123cb5d1ec17513e06da"
```

and create the secret using kubectl:

```
$ kubectl create -f ./secret.yml
secret "digitalocean" created
```

You should now see the digitalocean secret in the `kube-system` namespace along with other secrets

```
$ kubectl -n kube-system get secrets
NAME                  TYPE                                  DATA      AGE
default-token-jskxx   kubernetes.io/service-account-token   3         18h
digitalocean          Opaque                                1         18h
```

#### 2. Deploy the CSI plugin and sidecars:

Before you continue, be sure to checkout to a [tagged
release](https://github.com/digitalocean/csi-digitalocean/releases). Always use the [latest stable version](https://github.com/digitalocean/csi-digitalocean/releases/latest) 
For example, to use the latest stable version (`v0.4.3`) you can execute the following command:

```
$ kubectl apply -f https://raw.githubusercontent.com/digitalocean/csi-digitalocean/master/deploy/kubernetes/releases/csi-digitalocean-v0.4.3.yaml
```

This file will be always updated to point to the latest stable release. If you
see any issues during the installation, this could be because the newly created
CRD's haven't been established yet. If you call `kubectl apply -f` again on the
same file, the missing resources will be applied again


#### 3. Test and verify:

Create a PersistentVolumeClaim. This makes sure a volume is created and provisioned on your behalf:

```
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: csi-pvc
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 5Gi
  storageClassName: do-block-storage
```

Check that a new `PersistentVolume` is created based on your claim:

```
$ kubectl get pv
NAME                                       CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS    CLAIM             STORAGECLASS       REASON    AGE
pvc-0879b207-9558-11e8-b6b4-5218f75c62b9   5Gi        RWO            Delete           Bound     default/csi-pvc   do-block-storage             3m
```

The above output means that the CSI plugin successfully created (provisioned) a
new Volume on behalf of you. You should be able to see this newly created
volume under the [Volumes tab in the DigitalOcean UI](https://cloud.digitalocean.com/droplets/volumes)

The volume is not attached to any node yet. It'll only attached to a node if a
workload (i.e: pod) is scheduled to a specific node. Now let us create a Pod
that refers to the above volume. When the Pod is created, the volume will be
attached, formatted and mounted to the specified Container:

```
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
        claimName: csi-pvc 
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

## Upgrading

When upgrading to a new Kubernetes minor version, you should upgrade the CSI
driver to match. See the table above for which driver version is used with each
Kubernetes version.

Special consideration is necessary when upgrading from Kubernetes 1.11 or
earlier, which uses CSI driver version 0.2 or earlier. In these early releases,
the driver name was `com.digitalocean.csi.dobs`, while in all subsequent
releases it is `dobs.csi.digitalocean.com`. When upgrading, use the commandline
flag `--driver-name` to force the new driver to use the old name. Failing to do
so will cause any existing PVs to be unusable since the new driver will not
manage them and the old driver is no longer running.

## Development

Requirements:

* Go: min `v1.11.x`

After making your changes, run the unit tests: 

```
$ make test
```

If you want to test your changes, create a new image with the version set to `dev`:

```
$ VERSION=dev make publish
```

This will create a binary with version `dev` and docker image pushed to
`digitalocean/do-csi-plugin:dev`


To run the integration tests run the following:

```
$ KUBECONFIG=$(pwd)/kubeconfig make test-integration
```

Dependencies are managed via [Go modules](https://github.com/golang/go/wiki/Modules).

### Release a new version

To release a new version bump first the version:

```
$ make NEW_VERSION=v1.0.0 bump-version
```

Make sure everything looks good. Create a new branch with all changes:

```
$ git checkout -b new-release
$ git add .
$ git push origin
```

After it's merged to master, [create a new Github
release](https://github.com/digitalocean/csi-digitalocean/releases/new) from
master with the version `v0.4.3` and then publish a new docker build:

```
$ git checkout master
$ make publish
```

This will create a binary with version `v0.4.3` and docker image pushed to
`digitalocean/do-csi-plugin:v0.4.3`

## Contributing

At DigitalOcean we value and love our community! If you have any issues or
would like to contribute, feel free to open an issue/PR
