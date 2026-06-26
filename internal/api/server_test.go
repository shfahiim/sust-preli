package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sust-cse/queuestorm-investigator/internal/analyzer"
	"github.com/sust-cse/queuestorm-investigator/internal/api"
	"github.com/sust-cse/queuestorm-investigator/internal/model"
)

func TestHealth(t *testing.T) {
	srv := api.NewServer(analyzer.New()).Routes()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status got %d", rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("body got %q", rr.Body.String())
	}
}

func TestAnalyzeValidRequest(t *testing.T) {
	srv := api.NewServer(analyzer.New()).Routes()
	body := `{"ticket_id":"TKT-1","complaint":"I paid 1200 taka but payment failed and balance was deducted","transaction_history":[{"transaction_id":"TXN-1","timestamp":"2026-04-14T16:00:00Z","type":"payment","amount":1200,"counterparty":"MERCHANT-X","status":"failed"}]}`
	rr := postJSON(srv, body)

	if rr.Code != http.StatusOK {
		t.Fatalf("status got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp model.Response
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if resp.TicketID != "TKT-1" || resp.CaseType != model.CasePaymentFailed || resp.RelevantTransactionID == nil {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestAnalyzeInvalidJSON(t *testing.T) {
	srv := api.NewServer(analyzer.New()).Routes()
	rr := postJSON(srv, `{bad json`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status got %d", rr.Code)
	}
}

func TestAnalyzeMissingRequiredFields(t *testing.T) {
	srv := api.NewServer(analyzer.New()).Routes()
	rr := postJSON(srv, `{"complaint":"hello"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status got %d", rr.Code)
	}
}

func TestAnalyzeEmptyComplaint(t *testing.T) {
	srv := api.NewServer(analyzer.New()).Routes()
	rr := postJSON(srv, `{"ticket_id":"TKT-1","complaint":"   "}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status got %d", rr.Code)
	}
}

func TestAnalyzeAcceptsStringAmount(t *testing.T) {
	srv := api.NewServer(analyzer.New()).Routes()
	body := `{"ticket_id":"TKT-1","complaint":"I paid 1200 taka but payment failed","transaction_history":[{"transaction_id":"TXN-1","type":"payment","amount":"1200","counterparty":"M","status":"failed"}]}`
	rr := postJSON(srv, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status got %d body=%s", rr.Code, rr.Body.String())
	}
}

func postJSON(handler http.Handler, body string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/analyze-ticket", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rr, req)
	return rr
}
