### What did you do? (required. The issue will be **closed** when not provided.)


### What did you expect to happen?


### Configuration (**MUST** fill this out):

* system logs:

Please provide the following logs:

```
kubectl logs -l app=csi-provisioner-doplugin -c digitalocean-csi-plugin -n kube-system  > csi-provisioner.log
kubectl logs -l app=csi-attacher-doplugin -c digitalocean-csi-plugin -n kube-system > csi-attacher.log
kubectl logs -l app=csi-doplugin -c csi-doplugin -n kube-system > csi-nodeplugin.log
```

* manifests, such as pvc, deployments, etc.. you used to reproduce:

Please provide the **total** set of manifests that is needed to reproduce the
issue. Just providing the `pvc` is not helpful. If you cannot provide it due
privacy concerns, please try creating a reproducible case.


* CSI Version:

* Kubernetes Version:

* Cloud provider/framework version, if applicable (such as Rancher):


Use a single gist via https://gist.github.com/ if your logs and manifests are
long, otherwise it's OK to paste it here.
