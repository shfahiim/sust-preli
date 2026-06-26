package adjudicator

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/sust-cse/queuestorm-investigator/internal/model"
)

var unsafePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(send|share|provide|give|tell|enter|input|type)\s+(us\s+)?(your\s+)?(pin|otp|password|full card|card number)\b`),
	regexp.MustCompile(`(?i)\b(what is|tell us|verify)\s+(your\s+)?(pin|otp|password)\b`),
	regexp.MustCompile(`(?i)\b(we will refund|refund is confirmed|refund has been|money has been recovered|we (have )?reversed|account unblocked|successfully reversed)\b`),
	regexp.MustCompile(`(?i)\b(contact|call|message)\s+(this|that)\s+(number|person|individual)\b`),
}

func ValidateResponse(resp model.Response, req model.Request, allowedIDs map[string]bool, ruleResp model.Response) error {
	if resp.TicketID != req.TicketID {
		return errors.New("llm ticket_id mismatch")
	}
	if resp.RelevantTransactionID != nil {
		if !allowedIDs[*resp.RelevantTransactionID] {
			return fmt.Errorf("llm invented transaction id %q", *resp.RelevantTransactionID)
		}
	}
	if !validEvidence[resp.EvidenceVerdict] || !validCase[resp.CaseType] || !validSeverity[resp.Severity] || !validDepartment[resp.Department] {
		return errors.New("llm response has invalid enum")
	}
	if resp.Confidence < 0 || resp.Confidence > 1 {
		return errors.New("llm confidence out of range")
	}
	if len(resp.AgentSummary) == 0 || len(resp.RecommendedNextAction) == 0 || len(resp.CustomerReply) == 0 {
		return errors.New("llm response missing text fields")
	}
	if unsafeText(resp.CustomerReply) || unsafeText(resp.RecommendedNextAction) {
		return errors.New("llm response failed safety validation")
	}
	if resp.CaseType == model.CasePhishingSocialEngineering && (resp.Severity != model.SeverityCritical || resp.Department != model.DepartmentFraudRisk || !resp.HumanReviewRequired) {
		return errors.New("llm phishing routing invalid")
	}
	if resp.RelevantTransactionID == nil && resp.EvidenceVerdict == model.EvidenceConsistent && ruleResp.EvidenceVerdict != model.EvidenceConsistent {
		return errors.New("llm consistent verdict without transaction")
	}
	if ruleResp.Confidence >= 0.90 && ruleResp.EvidenceVerdict == model.EvidenceConsistent && resp.CaseType != ruleResp.CaseType {
		return errors.New("llm contradicted high-confidence rule result")
	}
	return nil
}

var validEvidence = map[string]bool{
	model.EvidenceConsistent:       true,
	model.EvidenceInconsistent:     true,
	model.EvidenceInsufficientData: true,
}

var validCase = map[string]bool{
	model.CaseWrongTransfer:             true,
	model.CasePaymentFailed:             true,
	model.CaseRefundRequest:             true,
	model.CaseDuplicatePayment:          true,
	model.CaseMerchantSettlementDelay:   true,
	model.CaseAgentCashInIssue:          true,
	model.CasePhishingSocialEngineering: true,
	model.CaseOther:                     true,
}

var validSeverity = map[string]bool{
	model.SeverityLow:      true,
	model.SeverityMedium:   true,
	model.SeverityHigh:     true,
	model.SeverityCritical: true,
}

var validDepartment = map[string]bool{
	model.DepartmentCustomerSupport:    true,
	model.DepartmentDisputeResolution:  true,
	model.DepartmentPaymentsOps:        true,
	model.DepartmentMerchantOperations: true,
	model.DepartmentAgentOperations:    true,
	model.DepartmentFraudRisk:          true,
}

func unsafeText(s string) bool {
	lower := strings.ToLower(s)
	for _, safe := range []string{
		"do not share your pin", "do not share your otp", "never share your pin", "never share your otp", "never ask for your pin", "never ask for your otp",
	} {
		lower = strings.ReplaceAll(lower, safe, "")
	}
	for _, pattern := range unsafePatterns {
		if pattern.MatchString(lower) {
			return true
		}
	}
	return false
}

func allowedTransactionIDs(req model.Request) map[string]bool {
	ids := make(map[string]bool, len(req.TransactionHistory))
	for _, tx := range req.TransactionHistory {
		if tx.TransactionID != "" {
			ids[tx.TransactionID] = true
		}
	}
	return ids
}
