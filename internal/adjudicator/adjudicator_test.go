package adjudicator

import (
	"testing"

	"github.com/sust-cse/queuestorm-investigator/internal/model"
)

func TestShouldUseLLMForOtherWithMoneySignal(t *testing.T) {
	req := model.Request{TicketID: "T1", Complaint: "amar taka kete geche but bujhtesi na", TransactionHistory: []model.Transaction{{TransactionID: "TXN-1"}}}
	rule := model.Response{TicketID: "T1", CaseType: model.CaseOther, EvidenceVerdict: model.EvidenceInsufficientData, Confidence: 0.60}
	if !ShouldUseLLM(req, rule, 0.70) {
		t.Fatal("expected LLM gate for weak money-related other case")
	}
}

func TestShouldNotUseLLMForClearPhishing(t *testing.T) {
	req := model.Request{TicketID: "T1", Complaint: "someone asked for OTP"}
	rule := model.Response{TicketID: "T1", CaseType: model.CasePhishingSocialEngineering, EvidenceVerdict: model.EvidenceInsufficientData, Confidence: 0.95}
	if ShouldUseLLM(req, rule, 0.70) {
		t.Fatal("clear phishing should stay deterministic")
	}
}

func TestValidateRejectsInventedTransactionID(t *testing.T) {
	id := "TXN-NOT-ALLOWED"
	resp := validResponse("T1")
	resp.RelevantTransactionID = &id
	req := model.Request{TicketID: "T1", Complaint: "payment failed", TransactionHistory: []model.Transaction{{TransactionID: "TXN-1"}}}
	if err := ValidateResponse(resp, req, allowedTransactionIDs(req), model.Response{TicketID: "T1"}); err == nil {
		t.Fatal("expected invented transaction id rejection")
	}
}

func TestValidateRejectsUnsafeReply(t *testing.T) {
	resp := validResponse("T1")
	resp.CustomerReply = "Please send your PIN so we can refund you."
	req := model.Request{TicketID: "T1", Complaint: "help"}
	if err := ValidateResponse(resp, req, allowedTransactionIDs(req), model.Response{TicketID: "T1"}); err == nil {
		t.Fatal("expected unsafe reply rejection")
	}
}

func validResponse(ticketID string) model.Response {
	return model.Response{
		TicketID:              ticketID,
		EvidenceVerdict:       model.EvidenceInsufficientData,
		CaseType:              model.CaseOther,
		Severity:              model.SeverityLow,
		Department:            model.DepartmentCustomerSupport,
		AgentSummary:          "Customer needs support.",
		RecommendedNextAction: "Ask for official transaction details.",
		CustomerReply:         "Please share the transaction ID through official support channels. Do not share your PIN or OTP.",
		HumanReviewRequired:   false,
		Confidence:            0.7,
		ReasonCodes:           []string{"llm_adjudicated"},
	}
}
