package analyzer

import (
	"sort"
	"strings"

	"github.com/sust-cse/queuestorm-investigator/internal/model"
)

type Analyzer struct{}

func New() *Analyzer {
	return &Analyzer{}
}

type analysis struct {
	req             model.Request
	norm            string
	language        string
	amounts         []float64
	caseType        string
	relevant        *model.Transaction
	verdict         string
	department      string
	severity        string
	review          bool
	confidence      float64
	reasonCodes     []string
	ambiguous       bool
	duplicateFirst  *model.Transaction
	duplicateSecond *model.Transaction
	established     bool
}

func (a *Analyzer) Analyze(req model.Request) model.Response {
	ctx := analysis{
		req:      req,
		norm:     normalize(req.Complaint),
		language: detectLanguage(req.Language, req.Complaint),
	}
	ctx.amounts = extractAmounts(ctx.norm)
	sort.Float64s(ctx.amounts)

	ctx.caseType = classify(req, ctx.norm)
	ctx.relevant, ctx.ambiguous, ctx.duplicateFirst, ctx.duplicateSecond = selectRelevant(ctx.caseType, ctx.norm, ctx.amounts, req.TransactionHistory)
	ctx.established = hasEstablishedRecipientPattern(ctx.relevant, req.TransactionHistory)
	ctx.verdict = evidenceVerdict(ctx)
	ctx.department = routeDepartment(ctx)
	ctx.severity = severity(ctx)
	ctx.review = humanReviewRequired(ctx)
	ctx.confidence = confidence(ctx)
	ctx.reasonCodes = reasonCodes(ctx)

	summary, action, reply := renderTemplates(ctx)
	summary = sanitizeText(summary)
	action = sanitizeText(action)
	reply = sanitizeText(reply)

	var txID *string
	if ctx.relevant != nil {
		id := ctx.relevant.TransactionID
		txID = &id
	}

	return model.Response{
		TicketID:              req.TicketID,
		RelevantTransactionID: txID,
		EvidenceVerdict:       ctx.verdict,
		CaseType:              ctx.caseType,
		Severity:              ctx.severity,
		Department:            ctx.department,
		AgentSummary:          summary,
		RecommendedNextAction: action,
		CustomerReply:         reply,
		HumanReviewRequired:   ctx.review,
		Confidence:            ctx.confidence,
		ReasonCodes:           ctx.reasonCodes,
	}
}

func classify(req model.Request, norm string) string {
	userType := strings.ToLower(req.UserType)
	channel := strings.ToLower(req.Channel)

	if containsAny(norm, "otp", "o.t.p", "pin", "password", "passcode", "verification code", "security code", "ওটিপি", "পিন", "পাসওয়ার্ড") &&
		containsAny(norm, "ask", "asked", "asking", "ask for", "call", "called", "sms", "link", "blocked", "verify", "share", "send", "কলে", "কল", "চেয়েছে", "শেয়ার") {
		return model.CasePhishingSocialEngineering
	}
	if containsAny(norm, "phishing", "scam", "fraud", "suspicious link", "fake bkash", "pretending", "social engineering") {
		return model.CasePhishingSocialEngineering
	}
	if containsAny(norm, "twice", "two times", "double charged", "duplicate", "deducted twice", "charged twice", "paid once", "দুইবার", "ডাবল") {
		return model.CaseDuplicatePayment
	}
	if userType == "merchant" || channel == "merchant_portal" || containsAny(norm, "settlement", "settled", "sales", "merchant settlement") {
		if containsAny(norm, "settlement", "settled", "sales", "merchant") {
			return model.CaseMerchantSettlementDelay
		}
	}
	if containsAny(norm, "cash in", "cash-in", "cashin", "agent", "deposit", "balance not added", "not reflected", "ক্যাশ ইন", "এজেন্ট", "ব্যালেন্স", "টাকা আসেনি") {
		if containsAny(norm, "cash", "agent", "deposit", "ক্যাশ", "এজেন্ট") {
			return model.CaseAgentCashInIssue
		}
	}
	if containsAny(norm, "failed", "failure", "unsuccessful", "did not go through", "deducted", "balance was deducted", "payment hoy nai", "পেমেন্ট হয়নি", "recharge") &&
		containsAny(norm, "pay", "payment", "recharge", "merchant", "biller", "deducted", "পেমেন্ট") {
		return model.CasePaymentFailed
	}
	if containsAny(norm, "wrong number", "wrong person", "wrong recipient", "typed it wrong", "mistake", "wrongly sent", "sent to wrong", "ভুল নাম্বার", "ভুল নম্বর") {
		return model.CaseWrongTransfer
	}
	if containsAny(norm, "sent", "transfer", "pathailam", "পাঠিয়েছি", "পাঠাইছি") && containsAny(norm, "didn't get", "did not get", "not received", "didn't receive", "brother", "wrong") {
		return model.CaseWrongTransfer
	}
	if containsAny(norm, "refund", "return my money", "money back", "changed my mind", "don't want it", "dont want it", "ফেরত") {
		return model.CaseRefundRequest
	}
	return model.CaseOther
}

func routeDepartment(ctx analysis) string {
	switch ctx.caseType {
	case model.CaseWrongTransfer:
		return model.DepartmentDisputeResolution
	case model.CasePaymentFailed, model.CaseDuplicatePayment:
		return model.DepartmentPaymentsOps
	case model.CaseMerchantSettlementDelay:
		return model.DepartmentMerchantOperations
	case model.CaseAgentCashInIssue:
		return model.DepartmentAgentOperations
	case model.CasePhishingSocialEngineering:
		return model.DepartmentFraudRisk
	case model.CaseRefundRequest:
		if ctx.verdict == model.EvidenceInconsistent || highValue(ctx.relevant) {
			return model.DepartmentDisputeResolution
		}
		return model.DepartmentCustomerSupport
	default:
		return model.DepartmentCustomerSupport
	}
}

func severity(ctx analysis) string {
	if ctx.caseType == model.CasePhishingSocialEngineering {
		return model.SeverityCritical
	}
	if veryHighValue(ctx.relevant) {
		return model.SeverityCritical
	}

	switch ctx.caseType {
	case model.CaseDuplicatePayment:
		return model.SeverityHigh
	case model.CaseAgentCashInIssue:
		return model.SeverityHigh
	case model.CasePaymentFailed:
		if highValue(ctx.relevant) || amountAtLeast(ctx.amounts, 1000) || containsAny(ctx.norm, "deducted", "balance") {
			return model.SeverityHigh
		}
		return model.SeverityMedium
	case model.CaseWrongTransfer:
		if ctx.ambiguous || ctx.relevant == nil {
			return model.SeverityMedium
		}
		if ctx.verdict == model.EvidenceInconsistent {
			return model.SeverityMedium
		}
		return model.SeverityHigh
	case model.CaseMerchantSettlementDelay:
		return model.SeverityMedium
	case model.CaseRefundRequest:
		if highValue(ctx.relevant) {
			return model.SeverityMedium
		}
		return model.SeverityLow
	default:
		return model.SeverityLow
	}
}

func humanReviewRequired(ctx analysis) bool {
	if ctx.caseType == model.CasePhishingSocialEngineering || ctx.severity == model.SeverityCritical {
		return true
	}
	if ctx.caseType == model.CaseDuplicatePayment || ctx.caseType == model.CaseAgentCashInIssue {
		return true
	}
	if ctx.caseType == model.CaseWrongTransfer && ctx.relevant != nil {
		return true
	}
	if ctx.verdict == model.EvidenceInconsistent && ctx.relevant != nil {
		return true
	}
	if veryHighValue(ctx.relevant) {
		return true
	}
	return false
}

func confidence(ctx analysis) float64 {
	switch ctx.caseType {
	case model.CasePhishingSocialEngineering:
		return 0.95
	case model.CaseDuplicatePayment:
		return 0.93
	case model.CaseMerchantSettlementDelay:
		return 0.92
	case model.CaseAgentCashInIssue:
		return 0.88
	case model.CaseRefundRequest:
		return 0.85
	case model.CasePaymentFailed:
		return 0.90
	case model.CaseWrongTransfer:
		if ctx.ambiguous || ctx.relevant == nil {
			return 0.65
		}
		if ctx.verdict == model.EvidenceInconsistent {
			return 0.75
		}
		return 0.90
	default:
		return 0.60
	}
}

func reasonCodes(ctx analysis) []string {
	switch ctx.caseType {
	case model.CasePhishingSocialEngineering:
		return []string{"phishing", "credential_protection", "critical_escalation"}
	case model.CaseDuplicatePayment:
		return []string{"duplicate_payment", "biller_verification_required"}
	case model.CaseMerchantSettlementDelay:
		codes := []string{"merchant_settlement", "delay"}
		if ctx.relevant != nil && ctx.relevant.Status != "" {
			codes = append(codes, ctx.relevant.Status)
		}
		return codes
	case model.CaseAgentCashInIssue:
		codes := []string{"agent_cash_in"}
		if ctx.relevant != nil && ctx.relevant.Status == model.StatusPending {
			codes = append(codes, "pending_transaction")
		}
		return append(codes, "agent_ops")
	case model.CasePaymentFailed:
		return []string{"payment_failed", "potential_balance_deduction"}
	case model.CaseRefundRequest:
		return []string{"refund_request", "merchant_policy_dependent"}
	case model.CaseWrongTransfer:
		if ctx.ambiguous || ctx.relevant == nil {
			return []string{"ambiguous_match", "needs_clarification"}
		}
		if ctx.verdict == model.EvidenceInconsistent {
			return []string{"wrong_transfer_claim", "established_recipient_pattern", "evidence_inconsistent"}
		}
		return []string{"wrong_transfer", "transaction_match", "dispute_initiated"}
	default:
		return []string{"vague_complaint", "needs_clarification"}
	}
}

func highValue(tx *model.Transaction) bool {
	return tx != nil && tx.Amount.Float64() >= 10000
}

func veryHighValue(tx *model.Transaction) bool {
	return tx != nil && tx.Amount.Float64() >= 50000
}

func amountAtLeast(amounts []float64, min float64) bool {
	for _, amount := range amounts {
		if amount >= min {
			return true
		}
	}
	return false
}
