package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
)

func (c *kubeClient) listBackups(ctx context.Context, namespace string, r *dbRecord) ([]*Backup, error) {
	componentName := componentNames[r.Type]
	labelSel := fmt.Sprintf("app.kubernetes.io/instance=%s,apps.kubeblocks.io/component-name=%s", r.ResourceName, componentName)
	var payload struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				BackupMethod string `json:"backupMethod"`
			} `json:"spec"`
			Status struct {
				Phase          string `json:"phase"`
				CompletionTime string `json:"completionTimestamp"`
				TotalSize      string `json:"totalSize"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := c.doJSON(ctx, http.MethodGet,
		fmt.Sprintf("/apis/dataprotection.kubeblocks.io/v1alpha1/namespaces/%s/backups?labelSelector=%s", namespace, labelSel),
		nil, &payload); err != nil {
		return nil, err
	}
	out := make([]*Backup, 0, len(payload.Items))
	for _, item := range payload.Items {
		out = append(out, &Backup{
			Name:        item.Metadata.Name,
			Status:      item.Status.Phase,
			Method:      item.Spec.BackupMethod,
			TotalSize:   item.Status.TotalSize,
			CreatedAt:   item.Metadata.CreationTimestamp,
			CompletedAt: item.Status.CompletionTime,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

func (c *kubeClient) createBackup(ctx context.Context, namespace string, r *dbRecord) (*Backup, error) {
	componentName := componentNames[r.Type]
	policyName := fmt.Sprintf("%s-%s-backup-policy", r.ResourceName, componentName)
	method := backupMethodFor(r.Type)
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	backupName := fmt.Sprintf("%s-backup-%s", r.ResourceName, hex.EncodeToString(buf))
	payload := map[string]any{
		"apiVersion": "dataprotection.kubeblocks.io/v1alpha1",
		"kind":       "Backup",
		"metadata": map[string]any{
			"name":      backupName,
			"namespace": namespace,
			"labels": map[string]string{
				"app.kubernetes.io/instance":            r.ResourceName,
				"apps.kubeblocks.io/component-name":     componentName,
				"dcloud/db-name":                        r.Name,
			},
		},
		"spec": map[string]any{
			"backupPolicyName": policyName,
			"backupMethod":     method,
		},
	}
	var created struct {
		Metadata struct {
			Name              string `json:"name"`
			CreationTimestamp string `json:"creationTimestamp"`
		} `json:"metadata"`
	}
	if err := c.doJSON(ctx, http.MethodPost,
		fmt.Sprintf("/apis/dataprotection.kubeblocks.io/v1alpha1/namespaces/%s/backups", namespace),
		payload, &created); err != nil {
		return nil, err
	}
	return &Backup{
		Name:      created.Metadata.Name,
		Status:    "Running",
		Method:    method,
		CreatedAt: created.Metadata.CreationTimestamp,
	}, nil
}

func (c *kubeClient) deleteBackup(ctx context.Context, namespace, backupName string) error {
	return c.doJSON(ctx, http.MethodDelete,
		fmt.Sprintf("/apis/dataprotection.kubeblocks.io/v1alpha1/namespaces/%s/backups/%s", namespace, backupName),
		nil, nil)
}

func backupMethodFor(dbType string) string {
	switch dbType {
	case "mysql":
		return "xtrabackup"
	case "postgres":
		return "pg-basebackup"
	default:
		return "volume-snapshot"
	}
}
