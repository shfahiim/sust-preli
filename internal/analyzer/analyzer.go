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

	if isPhishingComplaint(norm) {
		return model.CasePhishingSocialEngineering
	}
	if isWrongTransferComplaint(norm) {
		return model.CaseWrongTransfer
	}
	if isDuplicatePaymentComplaint(norm) {
		return model.CaseDuplicatePayment
	}
	if isAgentCashInComplaint(norm) {
		return model.CaseAgentCashInIssue
	}
	if isMerchantSettlementComplaint(req, norm, userType, channel) {
		return model.CaseMerchantSettlementDelay
	}
	if isRefundTransactionComplaint(norm) {
		return model.CaseRefundRequest
	}
	if isPaymentFailedComplaint(norm) {
		return model.CasePaymentFailed
	}
	if isRefundComplaint(norm) {
		return model.CaseRefundRequest
	}
	return model.CaseOther
}

func isPhishingComplaint(norm string) bool {
	if containsAny(norm, "my pin is", "my otp is", "my password is", "আমার পিন", "আমার ওটিপি") && !containsAny(norm, "called", "caller", "sms", "message", "email", "link", "scam", "fake", "asked", "চেয়েছে") {
		return false
	}

	credential := containsAny(norm, "otp", "o.t.p", "pin", "password", "passcode", "verification code", "security code", "ওটিপি", "ও টি পি", "পিন", "পাসওয়ার্ড")
	action := containsAny(norm, "ask", "asked", "asking", "ask for", "call", "called", "caller", "sms", "message", "email", "link", "blocked", "suspend", "suspended", "verify", "share", "send", "unblock", "কলে", "কল", "লিংক", "মেসেজ", "চেয়েছে", "শেয়ার")
	if credential && action {
		return true
	}

	if containsAny(norm, "sms", "message", "email") && containsAny(norm, "link", "click", "bonus", "verify", "suspend", "blocked") {
		return true
	}
	return containsAny(norm,
		"phishing", "scam", "fraud", "suspicious link", "suspicious sms", "fake bkash", "fake support", "pretending", "claiming to be", "claims to be", "said they are from", "bkash officer", "support agent", "official support", "verify your account", "account verify", "account verification", "unblock your account", "account will be blocked", "account blocked", "account suspended", "suspension threat", "click this link", "click here", "login link", "reset link", "bonus link", "social engineering",
		"প্রতার", "স্ক্যাম", "ভুয়া", "লিংক", "ওটিপি", "পিন", "পাসওয়ার্ড",
	)
}

func isWrongTransferComplaint(norm string) bool {
	if containsAny(norm,
		"wrong number", "wrong person", "wrong recipient", "wrong account", "wrong transfer", "typed it wrong", "mistake", "wrongly sent", "sent to wrong", "wrong merchant", "merchant store", "call the receiver", "receiver at",
		"bhul", "vul", "bhool", "vool", "bhul number", "bhul number e", "pathailam", "pathaisi", "pathaise", "pathai", "pathalam", "pathaichi", "taka send koresi", "send koresi",
		"ভুল নাম্বার", "ভুল নাম্বারে", "ভুল নম্বর", "ভুল নম্বরে", "ভুল অ্যাকাউন্ট", "ভুল করে", "পাঠিয়েছি", "পাঠাইছি", "পাঠালাম",
	) {
		return true
	}
	return containsAny(norm, "sent", "send", "transfer", "pathailam", "pathaisi", "pathai", "চলে গেছে", "পাঠিয়েছি", "পাঠাইছি") &&
		containsAny(norm, "didn't get", "did not get", "not received", "didn't receive", "brother", "wrong", "bhul", "vul", "failed", "পায়নি", "পায়নি", "ভুল", "চলে গেছে")
}

func isDuplicatePaymentComplaint(norm string) bool {
	if containsAny(norm, "failed twice", "failing", "fail twice", "failed two times") && !containsAny(norm, "charged twice", "deducted twice", "debited twice", "debited three", "charged three") {
		return false
	}
	return containsAny(norm, "twice", "two times", "three times", "debited three", "debited twice", "double charged", "duplicate", "deducted twice", "charged twice", "charged three", "paid once", "duibar", "dui bar", "দুইবার", "ডাবল")
}

func isAgentCashInComplaint(norm string) bool {
	return containsAny(norm, "cash in", "cash out", "cash-out", "cash-in", "cashin", "cashout", "cash-in", "agent cash", "deposit", "balance not added", "not reflected", "money not received", "ক্যাশ ইন", "ক্যাশ আউট", "ক্যাশইন", "ব্যালেন্স", "টাকা আসেনি") &&
		containsAny(norm, "cash", "cashin", "cashout", "agent", "deposit", "ক্যাশ", "এজেন্ট")
}

func isMerchantSettlementComplaint(req model.Request, norm, userType, channel string) bool {
	return containsAny(norm, "settlement", "settled", "sales", "merchant settlement", "batch status") ||
		((userType == "merchant" || channel == "merchant_portal") && containsAny(norm, "settlement", "settled", "sales", "batch"))
}

func isPaymentFailedComplaint(norm string) bool {
	return containsAny(norm, "failed", "fail holo", "failure", "unsuccessful", "did not go through", "not completed", "pending", "stuck", "processing", "deducted", "balance was deducted", "taka kete", "kete geche", "kete niche", "payment hoy nai", "payment hoyni", "transaction failed", "paid but", "customer paid", "didn't receive confirmation", "did not receive confirmation", "recharge", "reversed", "হয়নি", "হয়নি", "কেটেছে", "কেটে", "পেমেন্ট হয়নি", "রিচার্জ") &&
		containsAny(norm, "pay", "paid", "payment", "transaction", "recharge", "merchant", "biller", "deducted", "pending", "confirmation", "receive", "received", "পেমেন্ট", "রিচার্জ")
}

func isRefundComplaint(norm string) bool {
	return containsAny(norm, "refund", "cashback", "cash back", "return my money", "money back", "changed my mind", "don't want it", "dont want it", "ফেরত")
}

func isRefundTransactionComplaint(norm string) bool {
	return containsAny(norm, "received a refund", "refund of", "refund pending", "refund is pending", "cashback", "cash back")
}

func routeDepartment(ctx analysis) string {
	switch ctx.caseType {
	case model.CaseWrongTransfer:
		return model.DepartmentDisputeResolution
	case model.CasePaymentFailed:
		if strings.ToLower(ctx.req.UserType) == "merchant" || strings.ToLower(ctx.req.Channel) == "merchant_portal" {
			return model.DepartmentMerchantOperations
		}
		return model.DepartmentPaymentsOps
	case model.CaseDuplicatePayment:
		return model.DepartmentPaymentsOps
	case model.CaseMerchantSettlementDelay:
		return model.DepartmentMerchantOperations
	case model.CaseAgentCashInIssue:
		return model.DepartmentAgentOperations
	case model.CasePhishingSocialEngineering:
		return model.DepartmentFraudRisk
	case model.CaseRefundRequest:
		if ctx.relevant != nil && ctx.relevant.Type == model.TxRefund {
			return model.DepartmentPaymentsOps
		}
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
	if smallValue(ctx.relevant) {
		return model.SeverityLow
	}

	switch ctx.caseType {
	case model.CaseDuplicatePayment:
		return model.SeverityHigh
	case model.CaseAgentCashInIssue:
		return model.SeverityHigh
	case model.CasePaymentFailed:
		if veryHighValue(ctx.relevant) {
			return model.SeverityCritical
		}
		if highValue(ctx.relevant) || amountAtLeast(ctx.amounts, 1000) || containsAny(ctx.norm, "deducted", "balance", "taka kete", "কেটেছে") {
			return model.SeverityHigh
		}
		return model.SeverityMedium
	case model.CaseWrongTransfer:
		if veryHighValue(ctx.relevant) {
			return model.SeverityCritical
		}
		if ctx.ambiguous || ctx.relevant == nil {
			return model.SeverityMedium
		}
		if ctx.verdict == model.EvidenceInconsistent {
			return model.SeverityMedium
		}
		return model.SeverityHigh
	case model.CaseMerchantSettlementDelay:
		if veryHighValue(ctx.relevant) {
			return model.SeverityHigh
		}
		return model.SeverityMedium
	case model.CaseRefundRequest:
		if highValue(ctx.relevant) || containsAny(ctx.norm, "sue", "right now", "immediately", "urgent", "pending") {
			return model.SeverityMedium
		}
		return model.SeverityLow
	default:
		return model.SeverityLow
	}
}

func humanReviewRequired(ctx analysis) bool {
	if ctx.caseType == model.CasePhishingSocialEngineering {
		return true
	}
	if ctx.caseType == model.CaseMerchantSettlementDelay {
		return false
	}
	if ctx.severity == model.SeverityCritical {
		return true
	}
	if ctx.caseType == model.CasePaymentFailed && ctx.relevant != nil && (ctx.relevant.Status == model.StatusPending || strings.ToLower(ctx.req.UserType) == "merchant" || strings.ToLower(ctx.req.Channel) == "merchant_portal") {
		return true
	}
	if ctx.caseType == model.CaseRefundRequest && ctx.relevant != nil && ctx.relevant.Type == model.TxRefund && ctx.relevant.Status == model.StatusPending {
		return true
	}
	if ctx.caseType == model.CaseDuplicatePayment {
		return ctx.verdict == model.EvidenceConsistent || ctx.verdict == model.EvidenceInconsistent
	}
	if ctx.caseType == model.CaseAgentCashInIssue {
		return true
	}
	if ctx.caseType == model.CaseWrongTransfer && ctx.relevant != nil {
		return true
	}
	if ctx.verdict == model.EvidenceInconsistent && ctx.relevant != nil {
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
		if ctx.verdict == model.EvidenceInconsistent {
			return []string{"duplicate_claim", "duplicate_not_verified", "evidence_inconsistent"}
		}
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
			if ctx.relevant.Type != model.TxTransfer {
				return []string{"wrong_transfer_claim", "non_transfer_transaction", "evidence_inconsistent"}
			}
			if timeSensitiveTransferContradiction(ctx) {
				return []string{"wrong_transfer_claim", "time_mismatch", "evidence_inconsistent"}
			}
			return []string{"wrong_transfer_claim", "established_recipient_pattern", "evidence_inconsistent"}
		}
		return []string{"wrong_transfer", "transaction_match", "dispute_initiated"}
	default:
		return []string{"vague_complaint", "needs_clarification"}
	}
}

func smallValue(tx *model.Transaction) bool {
	return tx != nil && tx.Amount.Float64() > 0 && tx.Amount.Float64() < 100
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
