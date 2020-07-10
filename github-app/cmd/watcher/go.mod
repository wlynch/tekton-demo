module github.com/wlynch/tekton-demo/github-app/cmd/watcher

go 1.14

require (
	contrib.go.opencensus.io/exporter/ocagent v0.7.0 // indirect
	contrib.go.opencensus.io/exporter/prometheus v0.2.0 // indirect
	github.com/bradleyfalzon/ghinstallation v1.1.1
	github.com/google/go-github v17.0.0+incompatible
	github.com/tektoncd/pipeline v0.12.0
	go.uber.org/zap v1.15.0
	k8s.io/api v0.18.5
	k8s.io/apimachinery v0.18.5
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	knative.dev/pkg v0.0.0-20200710003319-43f4f824e3a3
	sigs.k8s.io/yaml v1.2.0
)

// Knative deps (release-0.13)
replace (
	contrib.go.opencensus.io/exporter/stackdriver => contrib.go.opencensus.io/exporter/stackdriver v0.12.9-0.20191108183826-59d068f8d8ff
	knative.dev/caching => knative.dev/caching v0.0.0-20200116200605-67bca2c83dfa
	knative.dev/pkg => knative.dev/pkg v0.0.0-20200306230727-a56a6ea3fa56
	knative.dev/pkg/vendor/github.com/spf13/pflag => github.com/spf13/pflag v1.0.5
)

// Pin k8s deps to 1.16.5
replace (
	k8s.io/api => k8s.io/api v0.16.5
	k8s.io/apimachinery => k8s.io/apimachinery v0.16.5
	k8s.io/client-go => k8s.io/client-go v0.16.5
	k8s.io/code-generator => k8s.io/code-generator v0.16.5
	k8s.io/gengo => k8s.io/gengo v0.0.0-20190327210449-e17681d19d3a
)
