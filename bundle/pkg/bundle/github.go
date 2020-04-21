package bundle

import (
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	trigger "github.com/tektoncd/triggers/pkg/apis/triggers/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
)

// GitHub defines a bundle for GitHub resources
type GitHub struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec holds the desired state of the Pipeline from the client
	// +optional
	Spec GitHubSpec `json:"spec"`
}

// GitHubSpec github spec
type GitHubSpec struct {
	URL   string          `json:"url,omitempty"`
	Steps []pipeline.Step `json:"steps,omitempty"`
}

type Unpacked struct {
	TriggerTemplate *trigger.TriggerTemplate
	Trigger         *trigger.EventListener
}

func Unpack(g *GitHub) (*Unpacked, error) {
	if g.Namespace == "" {
		g.Namespace = "default"
	}
	labels := g.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["bundle.tekton.dev/component"] = g.Name

	pr := &pipeline.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: g.Namespace,
			Name:      g.Name,
			Labels:    labels,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: pipeline.SchemeGroupVersion.String(),
			Kind:       "PipelineRun",
		},
		Spec: pipeline.PipelineRunSpec{
			PipelineSpec: &pipeline.PipelineSpec{
				Tasks: []pipeline.PipelineTask{
					{
						Name:    "git-clone",
						TaskRef: &pipeline.TaskRef{Name: "git-clone"},
						Params: []pipeline.Param{
							{
								Name: "url",
								Value: pipeline.ArrayOrString{
									Type:      pipeline.ParamTypeString,
									StringVal: g.Spec.URL,
								},
							},
							{
								Name: "deleteExisting",
								Value: pipeline.ArrayOrString{
									Type:      pipeline.ParamTypeString,
									StringVal: "true",
								},
							},
						},
					},
					{
						Name: g.Name,
						TaskSpec: &pipeline.TaskSpec{
							Steps: g.Spec.Steps,
						},
					},
					// insert status
				},
			},
		},
	}
	klog.Infof("PipelineRun: %+v", pr.Spec.PipelineSpec.Tasks)

	tmpl := &trigger.TriggerTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: g.Namespace,
			Name:      g.Name,
			Labels:    labels,
		},
		Spec: trigger.TriggerTemplateSpec{
			ResourceTemplates: []trigger.TriggerResourceTemplate{{
				runtime.RawExtension{Object: pr},
			}},
		},
	}

	trig := &trigger.EventListener{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: g.Namespace,
			Name:      g.Name,
			Labels:    labels,
		},
		Spec: trigger.EventListenerSpec{
			Triggers: []trigger.EventListenerTrigger{{
				Name: g.Name,
				Interceptors: []*trigger.EventInterceptor{
					&trigger.EventInterceptor{
						GitHub: &trigger.GitHubInterceptor{
							EventTypes: []string{"pull_request"},
						},
					},
				},
				/*
					Bindings: []*trigger.EventListenerBinding{{
						Name: "default_github_binding",
						Kind: trigger.NamespacedTriggerBindingKind,
					}},
				*/
				Template: trigger.EventListenerTemplate{
					Name: tmpl.Name,
				},
			}},
		},
	}

	up := &Unpacked{
		TriggerTemplate: tmpl,
		Trigger:         trig,
	}
	return up, nil
}
