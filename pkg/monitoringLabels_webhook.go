package pkg

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:admissionReviewVersions=v1,sideEffects=None,path=/mutate,mutating=true,
// failurePolicy=fail,groups="",resources=pods,verbs=create;update,versions=v1,name=mpod.kb.io

var (
	defaultMonitoringUrl = "prometheus.io"
	defaultMonitoring    = "monitoring.bebc.com/scrape"
)

type PodLabels struct {
	Client  client.Client
	decoder *admission.Decoder
	Log     logr.Logger
}

func (p *PodLabels) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	err := p.decoder.Decode(req, pod)

	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if pod.Labels == nil {
		return admission.Allowed("")
	}

	v, ok := pod.Labels[defaultMonitoring]
	if ok && v != "true" {
		return admission.Allowed("")
	}

	if !ok {
		return admission.Allowed("")
	}

	p.Log.Info("inject annotations", "pod name", pod.Name)

	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}

	pod.Annotations[defaultMonitoringUrl+"/port"] = "8080"

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)

}

func (p *PodLabels) InjectDecoder(d *admission.Decoder) error {
	p.decoder = d
	return nil
}
