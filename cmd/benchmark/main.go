package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type result struct {
	Endpoint         string  `json:"endpoint"`
	Requests         int     `json:"requests"`
	Concurrency      int     `json:"concurrency"`
	AverageLatencyMs int64   `json:"averageLatencyMs"`
	P95LatencyMs     int64   `json:"p95LatencyMs"`
	ErrorCount       int64   `json:"errorCount"`
	ErrorRate        float64 `json:"errorRate"`
	DurationSeconds  int64   `json:"durationSeconds"`
	TokensPerSecond  float64 `json:"tokensPerSecond"`
}

func main() {
	endpoint := env("MODEL_ENDPOINT", "http://localhost:11434")
	provider := env("MODEL_PROVIDER", "ollama")
	model := env("MODEL_NAME", "llama3")
	prompt := env("SAMPLE_PROMPT", "Explain Kubernetes operators in simple terms.")
	requests := envInt("REQUESTS", 20)
	concurrency := envInt("CONCURRENCY", 2)
	if concurrency <= 0 {
		concurrency = 1
	}

	client := &http.Client{Timeout: 120 * time.Second}
	jobs := make(chan int)
	latencies := make([]int64, 0, requests)
	var mu sync.Mutex
	var errors int64
	var tokenTotal int64
	start := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				latency, tokens, err := sendPrompt(client, provider, endpoint, model, prompt)
				if err != nil {
					atomic.AddInt64(&errors, 1)
					fmt.Fprintf(os.Stderr, "benchmark request failed: %v\n", err)
					continue
				}
				atomic.AddInt64(&tokenTotal, int64(tokens))
				mu.Lock()
				latencies = append(latencies, latency)
				mu.Unlock()
			}
		}()
	}

	for i := 0; i < requests; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	duration := time.Since(start)
	res := result{
		Endpoint:        endpoint,
		Requests:        requests,
		Concurrency:     concurrency,
		ErrorCount:      errors,
		ErrorRate:       float64(errors) / float64(requests),
		DurationSeconds: int64(duration.Seconds()),
	}
	if len(latencies) > 0 {
		res.AverageLatencyMs = average(latencies)
		res.P95LatencyMs = percentile(latencies, 0.95)
	}
	if duration.Seconds() > 0 {
		res.TokensPerSecond = float64(tokenTotal) / duration.Seconds()
	}

	encoded, _ := json.MarshalIndent(res, "", "  ")
	fmt.Println(string(encoded))
	if err := writeResultConfigMap(encoded); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write benchmark result ConfigMap: %v\n", err)
	}
	if errors > 0 {
		os.Exit(1)
	}
}

func writeResultConfigMap(encoded []byte) error {
	name := os.Getenv("RESULT_CONFIGMAP")
	namespace := os.Getenv("POD_NAMESPACE")
	if name == "" || namespace == "" {
		return nil
	}
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}
	ctx := context.Background()
	configMaps := clientset.CoreV1().ConfigMaps(namespace)
	cm, err := configMaps.Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Data: map[string]string{
				"status":      "completed",
				"result.json": string(encoded),
			},
		}
		_, err = configMaps.Create(ctx, cm, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data["status"] = "completed"
	cm.Data["result.json"] = string(encoded)
	_, err = configMaps.Update(ctx, cm, metav1.UpdateOptions{})
	return err
}

func sendPrompt(client *http.Client, provider, endpoint, model, prompt string) (int64, int, error) {
	var url string
	var body []byte
	var err error

	switch provider {
	case "vllm":
		url = endpoint + "/v1/completions"
		body, err = json.Marshal(map[string]any{
			"model":      model,
			"prompt":     prompt,
			"max_tokens": 128,
		})
	default:
		url = endpoint + "/api/generate"
		body, err = json.Marshal(map[string]any{
			"model":  model,
			"prompt": prompt,
			"stream": false,
		})
	}
	if err != nil {
		return 0, 0, err
	}

	start := time.Now()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return 0, 0, fmt.Errorf("status %d: %s", resp.StatusCode, string(payload))
	}

	return time.Since(start).Milliseconds(), estimateTokens(payload), nil
}

func estimateTokens(payload []byte) int {
	var parsed map[string]any
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return len(payload) / 4
	}
	if count, ok := parsed["eval_count"].(float64); ok {
		return int(count)
	}
	if choices, ok := parsed["choices"].([]any); ok && len(choices) > 0 {
		encoded, _ := json.Marshal(choices)
		return len(encoded) / 4
	}
	return len(payload) / 4
}

func average(values []int64) int64 {
	var total int64
	for _, value := range values {
		total += value
	}
	return total / int64(len(values))
}

func percentile(values []int64, p float64) int64 {
	if len(values) == 0 {
		return 0
	}
	copyValues := append([]int64(nil), values...)
	for i := 1; i < len(copyValues); i++ {
		value := copyValues[i]
		j := i - 1
		for j >= 0 && copyValues[j] > value {
			copyValues[j+1] = copyValues[j]
			j--
		}
		copyValues[j+1] = value
	}
	index := int(float64(len(copyValues)-1) * p)
	return copyValues[index]
}

func env(name, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
