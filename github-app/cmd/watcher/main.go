/*
Copyright 2020 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/github"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	taskruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/taskrun"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	"sigs.k8s.io/yaml"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
	_ "knative.dev/pkg/system/testing"
)

var (
	apiAddr   = flag.String("api_addr", "localhost:50051", "Address of API server to report to")
	namespace = flag.String("namespace", corev1.NamespaceAll, "Namespace to restrict informer to. Optional, defaults to all namespaces.")
)

var (
	/*
	   	summaryTmpl = template.Must(template.New("tmpl").Parse(`
	   | Build Information |   |
	   | ----------------- | - |
	   | Name | {{.Name}}
	   | Status   | {{ range .Status.Conditions }}{{.reason}}{{end}} |
	   | Details   | {{ range .Status.Conditions }}{{.message}}{{end}} |
	   | Start   | {{ .status.startTime }} |
	   | End | {{ .status.finishTime }} |
	   `))
	*/
	summaryTmpl = template.Must(template.New("tmpl").Parse(`
| Task Summary |   |
| ----------------- | - |
| API Version | {{.APIVersion}}
| Kind    | {{.Kind}}
| Namespace   | {{.Namespace}}
| Name    | {{.Name}}
| Status  | {{ range .Status.Conditions }}{{.Reason}}{{end}} |
| Details | {{ range .Status.Conditions }}{{.Message}}{{end}} |
| Start   | {{ .Status.StartTime }} |
| End     | {{ .Status.CompletionTime }} |

## Steps

| Name | Status | Start | End
| ---- | ------ | ----- | ---
{{ range .Status.Steps }}{{.Name}} |  {{.ContainerState.Terminated.Reason}} | {{.ContainerState.Terminated.StartedAt}} | {{.ContainerState.Terminated.FinishedAt}}
{{end}}

## Spec
`))
)

func main() {
	flag.Parse()

	// Wrap the shared transport for use with the app ID 1 authenticating with installation ID 99.
	keypath := filepath.Join(os.Getenv("HOME"), "Downloads", "wlynch-test.2020-07-09.private-key.pem")
	at, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, 9994, keypath)
	if err != nil {
		log.Fatal(err)
	}

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	sharedmain.MainWithContext(injection.WithNamespaceScope(signals.NewContext(), *namespace), "watcher", func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		logger := logging.FromContext(ctx)
		taskRunInformer := taskruninformer.Get(ctx)

		c := &reconciler{
			logger:        logger,
			taskRunLister: taskRunInformer.Lister(),
			at:            at,
			k8s:           clientset,
		}
		impl := controller.NewImpl(c, c.logger, pipeline.TaskRunControllerName)

		taskRunInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    impl.Enqueue,
			UpdateFunc: controller.PassNew(impl.Enqueue),
		})

		return impl
	})
}

type reconciler struct {
	logger        *zap.SugaredLogger
	taskRunLister listers.TaskRunLister
	at            *ghinstallation.AppsTransport
	k8s           kubernetes.Interface
}

func (r *reconciler) Reconcile(ctx context.Context, key string) error {
	fmt.Println("RECONCILE")
	r.logger.Infof("reconciling resource key: %s", key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		r.logger.Errorf("invalid resource key: %s", key)
		return nil
	}

	// Get the Task Run resource with this namespace/name
	tr, err := r.taskRunLister.TaskRuns(namespace).Get(name)
	if errors.IsNotFound(err) {
		// The resource no longer exists, in which case we stop processing.
		r.logger.Infof("task run %q in work queue no longer exists", key)
		return nil
	} else if err != nil {
		r.logger.Errorf("Error retrieving TaskRun %q: %s", name, err)
		return err
	}

	// Only respond to final state for now.
	if len(tr.Status.Conditions) < 1 || tr.Status.Conditions[0].Type != "Succeeded" || tr.Status.Conditions[0].IsFalse() {
		return nil
	}

	r.logger.Infof("Sending update for %s/%s (uid %s)", namespace, name, tr.UID)

	ia := newIntegrationAnnotations(tr.ObjectMeta)
	id := ia.key("app-installation")
	if id == "" {
		r.logger.Infof("%s/%s (uid %s) not a GitHub App task, skipping", namespace, name, tr.UID)
		return nil
	}
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return err
	}
	gh := github.NewClient(&http.Client{Transport: ghinstallation.NewFromAppsTransport(r.at, n)})

	b := new(bytes.Buffer)
	if err := summaryTmpl.Execute(b, tr); err != nil {
		r.logger.Errorf("%s/%s (uid %s) template.Execute: %v", namespace, name, tr.UID, err)
		return err
	}
	spec, err := yaml.Marshal(tr.Spec)
	if err != nil {
		r.logger.Errorf("%s/%s (uid %s) spec marshal: %v", namespace, name, tr.UID, err)
		return err
	}
	b.WriteString(fmt.Sprintf("```\n%s\n```", string(spec)))

	logs, err := getLogs(ctx, r.k8s, tr)
	if err != nil {
		r.logger.Errorf("%s/%s (uid %s) get logs: %v", namespace, name, tr.UID, err)
		return err
	}

	if _, _, err := gh.Checks.CreateCheckRun(ctx, ia.key("owner"), ia.key("repo"), github.CreateCheckRunOptions{
		ExternalID: github.String(string(tr.UID)),
		Name:       fmt.Sprintf("%s", tr.Name),
		Conclusion: github.String("success"),
		HeadSHA:    ia.key("commit"),
		Output: &github.CheckRunOutput{
			Title:   github.String(tr.Name),
			Summary: github.String(b.String()),
			Text:    github.String(logs),
		},
		StartedAt:   &github.Timestamp{Time: tr.Status.StartTime.Time},
		CompletedAt: &github.Timestamp{Time: tr.Status.CompletionTime.Time},
		DetailsURL:  github.String(fmt.Sprintf("https://console.cloud.google.com/kubernetes/pod/us-east1/cb4a1/default/%s/details?project=wlynch-test", tr.Status.PodName)),
	}); err != nil {
		r.logger.Errorf("%s/%s (uid %s): CreateCheck: %v", namespace, name, tr.UID, err)
	}
	return nil
}

func getLogs(ctx context.Context, client kubernetes.Interface, tr *v1alpha1.TaskRun) (string, error) {
	b := new(bytes.Buffer)

	pod, err := client.CoreV1().Pods(tr.Namespace).Get(tr.Status.PodName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	for _, c := range pod.Spec.Containers {
		b.WriteString(fmt.Sprintf("# %s\n```\n", c.Name))
		rc, err := client.CoreV1().Pods(tr.Namespace).GetLogs(tr.Status.PodName, &corev1.PodLogOptions{Container: c.Name}).Stream()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		if _, err := io.Copy(b, rc); err != nil {
			return "", err
		}
		b.WriteString("\n```\n")

	}
	return b.String(), err
}

type integrationAnnotations map[string]string

func newIntegrationAnnotations(o metav1.ObjectMeta) integrationAnnotations {
	return integrationAnnotations(o.Annotations)
}

func (a integrationAnnotations) key(key string) string {
	return a["github.integrations.tekton.dev/"+key]
}
