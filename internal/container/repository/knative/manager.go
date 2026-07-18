package knative

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

	"github.com/daigo-suhara/dcloud/internal/container/domain"
)

// env reads an environment variable with a fallback default. Duplicated
// here (rather than importing a shared helper) to keep this package
// self-contained; the constants baked in below are the runtime knobs
// exposed by the container service deployment.
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

const (
	userServiceManagerLabel   = "dcloud-container"
	userLabelKey              = "dcloud.dev/user"
	projectLabelKey           = "dcloud.dev/project"
	serviceNameLabel          = "dcloud.dev/service-name"
)

var ErrServiceNotFound = errors.New("service not found")

type Manager struct {
	namespace    string
	PublicDomain string
	client       *http.Client
	baseURL      string
	token        string
}

func NewManager(namespace, publicDomain string) (*Manager, error) {
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

	return &Manager{
		namespace:    namespace,
		PublicDomain: publicDomain,
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

func (m *Manager) nsFor(projectID string) string {
	if projectID == "" {
		return m.namespace
	}
	return projectID
}

func (m *Manager) PublicURL(resourceName string) string {
	return fmt.Sprintf("https://%s.%s", resourceName, m.PublicDomain)
}

func (m *Manager) CustomURL(domain string) string {
	return fmt.Sprintf("https://%s", domain)
}

func (m *Manager) applyDomainMapping(ctx context.Context, ns, domainName, resourceName string, labels map[string]string) error {
	for _, path := range []string{
		fmt.Sprintf("/apis/networking.internal.knative.dev/v1alpha1/namespaces/%s/ingresses/%s", ns, domainName),
		fmt.Sprintf("/api/v1/namespaces/%s/services/%s", ns, resourceName+"-h1gw"),
	} {
		delReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, m.baseURL+path, nil)
		if delReq != nil {
			m.authorize(delReq)
			if delRes, err := m.client.Do(delReq); err == nil {
				delRes.Body.Close()
			}
		}
	}

	body, err := json.Marshal(map[string]any{
		"apiVersion": "serving.knative.dev/v1beta1",
		"kind":       "DomainMapping",
		"metadata": map[string]any{
			"name":      domainName,
			"namespace": ns,
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
		fmt.Sprintf("%s/apis/serving.knative.dev/v1beta1/namespaces/%s/domainmappings/%s?fieldManager=dcloud-container&force=true", m.baseURL, ns, domainName),
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

func (m *Manager) DeleteDomainMapping(ctx context.Context, projectID, domainName string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		fmt.Sprintf("%s/apis/serving.knative.dev/v1beta1/namespaces/%s/domainmappings/%s", m.baseURL, m.nsFor(projectID), domainName),
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

func (m *Manager) fetchDomainMappingReady(ctx context.Context, projectID, domainName string) (status, reason string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/apis/serving.knative.dev/v1beta1/namespaces/%s/domainmappings/%s",
			m.baseURL, m.nsFor(projectID), domainName),
		nil)
	if err != nil {
		return "", "", err
	}
	m.authorize(req)
	res, err := m.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return "", "", ErrServiceNotFound
	}
	if res.StatusCode >= 300 {
		return "", "", decodeAPIError(res)
	}
	var payload struct {
		Status struct {
			Conditions []struct {
				Type    string `json:"type"`
				Status  string `json:"status"`
				Reason  string `json:"reason"`
				Message string `json:"message"`
			} `json:"conditions"`
		} `json:"status"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return "", "", err
	}
	for _, cond := range payload.Status.Conditions {
		if cond.Type == "Ready" {
			msg := cond.Reason
			if cond.Message != "" {
				msg = cond.Message
			}
			return cond.Status, msg, nil
		}
	}
	return "Unknown", "", nil
}

// getDomainMappingStatus checks the Knative DomainMapping's Ready condition for
// a custom domain and, when not yet conclusive, falls back to a DNS CNAME lookup.
// Returns ("ready"|"pending"|"error", reason).
func (m *Manager) GetDomainMappingStatus(ctx context.Context, projectID, customDomain, defaultMapping string) (string, string) {
	dmCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	readyStatus, reason, err := m.fetchDomainMappingReady(dmCtx, projectID, customDomain)
	if err == nil {
		switch readyStatus {
		case "True":
			// Routing is active. Verify DNS has propagated via CNAME check.
			dnsCtx, dnsCancel := context.WithTimeout(ctx, 5*time.Second)
			defer dnsCancel()
			if cname, lookupErr := net.DefaultResolver.LookupCNAME(dnsCtx, customDomain); lookupErr == nil {
				target := strings.TrimSuffix(cname, ".")
				if strings.HasSuffix(target, "."+m.PublicDomain) || target == m.PublicDomain {
					return "ready", ""
				}
				// Apex domains use Cloudflare CNAME flattening: the CNAME is
				// transparently resolved to A records, so LookupCNAME returns
				// the domain itself. Compare A records with the default mapping.
				if target == strings.TrimSuffix(customDomain, ".") {
					if domainAddrs, e1 := net.DefaultResolver.LookupHost(dnsCtx, customDomain); e1 == nil {
						if targetAddrs, e2 := net.DefaultResolver.LookupHost(dnsCtx, defaultMapping); e2 == nil {
							for _, a := range domainAddrs {
								for _, b := range targetAddrs {
									if a == b {
										return "ready", ""
									}
								}
							}
						}
					}
				}
			}
			// Routing set up but DNS not yet visible from cluster — treat as pending.
			return "pending", fmt.Sprintf("CNAME を %s に設定してください", defaultMapping)
		case "False":
			if reason != "" {
				return "error", reason
			}
			return "error", "ドメインマッピングに問題があります"
		}
	}

	// DomainMapping not found or still reconciling — fall back to DNS check.
	dnsCtx, dnsCancel := context.WithTimeout(ctx, 8*time.Second)
	defer dnsCancel()
	if cname, lookupErr := net.DefaultResolver.LookupCNAME(dnsCtx, customDomain); lookupErr == nil {
		target := strings.TrimSuffix(cname, ".")
		if strings.HasSuffix(target, "."+m.PublicDomain) || target == m.PublicDomain {
			return "ready", ""
		}
	}
	return "pending", fmt.Sprintf("CNAME を %s に設定してください", defaultMapping)
}

func (m *Manager) SetCustomDomain(ctx context.Context, scope domain.ProjectScope, name, customDomain string) error {
	resourceName := ServiceResourceName(scope.ProjectID, name)
	labels := map[string]string{
		"app.kubernetes.io/instance":   "dcloud",
		"app.kubernetes.io/component":  "container",
		"app.kubernetes.io/managed-by": userServiceManagerLabel,
		userLabelKey:                   scope.UserID,
		projectLabelKey:                scope.ProjectID,
		serviceNameLabel:               name,
	}
	return m.applyDomainMapping(ctx, m.nsFor(scope.ProjectID), customDomain, resourceName, labels)
}

func (m *Manager) List(ctx context.Context, scope domain.ProjectScope) ([]domain.DeployedService, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/apis/serving.knative.dev/v1/namespaces/%s/services?labelSelector=%s", m.baseURL, m.nsFor(scope.ProjectID), strings.Join([]string{
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
					Metadata struct {
						Annotations map[string]string `json:"annotations"`
					} `json:"metadata"`
					Spec struct {
						Containers []struct {
							Image string `json:"image"`
							Ports []struct {
								ContainerPort int32 `json:"containerPort"`
							} `json:"ports"`
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

	services := make([]domain.DeployedService, 0, len(payload.Items))
	for _, item := range payload.Items {
		displayName := strings.TrimSpace(item.Metadata.Labels[serviceNameLabel])
		if displayName == "" {
			displayName = item.Metadata.Name
		}
		svc := domain.DeployedService{
			Name:         displayName,
			Image:        "",
			URL:          m.PublicURL(item.Metadata.Name),
			ResourceName: item.Metadata.Name,
			CreatedAt:    item.Metadata.CreationTimestamp.UTC().Format(time.RFC3339),
			UpdatedAt:    item.Metadata.CreationTimestamp.UTC().Format(time.RFC3339),
			Namespace:    item.Metadata.Namespace,
			ProjectID:    item.Metadata.Labels[projectLabelKey],
			Generation:   item.Metadata.Generation,
		}
		if len(item.Spec.Template.Spec.Containers) > 0 {
			c := item.Spec.Template.Spec.Containers[0]
			svc.Image = c.Image
			if len(c.Ports) > 0 {
				svc.Port = c.Ports[0].ContainerPort
			}
		}
		if ann := item.Spec.Template.Metadata.Annotations; ann != nil {
			svc.MinScale = parseScaleAnnotation(ann["autoscaling.knative.dev/minScale"])
			svc.MaxScale = parseScaleAnnotation(ann["autoscaling.knative.dev/maxScale"])
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

func (m *Manager) Deploy(ctx context.Context, scope domain.ProjectScope, req domain.DeployRequest) (domain.DeployedService, error) {
	resourceName := ServiceResourceName(scope.ProjectID, req.Name)
	manifest := knativeServiceManifest{
		APIVersion: "serving.knative.dev/v1",
		Kind:       "Service",
	}
	manifest.Metadata.Name = resourceName
	manifest.Metadata.Namespace = m.nsFor(scope.ProjectID)
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
	}
	if req.MinScale > 0 {
		manifest.Spec.Template.Metadata.Annotations["autoscaling.knative.dev/minScale"] = fmt.Sprintf("%d", req.MinScale)
	}
	if req.MaxScale > 0 {
		manifest.Spec.Template.Metadata.Annotations["autoscaling.knative.dev/maxScale"] = fmt.Sprintf("%d", req.MaxScale)
	}
	container := knativeContainer{
		Name:  req.Name,
		Image: req.Image,
		Ports: []knativeContainerPort{{ContainerPort: req.Port}},
	}
	if req.StartupScript != "" {
		container.Command = []string{"/bin/sh", "-c"}
		container.Args = []string{req.StartupScript}
	}
	if len(req.Env) > 0 {
		container.Env = make([]knativeEnvVar, len(req.Env))
		for i, e := range req.Env {
			container.Env[i] = knativeEnvVar{Name: e.Name, Value: e.Value}
		}
	}
	noTimeout := int64(0)
	manifest.Spec.Template.Spec.TimeoutSeconds = &noTimeout
	manifest.Spec.Template.Spec.RuntimeClassName = env("DCLD_CONTAINER_RUNTIME_CLASS", "")

	// Bucket volumes: inject an s3fs sidecar per mount and share an emptyDir
	// between it and the user container.
	sidecars := []knativeContainer{}
	for i, v := range req.BucketVolumes {
		volName := fmt.Sprintf("bucket-%d", i)
		credsName := fmt.Sprintf("bucket-creds-%d", i)
		container.VolumeMounts = append(container.VolumeMounts, knativeVolumeMount{
			Name: volName, MountPath: v.MountPath, MountPropagation: "HostToContainer",
		})
		manifest.Spec.Template.Spec.Volumes = append(manifest.Spec.Template.Spec.Volumes,
			knativeVolume{Name: volName, EmptyDir: &struct{}{}},
			knativeVolume{Name: credsName, Secret: &knativeVolumeSecretSource{SecretName: v.BucketName}},
		)
		privileged := false
		sidecars = append(sidecars, knativeContainer{
			Name:  fmt.Sprintf("s3fs-%d", i),
			Image: env("DCLD_S3FS_IMAGE", "ghcr.io/efrecon/s3fs:1.94"),
			Env: []knativeEnvVar{
				{Name: "AWS_S3_BUCKET", Value: v.BucketName},
				{Name: "AWS_S3_URL", Value: env("DCLD_S3_ENDPOINT", "http://rook-ceph-rgw-default.rook-ceph.svc.cluster.local:80")},
				{Name: "S3FS_ARGS", Value: "use_path_request_style,allow_other"},
			},
			VolumeMounts: []knativeVolumeMount{
				{Name: volName, MountPath: "/opt/s3fs/bucket", MountPropagation: "Bidirectional"},
				{Name: credsName, MountPath: "/opt/s3fs/creds", ReadOnly: true},
			},
			SecurityContext: &knativeSecurityContext{
				Capabilities: &knativeCapabilities{Add: []string{"SYS_ADMIN"}},
				Privileged:   &privileged,
			},
		})
	}
	manifest.Spec.Template.Spec.Containers = append([]knativeContainer{container}, sidecars...)

	body, err := json.Marshal(manifest)
	if err != nil {
		return domain.DeployedService{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, fmt.Sprintf("%s/apis/serving.knative.dev/v1/namespaces/%s/services/%s?fieldManager=dcloud-container&force=true", m.baseURL, m.nsFor(scope.ProjectID), resourceName), bytes.NewReader(body))
	if err != nil {
		return domain.DeployedService{}, err
	}
	m.authorize(httpReq)
	httpReq.Header.Set("Content-Type", "application/apply-patch+yaml")
	httpReq.Header.Set("Accept", "application/json")

	res, err := m.client.Do(httpReq)
	if err != nil {
		return domain.DeployedService{}, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return domain.DeployedService{}, decodeAPIError(res)
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
		return domain.DeployedService{}, err
	}

	defaultDomainLabels := map[string]string{
		"app.kubernetes.io/instance":   "dcloud",
		"app.kubernetes.io/component":  "container",
		"app.kubernetes.io/managed-by": userServiceManagerLabel,
		userLabelKey:                   scope.UserID,
		projectLabelKey:                scope.ProjectID,
		serviceNameLabel:               req.Name,
	}
	if err := m.applyDomainMapping(ctx, m.nsFor(scope.ProjectID), fmt.Sprintf("%s.%s", resourceName, m.PublicDomain), resourceName, defaultDomainLabels); err != nil {
		return domain.DeployedService{}, err
	}

	service := domain.DeployedService{
		Name:          req.Name,
		Image:         req.Image,
		URL:           m.PublicURL(resourceName),
		ResourceName:  resourceName,
		Namespace:     payload.Metadata.Namespace,
		ProjectID:     scope.ProjectID,
		Generation:    payload.Metadata.Generation,
		CreatedAt:     payload.Metadata.CreationTimestamp.UTC().Format(time.RFC3339),
		UpdatedAt:     payload.Metadata.CreationTimestamp.UTC().Format(time.RFC3339),
		Port:          req.Port,
		MinScale:      req.MinScale,
		MaxScale:      req.MaxScale,
		StartupScript: req.StartupScript,
		Env:           req.Env,
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

func (m *Manager) Delete(ctx context.Context, scope domain.ProjectScope, name, customDomain string) error {
	resourceName := ServiceResourceName(scope.ProjectID, name)
	defaultDomain := fmt.Sprintf("%s.%s", resourceName, m.PublicDomain)

	ns := m.nsFor(scope.ProjectID)
	if customDomain != "" {
		_ = m.DeleteDomainMapping(ctx, scope.ProjectID, customDomain)
	}
	_ = m.DeleteDomainMapping(ctx, scope.ProjectID, defaultDomain)

	legacyPaths := []string{
		fmt.Sprintf("/apis/networking.internal.knative.dev/v1alpha1/namespaces/%s/ingresses/%s", ns, defaultDomain),
		fmt.Sprintf("/api/v1/namespaces/%s/services/%s", ns, resourceName+"-h1gw"),
	}
	if customDomain != "" {
		legacyPaths = append(legacyPaths,
			fmt.Sprintf("/apis/networking.internal.knative.dev/v1alpha1/namespaces/%s/ingresses/%s", ns, customDomain))
	}
	for _, path := range legacyPaths {
		delReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, m.baseURL+path, nil)
		if delReq != nil {
			m.authorize(delReq)
			if delRes, err := m.client.Do(delReq); err == nil {
				delRes.Body.Close()
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/apis/serving.knative.dev/v1/namespaces/%s/services/%s", m.baseURL, ns, resourceName), nil)
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

func (m *Manager) authorize(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+m.token)
}

func ServiceResourceName(projectID, name string) string {
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
	Metadata struct {
		Name        string            `json:"name"`
		Namespace   string            `json:"namespace"`
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations,omitempty"`
	} `json:"metadata"`
	Spec struct {
		Template struct {
			Metadata struct {
				Labels      map[string]string `json:"labels"`
				Annotations map[string]string `json:"annotations,omitempty"`
			} `json:"metadata"`
			Spec struct {
				TimeoutSeconds   *int64             `json:"timeoutSeconds,omitempty"`
				RuntimeClassName string             `json:"runtimeClassName,omitempty"`
				Containers       []knativeContainer `json:"containers"`
				Volumes          []knativeVolume    `json:"volumes,omitempty"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
}

type knativeEnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type knativeContainer struct {
	Name            string                    `json:"name"`
	Image           string                    `json:"image"`
	Ports           []knativeContainerPort    `json:"ports,omitempty"`
	Command         []string                  `json:"command,omitempty"`
	Args            []string                  `json:"args,omitempty"`
	Env             []knativeEnvVar           `json:"env,omitempty"`
	VolumeMounts    []knativeVolumeMount      `json:"volumeMounts,omitempty"`
	SecurityContext *knativeSecurityContext   `json:"securityContext,omitempty"`
}

type knativeVolumeMount struct {
	Name             string `json:"name"`
	MountPath        string `json:"mountPath"`
	MountPropagation string `json:"mountPropagation,omitempty"`
	ReadOnly         bool   `json:"readOnly,omitempty"`
}

type knativeVolume struct {
	Name     string                     `json:"name"`
	EmptyDir *struct{}                  `json:"emptyDir,omitempty"`
	Secret   *knativeVolumeSecretSource `json:"secret,omitempty"`
}

type knativeVolumeSecretSource struct {
	SecretName string `json:"secretName"`
}

type knativeSecurityContext struct {
	Capabilities *knativeCapabilities `json:"capabilities,omitempty"`
	Privileged   *bool                `json:"privileged,omitempty"`
}

type knativeCapabilities struct {
	Add []string `json:"add,omitempty"`
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

func parseScaleAnnotation(v string) int32 {
	if v == "" {
		return 0
	}
	var n int32
	fmt.Sscanf(v, "%d", &n)
	return n
}
