package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func TestAnalyzeUsesAdjudicatorWhenEnabled(t *testing.T) {
	llmResp := model.Response{
		TicketID:              "TKT-LLM",
		EvidenceVerdict:       model.EvidenceInsufficientData,
		CaseType:              model.CaseOther,
		Severity:              model.SeverityLow,
		Department:            model.DepartmentCustomerSupport,
		AgentSummary:          "LLM summary.",
		RecommendedNextAction: "LLM action.",
		CustomerReply:         "LLM reply through official support channels.",
		HumanReviewRequired:   false,
		Confidence:            0.8,
		ReasonCodes:           []string{"llm_adjudicated"},
	}
	srv := api.NewServerWithAdjudicator(analyzer.New(), fakeAdjudicator{enabled: true, resp: llmResp}).Routes()
	rr := postJSON(srv, `{"ticket_id":"TKT-LLM","complaint":"Something happened with my taka but I do not know what","transaction_history":[{"transaction_id":"TXN-1","type":"payment","amount":100,"counterparty":"M","status":"completed"}]}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status got %d body=%s", rr.Code, rr.Body.String())
	}
	var got model.Response
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.AgentSummary != "LLM summary." || got.Confidence != 0.8 {
		t.Fatalf("expected adjudicator response, got %#v", got)
	}
}

func TestAnalyzeFallsBackWhenAdjudicatorFails(t *testing.T) {
	srv := api.NewServerWithAdjudicator(analyzer.New(), fakeAdjudicator{enabled: true, err: errors.New("timeout")}).Routes()
	rr := postJSON(srv, `{"ticket_id":"TKT-FALLBACK","complaint":"I paid 1200 taka but payment failed","transaction_history":[{"transaction_id":"TXN-1","type":"payment","amount":1200,"counterparty":"M","status":"failed"}]}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status got %d body=%s", rr.Code, rr.Body.String())
	}
	var got model.Response
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.CaseType != model.CasePaymentFailed || got.RelevantTransactionID == nil || *got.RelevantTransactionID != "TXN-1" {
		t.Fatalf("expected deterministic fallback, got %#v", got)
	}
}

type fakeAdjudicator struct {
	enabled bool
	resp    model.Response
	err     error
}

func (f fakeAdjudicator) ShouldAdjudicate(model.Request, model.Response) bool { return f.enabled }
func (f fakeAdjudicator) Adjudicate(context.Context, model.Request, model.Response) (model.Response, error) {
	if f.err != nil {
		return model.Response{}, f.err
	}
	return f.resp, nil
}

func postJSON(handler http.Handler, body string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/analyze-ticket", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rr, req)
	return rr
}
