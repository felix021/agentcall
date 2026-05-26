package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/felix021/agentcall/internal/sharedtypes"
)

func TestServerAcceptsSingleValidCallback(t *testing.T) {
	srv, err := NewServer("token-1", 10*time.Second)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer srv.Close(context.Background())

	payload := sharedtypes.CallbackPayload{
		Token:       "token-1",
		Status:      "ok",
		ContentType: "text/plain",
		Content:     "done",
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(srv.URL()+"/callback", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("http.Post() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}

	got := <-srv.Results()
	if got.Payload.Token != "token-1" || got.Payload.Status != "ok" {
		t.Fatalf("payload = %+v", got.Payload)
	}
}

func TestServerRejectsDuplicateCallbackWithConflict(t *testing.T) {
	srv, err := NewServer("token-dup", 10*time.Second)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer srv.Close(context.Background())

	body := []byte(`{"token":"token-dup","status":"ok","content_type":"text/plain","content":"done"}`)

	resp, err := http.Post(srv.URL()+"/callback", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("http.Post() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("first StatusCode = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}

	resp, err = http.Post(srv.URL()+"/callback", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("http.Post() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("second StatusCode = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}

func TestServerRejectsWrongTokenWithoutConsumingSlot(t *testing.T) {
	srv, err := NewServer("token-2", 10*time.Second)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer srv.Close(context.Background())

	bad := []byte(`{"token":"wrong","status":"ok","content_type":"text/plain","content":"x"}`)
	resp, err := http.Post(srv.URL()+"/callback", "application/json", bytes.NewReader(bad))
	if err != nil {
		t.Fatalf("http.Post() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	good := []byte(`{"token":"token-2","status":"ok","content_type":"text/plain","content":"y"}`)
	resp, err = http.Post(srv.URL()+"/callback", "application/json", bytes.NewReader(good))
	if err != nil {
		t.Fatalf("http.Post() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
}

func TestServerRejectsMalformedJSONWithoutConsumingSlot(t *testing.T) {
	srv, err := NewServer("token-3", 10*time.Second)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer srv.Close(context.Background())

	resp, err := http.Post(srv.URL()+"/callback", "application/json", bytes.NewBufferString(`{"token":"token-3"`))
	if err != nil {
		t.Fatalf("http.Post() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	good := []byte(`{"token":"token-3","status":"ok","content_type":"text/plain","content":"z"}`)
	resp, err = http.Post(srv.URL()+"/callback", "application/json", bytes.NewReader(good))
	if err != nil {
		t.Fatalf("http.Post() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
}

func TestServerRejectsIncompletePayloadWithoutConsumingSlot(t *testing.T) {
	srv, err := NewServer("token-4", 10*time.Second)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer srv.Close(context.Background())

	resp, err := http.Post(srv.URL()+"/callback", "application/json", bytes.NewBufferString(`{"token":"token-4"}`))
	if err != nil {
		t.Fatalf("http.Post() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	body := readBody(t, resp)
	for _, needle := range []string{"missing required fields", "status", "content_type", "content"} {
		if !strings.Contains(body, needle) {
			t.Fatalf("response body = %q, want substring %q", body, needle)
		}
	}

	good := []byte(`{"token":"token-4","status":"ok","content_type":"text/plain","content":"done"}`)
	resp, err = http.Post(srv.URL()+"/callback", "application/json", bytes.NewReader(good))
	if err != nil {
		t.Fatalf("http.Post() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
}

func TestServerRejectsTrailingJSONWithoutConsumingSlot(t *testing.T) {
	srv, err := NewServer("token-5", 10*time.Second)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer srv.Close(context.Background())

	resp, err := http.Post(srv.URL()+"/callback", "application/json", bytes.NewBufferString(`{"token":"token-5","status":"ok","content_type":"text/plain","content":"x"}{"extra":true}`))
	if err != nil {
		t.Fatalf("http.Post() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	good := []byte(`{"token":"token-5","status":"ok","content_type":"text/plain","content":"done"}`)
	resp, err = http.Post(srv.URL()+"/callback", "application/json", bytes.NewReader(good))
	if err != nil {
		t.Fatalf("http.Post() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
}

func TestServerRejectsUnsupportedStatusWithoutConsumingSlot(t *testing.T) {
	srv, err := NewServer("token-6", 10*time.Second)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer srv.Close(context.Background())

	resp, err := http.Post(srv.URL()+"/callback", "application/json", bytes.NewBufferString(`{"token":"token-6","status":"timed_out","content_type":"text/plain","content":"x"}`))
	if err != nil {
		t.Fatalf("http.Post() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	good := []byte(`{"token":"token-6","status":"refused","content_type":"text/plain","content":"done"}`)
	resp, err = http.Post(srv.URL()+"/callback", "application/json", bytes.NewReader(good))
	if err != nil {
		t.Fatalf("http.Post() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
}

func TestServerRejectsUnknownTopLevelFieldWithoutConsumingSlot(t *testing.T) {
	srv, err := NewServer("token-7", 10*time.Second)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer srv.Close(context.Background())

	resp, err := http.Post(srv.URL()+"/callback", "application/json", bytes.NewBufferString(`{"token":"token-7","status":"ok","content_type":"text/plain","content":"x","unexpected":"y"}`))
	if err != nil {
		t.Fatalf("http.Post() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	good := []byte(`{"token":"token-7","status":"ok","content_type":"text/plain","content":"done"}`)
	resp, err = http.Post(srv.URL()+"/callback", "application/json", bytes.NewReader(good))
	if err != nil {
		t.Fatalf("http.Post() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
}

func TestServerRejectsEmptyRequiredFieldsWithFieldNames(t *testing.T) {
	srv, err := NewServer("token-8", 10*time.Second)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer srv.Close(context.Background())

	resp, err := http.Post(srv.URL()+"/callback", "application/json", bytes.NewBufferString(`{"token":"token-8","status":"","content_type":"","content":"done"}`))
	if err != nil {
		t.Fatalf("http.Post() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	body := readBody(t, resp)
	for _, needle := range []string{"empty required fields", "status", "content_type"} {
		if !strings.Contains(body, needle) {
			t.Fatalf("response body = %q, want substring %q", body, needle)
		}
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v", err)
	}

	return string(body)
}
