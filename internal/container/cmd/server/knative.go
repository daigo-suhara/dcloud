package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	userServiceManagerLabel   = "dcloud-container"
	userLabelKey              = "dcloud.dev/user"
	projectLabelKey           = "dcloud.dev/project"
	serviceNameLabel          = "dcloud.dev/service-name"
)

var errKnativeServiceNotFound = errors.New("service not found")

type knativeServiceManager struct {
	namespace    string
	publicDomain string
	client       *http.Client
	baseURL      string
	token        string
}

func newKnativeServiceManager(namespace string, publicDomain string) (*knativeServiceManager, error) {
	baseURL := fmt.Sprintf("https://%s", env("KUBERNETES_SERVICE_HOST", "kubernetes.default.svc"))
	tokenPath := env("DCLD_KUBERNETES_TOKEN_FILE", "/var/run/secrets/kubernetes.io/serviceaccount/token")
	caPath := env("DCLD_KUBERNETES_CA_FILE", "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")

	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, err
	}

	rootCAs, err := x509.SystemCertPool()
	if err != nil || rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	if caBytes, readErr := os.ReadFile(caPath); readErr == nil {
		rootCAs.AppendCertsFromPEM(caBytes)
	}

	return &knativeServiceManager{
		namespace:    namespace,
		publicDomain: publicDomain,
		baseURL:      baseURL,
		token:        strings.TrimSpace(string(tokenBytes)),
		client: &http.Client{
			Timeout: 20 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{RootCAs: rootCAs},
			},
		},
	}, nil
}

func (m *knativeServiceManager) publicURL(resourceName string) string {
	return fmt.Sprintf("https://%s.%s", resourceName, m.publicDomain)
}

func (m *knativeServiceManager) customURL(domain string) string {
	return fmt.Sprintf("https://%s", domain)
}

func (m *knativeServiceManager) applyDomainMapping(ctx context.Context, domainName, resourceName string, labels map[string]string) error {
	body, err := json.Marshal(map[string]any{
		"apiVersion": "serving.knative.dev/v1beta1",
		"kind":       "DomainMapping",
		"metadata": map[string]any{
			"name":      domainName,
			"namespace": m.namespace,
			"labels":    labels,
		},
		"spec": map[string]any{
			"ref": map[string]any{
				"apiVersion": "serving.knative.dev/v1",
				"kind":       "Service",
				"name":       resourceName,
			},
		},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch,
		fmt.Sprintf("%s/apis/serving.knative.dev/v1beta1/namespaces/%s/domainmappings/%s?fieldManager=dcloud-container&force=true", m.baseURL, m.namespace, domainName),
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	m.authorize(req)
	req.Header.Set("Content-Type", "application/apply-patch+yaml")
	req.Header.Set("Accept", "application/json")
	res, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return decodeAPIError(res)
	}
	return nil
}

func (m *knativeServiceManager) deleteDomainMapping(ctx context.Context, domainName string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		fmt.Sprintf("%s/apis/serving.knative.dev/v1beta1/namespaces/%s/domainmappings/%s", m.baseURL, m.namespace, domainName),
		nil)
	if err != nil {
		return err
	}
	m.authorize(req)
	res, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 && res.StatusCode != http.StatusNotFound {
		return decodeAPIError(res)
	}
	return nil
}

// getDomainMappingStatus checks whether customDomain's DNS actually points to
// the cluster by using an external resolver (8.8.8.8), then falls back to IP
// comparison against defaultMapping (resourceName.publicDomain).
// Returns ("ready"|"pending"|"error", reason).
func (m *knativeServiceManager) getDomainMappingStatus(ctx context.Context, customDomain, defaultMapping string) (string, string) {
	dnsCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	// Check CNAME chain first (CNAME → *.publicDomain).
	if cname, err := net.DefaultResolver.LookupCNAME(dnsCtx, customDomain); err == nil {
		target := strings.TrimSuffix(cname, ".")
		if strings.HasSuffix(target, "."+m.publicDomain) || target == m.publicDomain {
			return "ready", ""
		}
	}

	// Fallback: compare A/AAAA records against the default mapping IPs.
	customIPs, err := net.DefaultResolver.LookupHost(dnsCtx, customDomain)
	if err != nil || len(customIPs) == 0 {
		return "pending", fmt.Sprintf("DNS が解決できません。CNAME を %s に設定してください", defaultMapping)
	}
	if defaultMapping != "" {
		clusterIPs, err := net.DefaultResolver.LookupHost(dnsCtx, defaultMapping)
		if err == nil && len(clusterIPs) > 0 {
			clusterSet := make(map[string]bool, len(clusterIPs))
			for _, ip := range clusterIPs {
				clusterSet[ip] = true
			}
			for _, ip := range customIPs {
				if clusterSet[ip] {
					return "ready", ""
				}
			}
			return "pending", fmt.Sprintf("DNS が別の宛先を向いています。CNAME を %s に設定してください", defaultMapping)
		}
	}
	return "ready", ""
}

func (m *knativeServiceManager) setCustomDomain(ctx context.Context, scope projectScope, name, customDomain string) error {
	resourceName := serviceResourceName(scope.ProjectID, name)
	labels := map[string]string{
		"app.kubernetes.io/instance":   "dcloud",
		"app.kubernetes.io/component":  "container",
		"app.kubernetes.io/managed-by": userServiceManagerLabel,
		userLabelKey:                   scope.UserID,
		projectLabelKey:                scope.ProjectID,
		serviceNameLabel:               name,
	}
	return m.applyDomainMapping(ctx, customDomain, resourceName, labels)
}

func (m *knativeServiceManager) list(ctx context.Context, scope projectScope) ([]deployedService, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/apis/serving.knative.dev/v1/namespaces/%s/services?labelSelector=%s", m.baseURL, m.namespace, strings.Join([]string{
		projectLabelKey + "=" + strings.TrimSpace(scope.ProjectID),
	}, ",")), nil)
	if err != nil {
		return nil, err
	}
	m.authorize(req)

	res, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return nil, decodeAPIError(res)
	}

	var payload struct {
		Items []struct {
			Metadata struct {
				Name              string            `json:"name"`
				CreationTimestamp time.Time         `json:"creationTimestamp"`
				Generation        int64             `json:"generation"`
				Namespace         string            `json:"namespace"`
				Labels            map[string]string `json:"labels"`
			} `json:"metadata"`
			Spec struct {
				Template struct {
					Spec struct {
						Containers []struct {
							Image string `json:"image"`
						} `json:"containers"`
					} `json:"spec"`
				} `json:"template"`
			} `json:"spec"`
			Status struct {
				Conditions []struct {
					Type               string    `json:"type"`
					Status             string    `json:"status"`
					Reason             string    `json:"reason"`
					LastTransitionTime time.Time `json:"lastTransitionTime"`
				} `json:"conditions"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}

	services := make([]deployedService, 0, len(payload.Items))
	for _, item := range payload.Items {
		displayName := strings.TrimSpace(item.Metadata.Labels[serviceNameLabel])
		if displayName == "" {
			displayName = item.Metadata.Name
		}
		svc := deployedService{
			Name:         displayName,
			Image:        "",
			URL:          m.publicURL(item.Metadata.Name),
			ResourceName: item.Metadata.Name,
			CreatedAt:    item.Metadata.CreationTimestamp.UTC().Format(time.RFC3339),
			UpdatedAt:    item.Metadata.CreationTimestamp.UTC().Format(time.RFC3339),
			Namespace:    item.Metadata.Namespace,
			ProjectID:    item.Metadata.Labels[projectLabelKey],
			Generation:   item.Metadata.Generation,
		}
		if len(item.Spec.Template.Spec.Containers) > 0 {
			svc.Image = item.Spec.Template.Spec.Containers[0].Image
		}
		for _, cond := range item.Status.Conditions {
			if cond.Type == "Ready" {
				svc.Ready = cond.Status == "True"
				svc.Reason = cond.Reason
				if !cond.LastTransitionTime.IsZero() {
					svc.UpdatedAt = cond.LastTransitionTime.UTC().Format(time.RFC3339)
				}
				break
			}
		}
		services = append(services, svc)
	}
	return services, nil
}

func (m *knativeServiceManager) deploy(ctx context.Context, scope projectScope, req deployRequest) (deployedService, error) {
	resourceName := serviceResourceName(scope.ProjectID, req.Name)
	manifest := knativeServiceManifest{
		APIVersion: "serving.knative.dev/v1",
		Kind:       "Service",
	}
	manifest.Metadata.Name = resourceName
	manifest.Metadata.Namespace = m.namespace
	manifest.Metadata.Labels = map[string]string{
		"app.kubernetes.io/instance":   "dcloud",
		"app.kubernetes.io/component":  "container",
		"app.kubernetes.io/managed-by": userServiceManagerLabel,
		userLabelKey:                   scope.UserID,
		projectLabelKey:                scope.ProjectID,
		serviceNameLabel:               req.Name,
	}
	manifest.Spec.Template.Metadata.Labels = map[string]string{
		"app.kubernetes.io/instance":  "dcloud",
		"app.kubernetes.io/component": "container",
		userLabelKey:                  scope.UserID,
		projectLabelKey:               scope.ProjectID,
		serviceNameLabel:              req.Name,
	}
	if req.MinScale > 0 || req.MaxScale > 0 {
		manifest.Spec.Template.Metadata.Annotations = map[string]string{}
		if req.MinScale > 0 {
			manifest.Spec.Template.Metadata.Annotations["autoscaling.knative.dev/minScale"] = fmt.Sprintf("%d", req.MinScale)
		}
		if req.MaxScale > 0 {
			manifest.Spec.Template.Metadata.Annotations["autoscaling.knative.dev/maxScale"] = fmt.Sprintf("%d", req.MaxScale)
		}
	}
	manifest.Spec.Template.Spec.Containers = []knativeContainer{{
		Name:  req.Name,
		Image: req.Image,
		Ports: []knativeContainerPort{{ContainerPort: req.Port}},
	}}

	body, err := json.Marshal(manifest)
	if err != nil {
		return deployedService{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, fmt.Sprintf("%s/apis/serving.knative.dev/v1/namespaces/%s/services/%s?fieldManager=dcloud-container&force=true", m.baseURL, m.namespace, resourceName), bytes.NewReader(body))
	if err != nil {
		return deployedService{}, err
	}
	m.authorize(httpReq)
	httpReq.Header.Set("Content-Type", "application/apply-patch+yaml")
	httpReq.Header.Set("Accept", "application/json")

	res, err := m.client.Do(httpReq)
	if err != nil {
		return deployedService{}, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return deployedService{}, decodeAPIError(res)
	}

	var payload struct {
		Metadata struct {
			Name              string    `json:"name"`
			CreationTimestamp time.Time `json:"creationTimestamp"`
			Generation        int64     `json:"generation"`
			Namespace         string    `json:"namespace"`
		} `json:"metadata"`
		Spec struct {
			Template struct {
				Spec struct {
					Containers []struct {
						Image string `json:"image"`
					} `json:"containers"`
				} `json:"spec"`
			} `json:"template"`
		} `json:"spec"`
		Status struct {
			Conditions []struct {
				Type               string    `json:"type"`
				Status             string    `json:"status"`
				Reason             string    `json:"reason"`
				LastTransitionTime time.Time `json:"lastTransitionTime"`
			} `json:"conditions"`
		} `json:"status"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return deployedService{}, err
	}

	defaultDomainLabels := map[string]string{
		"app.kubernetes.io/instance":   "dcloud",
		"app.kubernetes.io/component":  "container",
		"app.kubernetes.io/managed-by": userServiceManagerLabel,
		userLabelKey:                   scope.UserID,
		projectLabelKey:                scope.ProjectID,
		serviceNameLabel:               req.Name,
	}
	if err := m.applyDomainMapping(ctx, fmt.Sprintf("%s.%s", resourceName, m.publicDomain), resourceName, defaultDomainLabels); err != nil {
		return deployedService{}, err
	}

	service := deployedService{
		Name:         req.Name,
		Image:        req.Image,
		URL:          m.publicURL(resourceName),
		ResourceName: resourceName,
		Namespace:    payload.Metadata.Namespace,
		ProjectID:    scope.ProjectID,
		Generation:   payload.Metadata.Generation,
		CreatedAt:    payload.Metadata.CreationTimestamp.UTC().Format(time.RFC3339),
		UpdatedAt:    payload.Metadata.CreationTimestamp.UTC().Format(time.RFC3339),
	}
	if len(payload.Spec.Template.Spec.Containers) > 0 {
		service.Image = payload.Spec.Template.Spec.Containers[0].Image
	}
	for _, cond := range payload.Status.Conditions {
		if cond.Type == "Ready" {
			service.Ready = cond.Status == "True"
			service.Reason = cond.Reason
			if !cond.LastTransitionTime.IsZero() {
				service.UpdatedAt = cond.LastTransitionTime.UTC().Format(time.RFC3339)
			}
			break
		}
	}
	return service, nil
}

func (m *knativeServiceManager) delete(ctx context.Context, scope projectScope, name, customDomain string) error {
	resourceName := serviceResourceName(scope.ProjectID, name)
	if customDomain != "" {
		if err := m.deleteDomainMapping(ctx, customDomain); err != nil {
			return err
		}
	}
	if err := m.deleteDomainMapping(ctx, fmt.Sprintf("%s.%s", resourceName, m.publicDomain)); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/apis/serving.knative.dev/v1/namespaces/%s/services/%s", m.baseURL, m.namespace, resourceName), nil)
	if err != nil {
		return err
	}
	m.authorize(req)
	res, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 && res.StatusCode != http.StatusNotFound {
		return decodeAPIError(res)
	}
	return nil
}

func (m *knativeServiceManager) authorize(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+m.token)
}

func serviceResourceName(projectID, name string) string {
	seed := strings.TrimSpace(projectID) + ":" + strings.TrimSpace(name)
	sum := sha256.Sum256([]byte(seed))
	suffix := hex.EncodeToString(sum[:4])
	prefix := sanitizeDNSLabel(name)
	if prefix == "" {
		prefix = "service"
	}
	maxPrefixLen := 63 - 1 - len(suffix)
	if len(prefix) > maxPrefixLen {
		prefix = strings.TrimRight(prefix[:maxPrefixLen], "-")
	}
	if prefix == "" {
		prefix = "service"
	}
	return prefix + "-" + suffix
}

func sanitizeDNSLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastHyphen := false
	for _, ch := range value {
		switch {
		case ch >= 'a' && ch <= 'z':
			builder.WriteRune(ch)
			lastHyphen = false
		case ch >= '0' && ch <= '9':
			builder.WriteRune(ch)
			lastHyphen = false
		case builder.Len() > 0 && !lastHyphen:
			builder.WriteRune('-')
			lastHyphen = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

type knativeServiceManifest struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string            `json:"name"`
		Namespace string            `json:"namespace"`
		Labels    map[string]string `json:"labels"`
	} `json:"metadata"`
	Spec struct {
		Template struct {
			Metadata struct {
				Labels      map[string]string `json:"labels"`
				Annotations map[string]string `json:"annotations,omitempty"`
			} `json:"metadata"`
			Spec struct {
				Containers []knativeContainer `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
}

type knativeContainer struct {
	Name  string               `json:"name"`
	Image string               `json:"image"`
	Ports []knativeContainerPort `json:"ports"`
}

type knativeContainerPort struct {
	ContainerPort int32 `json:"containerPort"`
}

func decodeAPIError(res *http.Response) error {
	var payload struct {
		Message string `json:"message"`
		Reason  string `json:"reason"`
		Code    int    `json:"code"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err == nil {
		if payload.Message != "" {
			return fmt.Errorf("%s", payload.Message)
		}
		if payload.Reason != "" {
			return fmt.Errorf("%s", payload.Reason)
		}
	}
	return fmt.Errorf("kubernetes api returned %s", res.Status)
}
