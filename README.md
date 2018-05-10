# csi-digitalocean [![Build Status](https://travis-ci.org/digitalocean/csi-digitalocean.svg?branch=master)](https://travis-ci.org/digitalocean/csi-digitalocean)
A Container Storage Interface ([CSI](https://github.com/container-storage-interface/spec)) Driver for DigitalOcean Block Storage. The CSI plugin allows you to use DigitalOcean Block Storage with your preferred Container Orchestrator.

The DigitalOcean CSI plugin is mostly tested on Kubernetes. Theoretically it
should also work on other Container Orchestrator's, such as Mesos or
Cloud Foundry. Feel free to test it on other CO's and give us a feedback.

## Installing to Kubernetes

**Requirements:**

* Kubernetes v1.10 minimum 
* `--allow-privileged` flag must be set to true for both the API server and the kubelet
* (if you use Docker) the Docker daemon of the cluster nodes must allow shared mounts

#### 1. Create a secret with your DigitalOcean API Access Token:

First encode your token in base64 (in this example, the string starting with
`a05...` is the API token):

```
$  echo -n "a05dd2f26b9b9ac2asdasdsd3fabf713560a129323890123cb5d1ec17513e06da" | base64
YTA1ZGQyZjI2YjliOWFjMmFzZGFzZHNkM2ZhYmY3MTM1NjBhMTI5MzIzODkwMTIzY2I1ZDFlYzE3NTEzZTA2ZGE=
```

Write a secret resource file with the base64 output above:

```
apiVersion: v1
kind: Secret
metadata:
  name: dotoken
type: Opaque
data:
  token: YTA1ZGQyZjI2YjliOWFjMmFzZGFzZHNkM2ZhYmY3MTM1NjBhMTI5MzIzODkwMTIzY2I1ZDFlYzE3NTEzZTA2ZGE=
```

and create the secret using kubectl:

```
$ kubectl create -f ./secret.yml
secret "dotoken" created
```

#### 2. Deploy the CSI plugin and sidecars:

Before you continue, be sure to checkout to a [tagged
release](https://github.com/digitalocean/csi-digitalocean/releases). For
example, to use the version `v0.0.1` you can execute the following command:

```
$ kubectl apply -f https://raw.githubusercontent.com/digitalocean/csi-digitalocean/master/deploy/kubernetes/releases/csi-digitalocean-v0.0.1.yaml
```

A new storage class will be created with the name `do-block-storage` which is
responsible for dynamic provisioning. This is set to **"default"** for dynamic
provisioning. If you're using multiple storage classes you might want to remove
the annotation from the `csi-storageclass.yaml` and re-deploy it. This is
based on the [recommended mechanism](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/storage/container-storage-interface.md#recommended-mechanism-for-deploying-csi-drivers-on-kubernetes) of deploying CSI drivers on Kubernetes

*Note that the deployment proposal to Kubernetes is still a work in progress and not all of the written
features are implemented. When in doubt, open an issue or ask #sig-storage in [Kubernetes Slack](http://slack.k8s.io)*

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

After that create a Pod that refers to this volume. When the Pod is created, the volume will be attached, formatted and mounted to the specified Container

```
kind: Pod
apiVersion: v1
metadata:
  name: my-csi-app
spec:
  nodeName: "nodes-2"
  containers:
    - name: my-frontend
      image: busybox
      volumeMounts:
      - mountPath: "/data"
        name: my-csi-volume
      command: [ "sleep", "1000000" ]
  volumes:
    - name: my-csi-volume
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

## Contributing
At DigitalOcean we value and love our community! If you have any issues or
would like to contribute, feel free to open an issue/PR
