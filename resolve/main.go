package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	pipelineclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	prResources "github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	taskResources "github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func main() {
	ctx := context.Background()

	config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	if err != nil {
		panic(err.Error())
	}
	k8s, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("error creating kubernetes client: %v", err)
	}
	tekton, err := pipelineclient.NewForConfig(config)
	if err != nil {
		log.Fatalf("error creating tekton client: %v", err)
	}

	b, err := ioutil.ReadFile("testdata/pipelinerun.yaml")
	if err != nil {
		panic(fmt.Errorf("error reading input taskrun: %v", err))
	}
	pr := new(v1beta1.PipelineRun)
	if err := yaml.Unmarshal(b, pr); err != nil {
		panic(fmt.Errorf("error unmarshalling taskrun: %v", err))
	}

	prGet, err := prResources.GetPipelineFunc(ctx, k8s, tekton, pr)
	_, prSpec, err := prResources.GetPipelineData(ctx, pr, prGet)
	pr.Spec.PipelineRef = nil
	pr.Spec.PipelineSpec = prSpec

	for i, t := range prSpec.Tasks {
		if t.TaskSpec != nil {
			continue
		}
		get, _, err := taskResources.GetTaskFunc(ctx, k8s, tekton, t.TaskRef, pr.GetNamespace(), pr.GetServiceAccountName(t.Name))
		if err != nil {
			log.Fatal("GetTaskFunc:", err)
		}
		tr := &v1beta1.TaskRun{
			Spec: v1beta1.TaskRunSpec{
				TaskRef: t.TaskRef,
			},
		}
		meta, spec, _ := taskResources.GetTaskData(ctx, tr, get)
		t.TaskRef = nil
		t.TaskSpec = &v1beta1.EmbeddedTask{
			Metadata: v1beta1.PipelineTaskMetadata{
				Annotations: meta.Annotations,
				Labels:      meta.Labels,
			},
			TaskSpec: *spec,
		}
		prSpec.Tasks[i] = t
	}

	o, _ := yaml.Marshal(pr)
	fmt.Println(string(o))
}
