package insight

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientFillsContractVersionAndIdempotencyKey(t *testing.T) {
	var got CreateSubjectRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/subjects" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"contractVersion":"1","subjectId":"s1","subject":{"namespace":"n","id":"i"},"title":"t","createdAt":"2026-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	subject, err := NewClient(server.URL).CreateSubject(context.Background(), CreateSubjectRequest{Subject: SubjectRef{Namespace: "n", ID: "i"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.ContractVersion != ContractVersion || len(got.IdempotencyKey) != 32 || subject.SubjectID != "s1" {
		t.Fatalf("request = %+v, subject = %+v", got, subject)
	}
}

func TestClientMapsContractErrorsToTypedErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"contractVersion":"1","error":{"code":"IDEMPOTENCY_CONFLICT","message":"key reused"}}`))
	}))
	defer server.Close()

	_, err := NewClient(server.URL).StartAnalysis(context.Background(), "s1", StartAnalysisRequest{IdempotencyKey: "k"})
	want := &Error{Code: CodeIdempotencyConflict, Message: "key reused", HTTPStatus: http.StatusConflict}
	var got *Error
	if !errors.As(err, &got) || got.Code != want.Code || got.Message != want.Message || got.HTTPStatus != want.HTTPStatus {
		t.Fatalf("err = %#v, want %#v", err, want)
	}
	if !errors.Is(err, &Error{Code: CodeIdempotencyConflict}) {
		t.Fatal("errors.Is must match on the error code")
	}
}

func TestClientRejectsAResponseInAnUnsupportedContractVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"contractVersion":"2","contractSchema":"insight-lab.public-engine"}`))
	}))
	defer server.Close()

	_, err := NewClient(server.URL).GetEngine(context.Background())
	if !errors.Is(err, &Error{Code: CodeUnsupportedContractVersion}) {
		t.Fatalf("err = %v, want UNSUPPORTED_CONTRACT_VERSION", err)
	}
}

func TestClientReportsAnUnreachableServerAsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()

	_, err := NewClient(url).GetEngine(context.Background())
	if !errors.Is(err, &Error{Code: CodeUnavailable}) {
		t.Fatalf("err = %v, want UNAVAILABLE", err)
	}
}

func TestClientReportsANonContractResponseAsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>bad gateway</html>"))
	}))
	defer server.Close()

	_, err := NewClient(server.URL).GetEngine(context.Background())
	var got *Error
	if !errors.As(err, &got) || got.Code != CodeUnavailable || got.HTTPStatus != http.StatusBadGateway {
		t.Fatalf("err = %#v, want UNAVAILABLE with HTTP 502", err)
	}
}

func TestWaitForAnalysisStopsWhenTheContextIsCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"contractVersion":"1","subjectId":"s1","analysisId":"a1","status":"running","createdAt":"2026-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := NewClient(server.URL, WithPollInterval(5*time.Millisecond)).WaitForAnalysis(ctx, "s1", "a1")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestWaitForAnalysisReturnsAFailedRunWithoutAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"contractVersion":"1","subjectId":"s1","analysisId":"a1","status":"failed","error":"no documents","createdAt":"2026-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	run, err := NewClient(server.URL).WaitForAnalysis(context.Background(), "s1", "a1")
	if err != nil || run.Status != "failed" || run.Error != "no documents" {
		t.Fatalf("run = %+v, err = %v", run, err)
	}
}
