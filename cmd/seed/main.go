// seed sends a batch of varied Sentry-protocol test events to a running Bugrail instance.
// Usage: go run ./cmd/seed <DSN>
// Example: go run ./cmd/seed http://abc123@localhost:8080/1
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: seed <DSN>")
		fmt.Fprintln(os.Stderr, "  example: go run ./cmd/seed http://abc123@localhost:8080/1")
		os.Exit(2)
	}

	dsn, err := parseDSN(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid DSN: %v\n", err)
		os.Exit(1)
	}

	client := &http.Client{Timeout: 10 * time.Second}

	sent, failed := 0, 0
	for _, e := range events() {
		var sendErr error
		if e.viaEnvelope {
			sendErr = sendEnvelope(client, dsn, e)
		} else {
			sendErr = sendStore(client, dsn, e)
		}
		if sendErr != nil {
			fmt.Fprintf(os.Stderr, "  FAIL  %s: %v\n", e.title(), sendErr)
			failed++
		} else {
			fmt.Printf("  OK    %s\n", e.title())
			sent++
		}
		time.Sleep(50 * time.Millisecond)
	}

	fmt.Printf("\nSent %d events, %d failed.\n", sent, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

type event struct {
	EventID     string      `json:"event_id"`
	Platform    string      `json:"platform"`
	Environment string      `json:"environment"`
	Level       string      `json:"level"`
	Message     string      `json:"message,omitempty"`
	Culprit     string      `json:"culprit,omitempty"`
	Transaction string      `json:"transaction,omitempty"`
	Exception   *exceptions `json:"exception,omitempty"`
	Fingerprint []string    `json:"fingerprint,omitempty"`
	viaEnvelope bool
}

type exceptions struct {
	Values []exceptionValue `json:"values"`
}

type exceptionValue struct {
	Type       string     `json:"type"`
	Value      string     `json:"value"`
	Stacktrace *stackInfo `json:"stacktrace,omitempty"`
}

type stackInfo struct {
	Frames []frame `json:"frames"`
}

type frame struct {
	Filename string `json:"filename"`
	Function string `json:"function"`
	Lineno   int    `json:"lineno"`
}

func (e event) title() string {
	if e.Exception != nil && len(e.Exception.Values) > 0 {
		v := e.Exception.Values[0]
		return v.Type + ": " + v.Value
	}
	if e.Message != "" {
		return e.Message
	}
	return e.EventID
}

func randomID() string {
	const hex = "0123456789abcdef"
	b := make([]byte, 32)
	for i := range b {
		b[i] = hex[rand.IntN(16)]
	}
	return string(b)
}

func events() []event {
	return []event{
		{
			EventID:     randomID(),
			Platform:    "javascript",
			Environment: "production",
			Level:       "error",
			Culprit:     "checkout.js in submitOrder",
			Exception: &exceptions{Values: []exceptionValue{{
				Type:  "TypeError",
				Value: "Cannot read properties of undefined (reading 'price')",
				Stacktrace: &stackInfo{Frames: []frame{
					{Filename: "~/src/cart/checkout.js", Function: "submitOrder", Lineno: 42},
					{Filename: "~/src/cart/checkout.js", Function: "handleSubmit", Lineno: 87},
					{Filename: "~/src/app.js", Function: "onClick", Lineno: 12},
				}},
			}}},
		},
		{
			EventID:     randomID(),
			Platform:    "javascript",
			Environment: "production",
			Level:       "error",
			Culprit:     "checkout.js in submitOrder",
			Exception: &exceptions{Values: []exceptionValue{{
				Type:  "TypeError",
				Value: "Cannot read properties of undefined (reading 'price')",
			}}},
		},
		{
			EventID:     randomID(),
			Platform:    "python",
			Environment: "production",
			Level:       "error",
			Culprit:     "worker.process_job",
			Exception: &exceptions{Values: []exceptionValue{{
				Type:  "KeyError",
				Value: "'user_id'",
				Stacktrace: &stackInfo{Frames: []frame{
					{Filename: "worker.py", Function: "process_job", Lineno: 118},
					{Filename: "queue.py", Function: "run", Lineno: 55},
				}},
			}}},
			viaEnvelope: true,
		},
		{
			EventID:     randomID(),
			Platform:    "python",
			Environment: "staging",
			Level:       "warning",
			Culprit:     "auth.verify_token",
			Exception: &exceptions{Values: []exceptionValue{{
				Type:  "jwt.ExpiredSignatureError",
				Value: "Signature has expired",
			}}},
			viaEnvelope: true,
		},
		{
			EventID:     randomID(),
			Platform:    "php",
			Environment: "production",
			Level:       "error",
			Culprit:     "App\\Http\\Controllers\\UserController::show",
			Exception: &exceptions{Values: []exceptionValue{{
				Type:  "Illuminate\\Database\\Eloquent\\ModelNotFoundException",
				Value: "No query results for model [App\\Models\\User]",
				Stacktrace: &stackInfo{Frames: []frame{
					{Filename: "app/Http/Controllers/UserController.php", Function: "show", Lineno: 34},
					{Filename: "vendor/laravel/framework/src/Illuminate/Routing/Controller.php", Function: "callAction", Lineno: 54},
				}},
			}}},
		},
		{
			EventID:     randomID(),
			Platform:    "go",
			Environment: "production",
			Level:       "fatal",
			Culprit:     "github.com/acme/api/internal/db.(*Client).Query",
			Exception: &exceptions{Values: []exceptionValue{{
				Type:  "runtime error",
				Value: "invalid memory address or nil pointer dereference",
				Stacktrace: &stackInfo{Frames: []frame{
					{Filename: "internal/db/client.go", Function: "(*Client).Query", Lineno: 77},
					{Filename: "internal/handler/orders.go", Function: "ListOrders", Lineno: 23},
				}},
			}}},
			viaEnvelope: true,
		},
		{
			EventID:     randomID(),
			Platform:    "python",
			Environment: "production",
			Level:       "error",
			Culprit:     "payments.charge",
			Fingerprint: []string{"stripe-timeout"},
			Exception: &exceptions{Values: []exceptionValue{{
				Type:  "stripe.error.APIConnectionError",
				Value: "Request timed out after 30.0 seconds",
			}}},
		},
		{
			EventID:     randomID(),
			Platform:    "python",
			Environment: "production",
			Level:       "error",
			Culprit:     "payments.charge",
			Fingerprint: []string{"stripe-timeout"},
			Exception: &exceptions{Values: []exceptionValue{{
				Type:  "stripe.error.APIConnectionError",
				Value: "Request timed out after 30.0 seconds",
			}}},
		},
		{
			EventID:     randomID(),
			Platform:    "javascript",
			Environment: "production",
			Level:       "info",
			Message:     "Feature flag evaluation failed: flag 'new-checkout' not found",
			Culprit:     "flags.evaluate",
		},
	}
}

type dsnConfig struct {
	publicKey string
	baseURL   string
	projectID string
}

func parseDSN(raw string) (dsnConfig, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return dsnConfig{}, err
	}
	if u.User == nil || u.User.Username() == "" {
		return dsnConfig{}, fmt.Errorf("missing public key in DSN userinfo")
	}
	projectID := strings.Trim(u.Path, "/")
	if projectID == "" {
		return dsnConfig{}, fmt.Errorf("missing project ID in DSN path")
	}
	base := u.Scheme + "://" + u.Host
	return dsnConfig{publicKey: u.User.Username(), baseURL: base, projectID: projectID}, nil
}

func sendStore(client *http.Client, dsn dsnConfig, e event) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/api/%s/store/?sentry_key=%s", dsn.baseURL, dsn.projectID, dsn.publicKey)
	resp, err := client.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func sendEnvelope(client *http.Client, dsn dsnConfig, e event) error {
	header, err := json.Marshal(map[string]string{"event_id": e.EventID})
	if err != nil {
		return err
	}
	itemHeader, err := json.Marshal(map[string]string{"type": "event"})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(e)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.Write(header)
	buf.WriteByte('\n')
	buf.Write(itemHeader)
	buf.WriteByte('\n')
	buf.Write(payload)
	buf.WriteByte('\n')

	endpoint := fmt.Sprintf("%s/api/%s/envelope/?sentry_key=%s", dsn.baseURL, dsn.projectID, dsn.publicKey)
	resp, err := client.Post(endpoint, "application/x-sentry-envelope", &buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}
