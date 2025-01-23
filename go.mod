module github.com/digitalocean/csi-digitalocean

go 1.23.5

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/container-storage-interface/spec v1.8.0
	github.com/digitalocean/go-metadata v0.0.0-20241007135954-c6c417d75f63
	github.com/digitalocean/godo v1.134.0
	github.com/docker/docker v26.1.5+incompatible
	github.com/golang/protobuf v1.5.4
	github.com/google/go-cmp v0.6.0
	github.com/google/uuid v1.6.0
	github.com/kubernetes-csi/csi-test/v4 v4.4.0
	github.com/magiconair/properties v1.8.7
	github.com/sirupsen/logrus v1.9.3
	golang.org/x/oauth2 v0.25.0
	golang.org/x/sync v0.10.0
	golang.org/x/sys v0.29.0
	google.golang.org/grpc v1.69.4
	gotest.tools/v3 v3.5.1
	k8s.io/apimachinery v0.32.1
	k8s.io/klog/v2 v2.130.1
	k8s.io/mount-utils v0.32.1
	k8s.io/utils v0.0.0-20241210054802-24370beab758
)

require (
	github.com/Microsoft/go-winio v0.4.14 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/moby/term v0.5.2 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.35.1 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.54.0 // indirect
	go.opentelemetry.io/otel v1.33.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.33.0 // indirect
	go.opentelemetry.io/otel/metric v1.33.0 // indirect
	go.opentelemetry.io/otel/sdk v1.33.0 // indirect
	go.opentelemetry.io/otel/trace v1.33.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/protobuf v1.36.3 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace google.golang.org/genproto => google.golang.org/genproto v0.0.0-20241209162323-e6fa225c2576

replace k8s.io/api => k8s.io/api v0.32.0

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.32.0

replace k8s.io/apimachinery => k8s.io/apimachinery v0.32.0

replace k8s.io/apiserver => k8s.io/apiserver v0.32.0

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.32.0

replace k8s.io/client-go => k8s.io/client-go v0.32.0

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.32.0

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.32.0

replace k8s.io/code-generator => k8s.io/code-generator v0.32.0

replace k8s.io/component-base => k8s.io/component-base v0.32.0

replace k8s.io/component-helpers => k8s.io/component-helpers v0.32.0

replace k8s.io/controller-manager => k8s.io/controller-manager v0.32.0

replace k8s.io/cri-api => k8s.io/cri-api v0.32.0

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.32.0

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.32.0

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.32.0

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.32.0

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.32.0

replace k8s.io/kubectl => k8s.io/kubectl v0.32.0

replace k8s.io/kubelet => k8s.io/kubelet v0.32.0

replace k8s.io/metrics => k8s.io/metrics v0.32.0

replace k8s.io/mount-utils => k8s.io/mount-utils v0.32.0

replace k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.32.0

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.32.0

replace k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.32.0

replace k8s.io/kms => k8s.io/kms v0.32.0

replace k8s.io/endpointslice => k8s.io/endpointslice v0.32.0

replace k8s.io/cri-client => k8s.io/cri-client v0.32.0

replace k8s.io/externaljwt => k8s.io/externaljwt v0.32.0
