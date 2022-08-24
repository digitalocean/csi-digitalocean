module github.com/digitalocean/csi-digitalocean

go 1.17

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/container-storage-interface/spec v1.5.0
	github.com/digitalocean/go-metadata v0.0.0-20180111002115-15bd36e5f6f7
	github.com/digitalocean/godo v1.29.0
	github.com/docker/docker v20.10.2+incompatible
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.5
	github.com/google/uuid v1.2.0
	github.com/kubernetes-csi/csi-test/v4 v4.3.0
	github.com/magiconair/properties v1.8.5
	github.com/sirupsen/logrus v1.8.1
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20220209214540-3681064d5158
	google.golang.org/grpc v1.40.0
	k8s.io/apimachinery v0.24.3
	k8s.io/mount-utils v0.23.7
	k8s.io/utils v0.0.0-20220210201930-3a6ce19ff2f9
)

require (
	github.com/Microsoft/go-winio v0.4.17 // indirect
	github.com/containerd/containerd v1.5.8 // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/go-logr/logr v1.2.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/nxadm/tail v1.4.5 // indirect
	github.com/onsi/ginkgo v1.14.2 // indirect
	github.com/onsi/gomega v1.10.4 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20220107163113-42d7afdf6368 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/klog/v2 v2.60.1 // indirect
)

replace k8s.io/api => k8s.io/api v0.24.3

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.24.3

replace k8s.io/apimachinery => k8s.io/apimachinery v0.24.4-rc.0

replace k8s.io/apiserver => k8s.io/apiserver v0.24.3

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.24.3

replace k8s.io/client-go => k8s.io/client-go v0.24.3

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.24.3

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.24.3

replace k8s.io/code-generator => k8s.io/code-generator v0.24.4-rc.0

replace k8s.io/component-base => k8s.io/component-base v0.24.3

replace k8s.io/component-helpers => k8s.io/component-helpers v0.24.3

replace k8s.io/controller-manager => k8s.io/controller-manager v0.24.3

replace k8s.io/cri-api => k8s.io/cri-api v0.25.0-alpha.0

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.24.3

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.24.3

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.24.3

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.24.3

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.24.3

replace k8s.io/kubectl => k8s.io/kubectl v0.24.3

replace k8s.io/kubelet => k8s.io/kubelet v0.24.3

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.24.3

replace k8s.io/metrics => k8s.io/metrics v0.24.3

replace k8s.io/mount-utils => k8s.io/mount-utils v0.24.4-rc.0

replace k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.24.3

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.24.3
