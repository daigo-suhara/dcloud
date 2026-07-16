package rook

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/daigo-suhara/dcloud/internal/storage/domain"
	storagepb "github.com/daigo-suhara/dcloud/internal/pb/storagepb"
)

var (
	ErrInvalidArgument = errors.New("invalid argument")
	ErrNotFound        = errors.New("not found")
	ErrAlreadyExists   = errors.New("already exists")
	ErrUnavailable     = errors.New("unavailable")
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func BucketResourceName(userID, projectID, name string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(userID) + ":" + strings.TrimSpace(projectID) + ":" + strings.TrimSpace(name)))
	return "bkt-" + hex.EncodeToString(sum[:8])
}

type Client struct {
	baseURL string
	client  *http.Client
	token   string
}

func NewClient() (*Client, error) {
	token, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return nil, err
	}
	caCert, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to load kubernetes ca")
	}
	baseURL := strings.TrimRight(env("DCLD_KUBERNETES_API_URL", "https://kubernetes.default.svc"), "/")
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: pool},
	}
	return &Client{
		baseURL: baseURL,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		token: strings.TrimSpace(string(token)),
	}, nil
}

type kubeOBCList struct {
	Items []kubeOBC `json:"items"`
}

type kubeOBC struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Metadata   struct {
		Name              string            `json:"name"`
		Namespace         string            `json:"namespace"`
		Labels            map[string]string `json:"labels"`
		Annotations       map[string]string `json:"annotations"`
		CreationTimestamp string            `json:"creationTimestamp,omitempty"`
	} `json:"metadata"`
	Spec struct {
		StorageClassName   string `json:"storageClassName"`
		GenerateBucketName string `json:"generateBucketName,omitempty"`
		BucketName         string `json:"bucketName,omitempty"`
	} `json:"spec"`
	Status struct {
		Phase string `json:"phase"`
	} `json:"status"`
}

type kubeSecret struct {
	Data map[string]string `json:"data"`
}

type kubeConfigMap struct {
	Data map[string]string `json:"data"`
}

type kubeStatus struct {
	Message string `json:"message"`
	Reason  string `json:"reason"`
	Code    int    `json:"code"`
}

func (c *Client) ListOBCs(ctx context.Context, namespace, userID, projectID string) ([]domain.BucketRecord, error) {
	selector := url.QueryEscape(fmt.Sprintf("dcloud-component=storage,dcloud-user-id=%s,dcloud-project-id=%s", userID, projectID))
	var payload kubeOBCList
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/apis/objectbucket.io/v1alpha1/namespaces/%s/objectbucketclaims?labelSelector=%s", namespace, selector), nil, &payload); err != nil {
		return nil, err
	}
	records := make([]domain.BucketRecord, 0, len(payload.Items))
	for _, item := range payload.Items {
		records = append(records, OBCToRecord(item))
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].CreatedAt == records[j].CreatedAt {
			return records[i].Name < records[j].Name
		}
		return records[i].CreatedAt < records[j].CreatedAt
	})
	return records, nil
}

func (c *Client) CreateOBC(ctx context.Context, namespace, userID, projectID, name, storageClass string) (domain.BucketRecord, error) {
	resourceName := BucketResourceName(userID, projectID, name)
	payload := kubeOBC{
		APIVersion: "objectbucket.io/v1alpha1",
		Kind:       "ObjectBucketClaim",
	}
	payload.Metadata.Name = resourceName
	payload.Metadata.Namespace = namespace
	payload.Metadata.Labels = map[string]string{
		"dcloud-component":       "storage",
		"dcloud-user-id":         userID,
		"dcloud-project-id":      projectID,
		"dcloud-display-name":    name,
		"app.kubernetes.io/name": "dcloud",
	}
	payload.Metadata.Annotations = map[string]string{
		"dcloud/name": name,
	}
	payload.Spec.StorageClassName = storageClass
	payload.Spec.GenerateBucketName = resourceName
	var created kubeOBC
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/apis/objectbucket.io/v1alpha1/namespaces/%s/objectbucketclaims", namespace), payload, &created); err != nil {
		return domain.BucketRecord{}, err
	}
	return OBCToRecord(created), nil
}

func (c *Client) DeleteOBC(ctx context.Context, namespace, resourceName string) error {
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/apis/objectbucket.io/v1alpha1/namespaces/%s/objectbucketclaims/%s", namespace, resourceName), nil, nil)
}

func (c *Client) GetBucketCredentials(ctx context.Context, namespace, resourceName, rgwEndpoint string) (*storagepb.BucketCredentials, error) {
	var secret kubeSecret
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", namespace, resourceName), nil, &secret); err != nil {
		return nil, err
	}
	var cm kubeConfigMap
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/v1/namespaces/%s/configmaps/%s", namespace, resourceName), nil, &cm); err != nil {
		return nil, err
	}
	accessKeyID := decodeBase64(secret.Data["AWS_ACCESS_KEY_ID"])
	secretAccessKey := decodeBase64(secret.Data["AWS_SECRET_ACCESS_KEY"])
	bucketName := cm.Data["BUCKET_NAME"]
	if rgwEndpoint == "" {
		host := cm.Data["BUCKET_HOST"]
		port := cm.Data["BUCKET_PORT"]
		if port != "" && port != "80" && port != "443" {
			rgwEndpoint = "http://" + host + ":" + port
		} else {
			rgwEndpoint = "http://" + host
		}
	}
	return &storagepb.BucketCredentials{
		Endpoint:        rgwEndpoint,
		BucketName:      bucketName,
		AccessKeyId:     accessKeyID,
		SecretAccessKey: secretAccessKey,
	}, nil
}

func decodeBase64(s string) string {
	if s == "" {
		return ""
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return s
	}
	return strings.TrimSpace(string(b))
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		message := kubeErrorMessage(raw)
		switch res.StatusCode {
		case http.StatusBadRequest, http.StatusUnprocessableEntity:
			return fmt.Errorf("%w: %s", ErrInvalidArgument, message)
		case http.StatusConflict:
			return fmt.Errorf("%w: %s", ErrAlreadyExists, message)
		case http.StatusNotFound:
			if isUnavailableMessage(message) {
				return fmt.Errorf("%w: %s", ErrUnavailable, message)
			}
			return fmt.Errorf("%w: %s", ErrNotFound, message)
		default:
			return fmt.Errorf("%s", message)
		}
	}
	if out == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func kubeErrorMessage(raw []byte) string {
	var payload kubeStatus
	if err := json.Unmarshal(raw, &payload); err == nil {
		if payload.Message != "" {
			return payload.Message
		}
		if payload.Reason != "" {
			return payload.Reason
		}
	}
	text := strings.TrimSpace(string(raw))
	if text != "" {
		return text
	}
	return "kubernetes api error"
}

func isUnavailableMessage(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(message, "could not find the requested resource") ||
		strings.Contains(message, "no matches for kind")
}

func OBCToRecord(item kubeOBC) domain.BucketRecord {
	name := ""
	if item.Metadata.Annotations != nil {
		name = strings.TrimSpace(item.Metadata.Annotations["dcloud/name"])
	}
	if name == "" {
		name = item.Metadata.Name
	}
	phase := strings.TrimSpace(item.Status.Phase)
	ready := strings.EqualFold(phase, "Bound")
	if phase == "" {
		phase = "Provisioning"
	}
	return domain.BucketRecord{
		Name:         name,
		Ready:        ready,
		Status:       phase,
		CreatedAt:    item.Metadata.CreationTimestamp,
		ProjectID:    item.Metadata.Labels["dcloud-project-id"],
		ResourceName: item.Metadata.Name,
	}
}
