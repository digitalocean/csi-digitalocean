# csi-digitalocean

![](https://github.com/digitalocean/csi-digitalocean/workflows/test/badge.svg)

A Container Storage Interface ([CSI](https://github.com/container-storage-interface/spec)) Driver for [DigitalOcean Block Storage](https://www.digitalocean.com/docs/volumes/). The CSI plugin allows you to use DigitalOcean Block Storage with your preferred Container Orchestrator.

The DigitalOcean CSI plugin is mostly tested on Kubernetes. Theoretically it
should also work on other Container Orchestrators, such as Mesos or
Cloud Foundry. Feel free to test it on other CO's and give us a feedback.

## Releases

The DigitalOcean CSI plugin follows [semantic versioning](https://semver.org/).
The version will be bumped following the rules below:

* Bug fixes will be released as a `PATCH` update.
* New features (such as CSI spec bumps with no breaking changes) will be released as a `MINOR` update.
* Significant breaking changes makes a `MAJOR` update.

## Features

Below is a list of functionality implemented by the plugin. In general, [CSI features](https://kubernetes-csi.github.io/docs/features.html) implementing an aspect of the [specification](https://github.com/container-storage-interface/spec/blob/master/spec.md) are available on any DigitalOcean Kubernetes version for which beta support for the feature is provided.

See also the [project examples](/examples/kubernetes) for use cases.

### Volume Expansion

Volumes can be expanded by updating the storage request value of the corresponding PVC:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: csi-pvc
  namespace: default
spec:
  [...]
  resources:
    requests:
      # The field below can be increased.
      storage: 10Gi
      [...]
```

After successful expansion, the _status_ section of the PVC object will reflect the actual volume capacity.

Important notes:

* Volumes can only be increased in size, not decreased; attempts to do so will lead to an error.
* Expanding a volume that is larger than the target size will have no effect. The PVC object status section will continue to represent the actual volume capacity.
* Resizing volumes other than through the PVC object (e.g., the DigitalOcean cloud control panel) is not recommended as this can potentially cause conflicts. Additionally, size updates will not be reflected in the PVC object status section immediately, and the section will eventually show the actual volume capacity.

### Raw Block Volume

Volumes can be used in raw block device mode by setting the `volumeMode` on the corresponding PVC:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: csi-pvc
  namespace: default
spec:
  [...]
  volumeMode: Block
```

Important notes:

* If using volume expansion functionality, only expansion of the underlying persistent volume is guaranteed. We do not guarantee to automatically
expand the filesystem if you have formatted the device.

### Volume Snapshots

Snapshots can be created and restored through `VolumeSnapshot` objects.

---
**Note:**

Since version 2, the CSI plugin support v1beta1 Volume Snapshots only. Support for the v1alpha1 has been dropped.

Users that want to migrate their v1alpha1 Volume Snapshots into a v1beta1 cluster can leverage [this migration tool](/cmd/migrate-snapshots). (For DOKS customers, the migration will be applied automatically during cluster upgrades.)

---

See also [the example](/examples/kubernetes/snapshot).

### Volume Statistics

Volume statistics are exposed through the CSI-conformant endpoints. Monitoring systems such as Prometheus can scrape metrics and provide insights into volume usage.

### Volume Transfer

Volumes can be transferred across clusters. The exact steps are outlined in [our example](/examples/kubernetes/pod-single-existing-volume).

## Installing to Kubernetes

### Kubernetes Compatibility

The following table describes the required DigitalOcean CSI driver version per Kubernetes release.

Kubernetes Release | DigitalOcean CSI Driver Version
------------------ | -------------------------------
1.10 (1.10.5+)     | v0.2.x
1.11               | v0.2.x
1.12               | v0.4.x
1.13               | v1.0.x
1.14               | v1.3.x
1.15               | v1.3.x
1.16               | v1.3.x
1.17               | v2.0.x (v1.3.x with v1alpha1 snapshots only)
1.18               | v2.0.x (v1.3.x with v1alpha1 snapshots only)
1.19               | v2.0.x (v1.3.x with v1alpha1 snapshots only)

---
**Note:**

The [DigitalOcean Kubernetes](https://www.digitalocean.com/products/kubernetes/) product comes with the CSI driver pre-installed and no further steps are required.

---
**Driver modes:**

By default, driver runs in both controller and node mode. It would create disk Volumes on DigitalOcean infrastructure and mount them on the required node.

Driver can be run in **controller only mode** outside DigitalOcean droplets.
To use this mode `--region` flag (valid DigitalOcean region slug) must be provided together with `--token` flag (DigitalOcean API token).

Alternatively driver can be run in **node only mode** on DigitalOcean droplets. Driver would only handle node related requests like mount volume.
To us this mode `--region` and `--token` flags must not be provided.

Skip secret creation (section 1. in following deployment instructions) when using **node only mode** as API token is not required.

---

**Requirements:**

* `--allow-privileged` flag must be set to true for both the API server and the kubelet
* `--feature-gates=KubeletPluginsWatcher=true,CSINodeInfo=true,CSIDriverRegistry=true` feature gate flags must be set to true for both the API server and the kubelet
* Mount Propagation needs to be enabled. If you use Docker, the Docker daemon of the cluster nodes must allow shared mounts.

#### 1. Create a secret with your DigitalOcean API Access Token

Replace the placeholder string starting with `a05...` with your own secret and
save it as `secret.yml`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: digitalocean
  namespace: kube-system
stringData:
  access-token: "a05dd2f26b9b9ac2asdas__REPLACE_ME____123cb5d1ec17513e06da"
```

and create the secret using kubectl:

```shell
$ kubectl create -f ./secret.yml
secret "digitalocean" created
```

You should now see the digitalocean secret in the `kube-system` namespace along with other secrets

```shell
$ kubectl -n kube-system get secrets
NAME                  TYPE                                  DATA      AGE
default-token-jskxx   kubernetes.io/service-account-token   3         18h
digitalocean          Opaque                                1         18h
```

#### 2. Deploy the CSI plugin and sidecars

Always use the [latest release](https://github.com/digitalocean/csi-digitalocean/releases) compatible with your Kubernetes release (see the [compatibility information](#kubernetes-compatibility)).

The [releases directory](deploy/kubernetes/releases) holds manifests for all plugin releases. You can deploy a specific version by executing the command

```shell
# Do *not* add a blank space after -f
kubectl apply -fhttps://raw.githubusercontent.com/digitalocean/csi-digitalocean/master/deploy/kubernetes/releases/csi-digitalocean-vX.Y.Z/{crds.yaml,driver.yaml,snapshot-controller.yaml}
```

where `vX.Y.Z` is the plugin target version. (Note that for releases older than v2.0.0, the driver was contained in a single YAML file. If you'd like to deploy an older release you need to use `kubectl apply -fhttps://raw.githubusercontent.com/digitalocean/csi-digitalocean/master/deploy/kubernetes/releases/csi-digitalocean-vX.Y.Z.yaml`)

If you see any issues during the installation, this could be because the newly
created CRDs haven't been established yet. If you call `kubectl apply -f` again
on the same file, the missing resources will be applied again.

#### 3. Test and verify

Create a PersistentVolumeClaim. This makes sure a volume is created and provisioned on your behalf:

```yaml
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

```shell
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
        claimName: csi-pvc
```

Check if the pod is running successfully:

```shell
kubectl describe pods/my-csi-app
```

Write inside the app container:

```shell
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

* Go at the version specified in `.github/workflows/test.yaml`
* Docker (for building via the Makefile, post-unit testing, and publishing)

Dependencies are managed via [Go modules](https://github.com/golang/go/wiki/Modules).

PRs from the code-hosting repository are automatically unit- and end-to-end-tested in our CI (implemented by Github Actions). See the [.github/workflows directory](.github/workflows) for details.

For every green build of the master branch, the container image `digitalocean/do-csi-plugin:master` is updated and pushed at the end of the CI run. This allows to test the latest commit easily.

Steps to run the tests manually are outlined below.

### Unit Tests

To execute the unit tests locally, run:

```shell
make test
```

### End-to-End Tests

To manually run the end-to-end tests, you need to build a container image for your change first and publish it to a registry. Repository owners can publish under `digitalocean/do-csi-plugin:dev`:

```shell
VERSION=dev make publish
```

If you do not have write permissions to `digitalocean/do-csi-plugin` on Docker Hub or are worried about conflicting usage of that tag, you can also publish under a different (presumably personal) organization:

```shell
DOCKER_REPO=johndoe VERSION=latest-feature make publish
```

This would yield the published container image `johndoe/do-csi-plugin:latest-feature`.

Assuming you have your DO API token assigned to the `DIGITALOCEAN_ACCESS_TOKEN` environment variable, you can then spin up a DOKS cluster on-the-fly and execute the upstream end-to-end tests for a given set of Kubernetes versions like this:

```shell
make test-e2e E2E_ARGS="-driver-image johndoe/do-csi-plugin:latest-feature 1.16 1.15 1.14"
```

See [our documentation](test/e2e/README.md) for an overview on how the end-to-end tests work as well as usage instructions.

### Integration Tests

There is a set of custom integration tests which are mostly useful for Kubernetes pre-1.14 installations as these are not covered by the upstream end-to-end tests.

To run the integration tests on a DOKS cluster, follow [the instructions](test/kubernetes/deploy/README.md).

## Updating the Kubernetes dependencies

Run

```shell
make NEW_KUBERNETES_VERSION=X.Y.Z update-k8s
```

to update the Kubernetes dependencies to version X.Y.Z.

## Releasing

To release a new version `vX.Y.Z`, first bump the version:

```shell
make NEW_VERSION=vX.Y.Z bump-version
```

This will create the set of files specific to a new release. Make sure everything looks good; in particular, ensure that the change log is up-to-date and is not missing any important, user-facing changes.

Create a new branch with all changes:

```shell
git checkout -b prepare-release-vX.Y.Z
git add .
git push origin
```

After it is merged to master, wait for the master build to go green. (This will entail another run of the entire test suite.)

Finally, check out the master branch again, tag the release, and push it:

```shell
git checkout master
git pull
git tag vX.Y.Z
git push origin vX.Y.Z
```

The CI will publish the container image `digitalocean/do-csi-plugin:vX.Y.Z` and create a Github Release under the name `vX.Y.Z` automatically. Nothing else needs to be done.

## Contributing

At DigitalOcean we value and love our community! If you have any issues or would like to contribute, feel free to open an issue or PR.
