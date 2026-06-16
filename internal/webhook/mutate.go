package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
)

// Config holds webhook configuration.
type Config struct {
	CAConfigMap string
	CAMountDir  string
	Excluded    map[string]bool
}

// HandleMutate is the /mutate HTTP handler.
func (c *Config) HandleMutate(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	var review admissionv1.AdmissionReview
	if err := json.Unmarshal(body, &review); err != nil {
		http.Error(w, "decode AdmissionReview", http.StatusBadRequest)
		return
	}

	review.Response = c.mutate(review.Request)
	review.Response.UID = review.Request.UID

	resp, err := json.Marshal(review)
	if err != nil {
		http.Error(w, "encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(resp) //nolint:errcheck
}

func (c *Config) mutate(req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	allow := &admissionv1.AdmissionResponse{Allowed: true}

	if c.Excluded[req.Namespace] {
		return allow
	}

	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		log.Printf("decode pod %s/%s: %v", req.Namespace, req.Name, err)
		return allow
	}

	patches := c.buildPatches(&pod)
	if len(patches) == 0 {
		return allow
	}

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		log.Printf("marshal patches: %v", err)
		return allow
	}

	pt := admissionv1.PatchTypeJSONPatch
	return &admissionv1.AdmissionResponse{
		Allowed:   true,
		PatchType: &pt,
		Patch:     patchBytes,
	}
}

type patch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func (c *Config) buildPatches(pod *corev1.Pod) []patch {
	var patches []patch

	if !hasVolume(pod, "zeus-mesh-ca") {
		optional := true
		vol := corev1.Volume{
			Name: "zeus-mesh-ca",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: c.CAConfigMap},
					Optional:             &optional,
				},
			},
		}
		if len(pod.Spec.Volumes) == 0 {
			patches = append(patches, patch{Op: "add", Path: "/spec/volumes", Value: []corev1.Volume{vol}})
		} else {
			patches = append(patches, patch{Op: "add", Path: "/spec/volumes/-", Value: vol})
		}
	}

	for i, ctr := range pod.Spec.InitContainers {
		patches = append(patches, c.containerPatches(ctr, fmt.Sprintf("/spec/initContainers/%d", i))...)
	}
	for i, ctr := range pod.Spec.Containers {
		patches = append(patches, c.containerPatches(ctr, fmt.Sprintf("/spec/containers/%d", i))...)
	}

	return patches
}

func (c *Config) containerPatches(ctr corev1.Container, base string) []patch {
	var patches []patch
	caFile := c.CAMountDir + "/ca.crt"

	// Collect env vars to inject (only those not already present)
	var newEnvs []corev1.EnvVar
	if !hasEnv(ctr, "SSL_CERT_DIR") {
		newEnvs = append(newEnvs, corev1.EnvVar{
			Name:  "SSL_CERT_DIR",
			Value: "/etc/ssl/certs:" + c.CAMountDir,
		})
	}
	if !hasEnv(ctr, "NODE_EXTRA_CA_CERTS") {
		newEnvs = append(newEnvs, corev1.EnvVar{
			Name:  "NODE_EXTRA_CA_CERTS",
			Value: caFile,
		})
	}

	// Apply env patches: initialize array if empty, else append individually
	if len(newEnvs) > 0 {
		if len(ctr.Env) == 0 {
			patches = append(patches, patch{Op: "add", Path: base + "/env", Value: newEnvs})
		} else {
			for _, e := range newEnvs {
				patches = append(patches, patch{Op: "add", Path: base + "/env/-", Value: e})
			}
		}
	}

	// Volume mount
	if !hasMount(ctr, "zeus-mesh-ca") {
		mount := corev1.VolumeMount{
			Name:      "zeus-mesh-ca",
			MountPath: c.CAMountDir,
			ReadOnly:  true,
		}
		if len(ctr.VolumeMounts) == 0 {
			patches = append(patches, patch{Op: "add", Path: base + "/volumeMounts", Value: []corev1.VolumeMount{mount}})
		} else {
			patches = append(patches, patch{Op: "add", Path: base + "/volumeMounts/-", Value: mount})
		}
	}

	return patches
}

func hasVolume(pod *corev1.Pod, name string) bool {
	for _, v := range pod.Spec.Volumes {
		if v.Name == name {
			return true
		}
	}
	return false
}

func hasMount(ctr corev1.Container, name string) bool {
	for _, m := range ctr.VolumeMounts {
		if m.Name == name {
			return true
		}
	}
	return false
}

func hasEnv(ctr corev1.Container, name string) bool {
	for _, e := range ctr.Env {
		if e.Name == name {
			return true
		}
	}
	return false
}
