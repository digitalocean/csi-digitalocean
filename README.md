# csi-digitalocean [![Build Status](https://travis-ci.org/digitalocean/csi-digitalocean.svg?branch=master)](https://travis-ci.org/digitalocean/csi-digitalocean)
A Container Storage Interface ([CSI](https://github.com/container-storage-interface/spec)) Driver for DigitalOcean Block Storage. The CSI plugin allows you to use DigitalOcean Block Storage with your preferred Container Orchestrator.

The DigitalOcean CSI plugin is mostly tested on Kubernetes. Theoretically it
should also work on other Container Orchestrators, such as Mesos or
Cloud Foundry. Feel free to test it on other CO's and give us a feedback.

## Releases

The DigitalOcean CSI plugin follows [semantic versioning](https://semver.org/).
The current version is: **`v1.1.1`**. The project is still
under active development and may not be production ready. The plugin will be
bumped following the rules below:

* Bug fixes will be released as a `PATCH` update.
* New features (such as CSI spec bumps with no breaking changes) will be released as a `MINOR` update.
* Significant breaking changes makes a `MAJOR` update.


## Installing to Kubernetes

### Kubernetes Compatibility

<table>
  <thead>
    <tr>
      <th></th>
      <th colspan=4>Kubernetes Version</th>
    </tr>
    <tr>
      <th>DigitalOcean CSI Driver</th>
      <th>1.10.5 - 1.11</th>
      <th>1.12+</th>
      <th>1.13+</th>
      <th>1.14+</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>v0.1.x - v0.2.x</td>
      <td>yes</td>
      <td>no</td>
      <td>no</td>
      <td>no</td>
    </tr>
    <tr>
      <td>v0.3.x - v0.4.x</td>
      <td>no</td>
      <td>yes</td>
      <td>no</td>
      <td>no</td>
    </tr>
    <tr>
      <td>v1.0.x - v1.0.x</td>
      <td>no</td>
      <td>no</td>
      <td>yes</td>
      <td>no</td>
    </tr>
    <tr>
      <td>v1.1.1 - v1.1.x</td>
      <td>no</td>
      <td>no</td>
      <td>no</td>
      <td>yes</td>
    </tr>
    <tr>
      <td>dev</td>
      <td>no</td>
      <td>no</td>
      <td>no</td>
      <td>yes</td>
    </tr>
  </tbody>
</table>

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
For example, to use the latest stable version (`v1.1.1`) you can execute the following command:

```
$ kubectl apply -f https://raw.githubusercontent.com/digitalocean/csi-digitalocean/master/deploy/kubernetes/releases/csi-digitalocean-v1.1.1.yaml
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
$ make NEW_VERSION=v1.1.1 bump-version
```

Make sure everything looks good. Create a new branch with all changes:

```
$ git checkout -b new-release
$ git add .
$ git push origin
```

After it's merged to master, [create a new Github
release](https://github.com/digitalocean/csi-digitalocean/releases/new) from
master with the version `v1.1.1` and then publish a new docker build:

```
$ git checkout master
$ make publish
```

This will create a binary with version `v1.1.1` and docker image pushed to
`digitalocean/do-csi-plugin:v1.1.1`

## Contributing

At DigitalOcean we value and love our community! If you have any issues or
would like to contribute, feel free to open an issue/PR
