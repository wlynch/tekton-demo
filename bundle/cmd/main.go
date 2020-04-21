package main

import (
	"flag"
	"os"
	"path/filepath"

	triggersclient "github.com/tektoncd/triggers/pkg/client/clientset/versioned/typed/triggers/v1alpha1"
	"github.com/wlynch/tekton-demo/bundle/pkg/bundle"
	"k8s.io/apimachinery/pkg/util/yaml"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog"
)

func main() {
	f, err := os.Open("bundle.yaml")
	if err != nil {
		klog.Exit(err)
	}
	b := new(bundle.GitHub)
	if err := yaml.NewYAMLOrJSONDecoder(f, 2048).Decode(b); err != nil {
		klog.Exit(err)
	}

	u, err := bundle.Unpack(b)
	if err != nil {
		klog.Exit(err)
	}
	klog.Infof("%+v", u)

	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}
	client, err := triggersclient.NewForConfig(config)
	if err != nil {
		klog.Exit(err)
	}
	tt, err := client.TriggerTemplates(u.TriggerTemplate.Namespace).Create(u.TriggerTemplate)
	if err != nil {
		klog.Exit(err)
	}
	klog.Infof("TriggerTemplate: %+v", tt)
	el, err := client.EventListeners(u.Trigger.Namespace).Create(u.Trigger)
	if err != nil {
		klog.Exit(err)
	}
	klog.Infof("EventListener: %+v", el)
}
