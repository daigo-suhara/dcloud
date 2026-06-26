package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type podSummary struct {
	Name           string
	Phase          string
	CreationTime   time.Time
	ContainerNames []string
}

// listServingPods returns pods for the Knative service ordered newest-first.
func (m *knativeServiceManager) listServingPods(ctx context.Context, resourceName string) ([]podSummary, error) {
	selector := url.QueryEscape("serving.knative.dev/service=" + resourceName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/api/v1/namespaces/%s/pods?labelSelector=%s", m.baseURL, m.namespace, selector), nil)
	if err != nil {
		return nil, err
	}
	m.authorize(req)
	req.Header.Set("Accept", "application/json")
	res, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kubernetes pods api returned status %d", res.StatusCode)
	}

	var payload struct {
		Items []struct {
			Metadata struct {
				Name              string    `json:"name"`
				CreationTimestamp time.Time `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				Containers []struct {
					Name string `json:"name"`
				} `json:"containers"`
			} `json:"spec"`
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := make([]podSummary, 0, len(payload.Items))
	for _, item := range payload.Items {
		names := make([]string, 0, len(item.Spec.Containers))
		for _, c := range item.Spec.Containers {
			names = append(names, c.Name)
		}
		out = append(out, podSummary{
			Name:           item.Metadata.Name,
			Phase:          item.Status.Phase,
			CreationTime:   item.Metadata.CreationTimestamp,
			ContainerNames: names,
		})
	}
	// Prefer Running pods, then newest first.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			a, b := out[i], out[j]
			aRunning := a.Phase == "Running"
			bRunning := b.Phase == "Running"
			swap := false
			if bRunning && !aRunning {
				swap = true
			} else if bRunning == aRunning && b.CreationTime.After(a.CreationTime) {
				swap = true
			}
			if swap {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

// pickUserContainerName returns the user container name (not queue-proxy/sidecars).
func pickUserContainerName(p podSummary) string {
	for _, n := range p.ContainerNames {
		if n == "queue-proxy" || strings.HasSuffix(n, "-sidecar") {
			continue
		}
		return n
	}
	if len(p.ContainerNames) > 0 {
		return p.ContainerNames[0]
	}
	return ""
}

// streamPodLogs opens a streaming log connection. The caller must call resp.Body.Close().
// tailLines <= 0 means: stream all (k8s default).
func (m *knativeServiceManager) streamPodLogs(ctx context.Context, podName, containerName string, tailLines int32, follow bool) (io.ReadCloser, error) {
	q := url.Values{}
	q.Set("container", containerName)
	q.Set("timestamps", "true")
	if follow {
		q.Set("follow", "true")
	}
	if tailLines > 0 {
		q.Set("tailLines", fmt.Sprintf("%d", tailLines))
	}
	endpoint := fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s/log?%s", m.baseURL, m.namespace, podName, q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	m.authorize(req)
	req.Header.Set("Accept", "text/plain")
	res, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return nil, fmt.Errorf("kubernetes log api returned %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return res.Body, nil
}

// forwardLogLines reads from the k8s log body line by line and invokes emit
// for each one. Lines are split on '\n' and the trailing newline is removed.
// k8s emits timestamps prepended like "2026-06-26T12:00:00.000Z message".
func forwardLogLines(ctx context.Context, body io.Reader, emit func(timestamp, text string) error) error {
	reader := bufio.NewReaderSize(body, 64*1024)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line, err := reader.ReadString('\n')
		if line != "" {
			line = strings.TrimRight(line, "\r\n")
			ts, rest := splitTimestamp(line)
			if err := emit(ts, rest); err != nil {
				return err
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

// splitTimestamp peels off a k8s RFC3339Nano timestamp at the beginning of the
// line, returning ("", line) if none.
func splitTimestamp(line string) (string, string) {
	idx := strings.IndexByte(line, ' ')
	if idx <= 0 {
		return "", line
	}
	tsCandidate := line[:idx]
	if _, err := time.Parse(time.RFC3339Nano, tsCandidate); err != nil {
		return "", line
	}
	return tsCandidate, line[idx+1:]
}

