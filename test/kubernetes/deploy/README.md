# Deploying and Testing a Development Version

The most convenient way to test changes to csi-digitalocean is on a
[DigitalOcean Kubernetes (DOKS)](https://www.digitalocean.com/products/kubernetes/)
cluster. However, since DOKS clusters run the DigitalOcean CSI controller as a
managed component that cannot be modified by the user, deploying with the
example manifests will not work.

To test a development version on a DOKS cluster, do the following:

1. Create a DOKS cluster on the latest version:
   ```console
   $ doctl k8s cluster create csi-integration-test
   Notice: cluster is provisioning, waiting for cluster to be running
   ..
   ```
   Wait for it to finish creating; your kubeconfig will automatically be updated.

2. Build and push a dev version of csi-digitalocean by running `VERSION=dev make publish`
   from the root of the repository.

3. Run `deploy.sh` from this directory, providing a DO API access token for your
   account:
   ```console
   $ DIGITALOCEAN_ACCESS_TOKEN=<token> ./deploy.sh
   Deploying a dev version of the CSI driver to context do-nyc1-csi-integration-test.
   Continue? (yes/no)
   yes
   ```
   This requires [`kustomize`](https://github.com/kubernetes-sigs/kustomize) and `kubectl`.

4. Run the integration tests from the repository root specifying a DigitalOcean API token
   against the dev storage class:
   ```console
   $ CSI_DIGITALOCEAN_ACCESS_TOKEN=aa823a5a07d5aa7c TEST_STORAGE_CLASS=do-block-storage-dev make test-integration
   ```

   **Note:** If the `CSI_DIGITALOCEAN_ACCESS_TOKEN` environment variable does not specify
   a token, the standard `DIGITALOCEAN_ACCESS_TOKEN` environment variable will be tried before
   returning an error.

## Alternative Image Locations

The instructions above assume you have push access to the
`digitalocean/do-csi-plugin` repository on Docker Hub. However, this is not
necessary to build and test a dev version of the CSI driver.

To build and publish a test version in your own image repository, do the
following from the root of the repository:

```console
$ DOCKER_REPO=<my-image-repository> VERSION=dev make publish
```

You can then follow the instructions above, setting the `DEV_IMAGE` environment
variable to your own image location when invoking `deploy.sh`.

Note that testing an image from another location will cause `kustomization.yaml`
to be updated. Please do not commit these changes.
