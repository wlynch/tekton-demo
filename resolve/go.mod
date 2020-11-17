module github.com/wlynch/tekton-demo/resolve

go 1.14

require (
	github.com/tektoncd/pipeline v0.18.1
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	sigs.k8s.io/yaml v1.2.0
)

// Pin Tekton Pipelines deps (v0.17.2)
replace (
	k8s.io/api => k8s.io/api v0.18.8
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.8
	k8s.io/client-go => k8s.io/client-go v0.18.8
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20200410145947-bcb3869e6f29
	knative.dev/pkg => knative.dev/pkg v0.0.0-20200922164940-4bf40ad82aab
)
