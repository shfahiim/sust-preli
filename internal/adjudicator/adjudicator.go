package adjudicator

import (
	"context"
	"errors"
	"strings"

	"github.com/sust-cse/queuestorm-investigator/internal/model"
)

var ErrSkipped = errors.New("llm adjudication skipped")

type Adjudicator interface {
	ShouldAdjudicate(req model.Request, ruleResp model.Response) bool
	Adjudicate(ctx context.Context, req model.Request, ruleResp model.Response) (model.Response, error)
}

type Noop struct{}

func (Noop) ShouldAdjudicate(model.Request, model.Response) bool { return false }
func (Noop) Adjudicate(context.Context, model.Request, model.Response) (model.Response, error) {
	return model.Response{}, ErrSkipped
}

func ShouldUseLLM(req model.Request, ruleResp model.Response, minConfidence float64) bool {
	if minConfidence <= 0 || minConfidence > 1 {
		minConfidence = 0.70
	}
	if ruleResp.CaseType == model.CasePhishingSocialEngineering {
		return false
	}
	if ruleResp.RelevantTransactionID != nil && ruleResp.Confidence >= 0.90 && ruleResp.EvidenceVerdict == model.EvidenceConsistent {
		return false
	}
	if hasReason(ruleResp, "ambiguous_match") {
		return true
	}
	if ruleResp.Confidence > 0 && ruleResp.Confidence < minConfidence {
		return true
	}
	if ruleResp.CaseType == model.CaseOther && hasMoneyOrRiskSignal(req.Complaint) {
		return true
	}
	if ruleResp.EvidenceVerdict == model.EvidenceInsufficientData && len(req.TransactionHistory) > 0 && hasMoneyOrRiskSignal(req.Complaint) {
		return true
	}
	if hasMultiIssueSignal(req.Complaint) {
		return true
	}
	return false
}

func hasReason(resp model.Response, code string) bool {
	for _, reason := range resp.ReasonCodes {
		if reason == code {
			return true
		}
	}
	return false
}

func hasMoneyOrRiskSignal(s string) bool {
	n := strings.ToLower(s)
	terms := []string{
		"taka", "tk", "bdt", "payment", "paid", "transfer", "sent", "refund", "cashback", "settlement", "cash in", "cash out", "duplicate", "deduct", "failed", "pending", "otp", "pin", "link",
		"টাকা", "পেমেন্ট", "ট্রান্সফার", "রিফান্ড", "ক্যাশব্যাক", "সেটেলমেন্ট", "ক্যাশ", "কেটেছে", "ফেইল", "পেন্ডিং", "ওটিপি", "পিন", "লিংক",
		"bhul", "vul", "pathai", "kete", "paini", "duibar", "ferot",
	}
	for _, term := range terms {
		if strings.Contains(n, term) {
			return true
		}
	}
	return false
}

func hasMultiIssueSignal(s string) bool {
	n := strings.ToLower(s)
	groups := 0
	if containsAny(n, "wrong", "bhul", "vul", "ভুল") {
		groups++
	}
	if containsAny(n, "failed", "fail", "deduct", "kete", "ফেইল", "কেটে", "কেটেছে") {
		groups++
	}
	if containsAny(n, "refund", "cashback", "ferot", "রিফান্ড", "ফেরত", "ক্যাশব্যাক") {
		groups++
	}
	if containsAny(n, "duplicate", "twice", "double", "duibar", "দুইবার", "ডাবল") {
		groups++
	}
	return groups >= 2
}

func containsAny(s string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(s, term) {
			return true
		}
	}
	return false
}
