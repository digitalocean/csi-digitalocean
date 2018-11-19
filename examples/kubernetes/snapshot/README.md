# Creating a snapshot from an exiting volume and restore it back

Note that we assume you correctly installed the csi-digitalocean driver, and
it's up and running. 


1. Create a `pvc`:


```
$ kubectl create -f pvc.yaml
```

2. Create a `snapshot` from the previous `pvc`:


```
$ kubectl create -f snapshot.yaml
```

At this point you should have a volume and a snapshot originating from that
volume. You can observe the state of your pvc's and snapshots with the
following command:


```
$ kubectl get pvc && kubectl get pv && kubectl get volumesnapshot
```


3. Restore from a `snapshot`:

To restore from a given snapshot, you need to create a new `pvc` that refers to
the snapshot:


```
$ kubectl create -f restore.yaml
```

This will create a new `pvc` that you can use with your applications.

4. Cleanup your resources:

Make sure to delete your test resources:

```
$ kubectl delete -f pvc.yaml 
$ kubectl delete -f restore.yaml 
$ kubectl delete -f snapshot.yaml 
```

---

To understand how snapshotting works, please read the official blog
announcement with examples:
https://kubernetes.io/blog/2018/10/09/introducing-volume-snapshot-alpha-for-kubernetes/


