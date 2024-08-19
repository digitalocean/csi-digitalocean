module github.com/digitalocean/csi-digitalocean

go 1.23.0

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/container-storage-interface/spec v1.8.0
	github.com/digitalocean/go-metadata v0.0.0-20220602160802-6f1b22e9ba8c
	github.com/digitalocean/godo v1.120.0
	github.com/docker/docker v27.1.2+incompatible
	github.com/golang/protobuf v1.5.4
	github.com/google/go-cmp v0.6.0
	github.com/google/uuid v1.6.0
	github.com/kubernetes-csi/csi-test/v4 v4.4.0
	github.com/magiconair/properties v1.8.7
	github.com/sirupsen/logrus v1.9.3
	golang.org/x/oauth2 v0.22.0
	golang.org/x/sync v0.8.0
	golang.org/x/sys v0.24.0
	google.golang.org/grpc v1.65.0
	gotest.tools/v3 v3.5.1
	k8s.io/apimachinery v0.27.1
	k8s.io/klog/v2 v2.130.1
	k8s.io/mount-utils v0.27.1
	k8s.io/utils v0.0.0-20240711033017-18e509b52bc8
)

require (
	github.com/Microsoft/go-winio v0.4.17 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/sys/mountinfo v0.7.1 // indirect
	github.com/moby/term v0.0.0-20221205130635-1aeaba878587 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.31.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/opencontainers/runc v1.1.13 // indirect
	github.com/opencontainers/runtime-spec v1.0.3-0.20220909204839-494a5a6aca78 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.53.0 // indirect
	go.opentelemetry.io/otel v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.28.0 // indirect
	go.opentelemetry.io/otel/metric v1.28.0 // indirect
	go.opentelemetry.io/otel/sdk v1.28.0 // indirect
	go.opentelemetry.io/otel/trace v1.28.0 // indirect
	golang.org/x/net v0.26.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	google.golang.org/genproto v0.0.0-20220502173005-c8bf987b8c21 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace k8s.io/api => k8s.io/api v0.31.0

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.31.0

replace k8s.io/apimachinery => k8s.io/apimachinery v0.31.0

replace k8s.io/apiserver => k8s.io/apiserver v0.31.0

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.31.0

replace k8s.io/client-go => k8s.io/client-go v0.31.0

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.31.0

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.31.0

replace k8s.io/code-generator => k8s.io/code-generator v0.31.0

replace k8s.io/component-base => k8s.io/component-base v0.31.0

replace k8s.io/component-helpers => k8s.io/component-helpers v0.31.0

replace k8s.io/controller-manager => k8s.io/controller-manager v0.31.0

replace k8s.io/cri-api => k8s.io/cri-api v0.31.0

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.31.0

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.31.0

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.31.0

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.31.0

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.31.0

replace k8s.io/kubectl => k8s.io/kubectl v0.31.0

replace k8s.io/kubelet => k8s.io/kubelet v0.31.0

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.30.0

replace k8s.io/metrics => k8s.io/metrics v0.31.0

replace k8s.io/mount-utils => k8s.io/mount-utils v0.31.0

replace k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.31.0

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.31.0

replace k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.31.0

replace k8s.io/kms => k8s.io/kms v0.31.0

replace k8s.io/endpointslice => k8s.io/endpointslice v0.31.0

replace k8s.io/cri-client => k8s.io/cri-client v0.31.0
