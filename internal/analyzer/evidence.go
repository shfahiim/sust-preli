package analyzer

import (
	"sort"
	"strings"
	"time"

	"github.com/sust-cse/queuestorm-investigator/internal/model"
)

type scoredTx struct {
	tx        *model.Transaction
	score     int
	exactID   bool
	amountHit bool
}

func selectRelevant(caseType, norm string, amounts []float64, history []model.Transaction) (*model.Transaction, bool, *model.Transaction, *model.Transaction) {
	if len(history) == 0 || caseType == model.CaseOther || caseType == model.CasePhishingSocialEngineering {
		return nil, false, nil, nil
	}

	if caseType == model.CaseDuplicatePayment {
		first, second := findDuplicatePayment(history, amounts)
		if second != nil {
			return second, false, first, second
		}
		if candidate := latestMatchingPayment(history, amounts); candidate != nil {
			return candidate, false, nil, nil
		}
	}

	scores := make([]scoredTx, 0, len(history))
	phones := extractPhones(norm)
	for i := range history {
		tx := &history[i]
		score, exactID, amountHit := scoreTransaction(caseType, norm, amounts, phones, tx)
		scores = append(scores, scoredTx{tx: tx, score: score, exactID: exactID, amountHit: amountHit})
	}

	sort.SliceStable(scores, func(i, j int) bool {
		if scores[i].score == scores[j].score {
			return scores[i].tx.Timestamp > scores[j].tx.Timestamp
		}
		return scores[i].score > scores[j].score
	})

	if len(scores) == 0 {
		return nil, false, nil, nil
	}
	if scores[0].score < 50 {
		if candidate := singleObviousCandidate(caseType, history); candidate != nil {
			return candidate, false, nil, nil
		}
		return nil, false, nil, nil
	}
	if scores[0].exactID {
		return scores[0].tx, false, nil, nil
	}

	if caseType == model.CaseWrongTransfer && hasEstablishedRecipientPattern(scores[0].tx, history) {
		return scores[0].tx, false, nil, nil
	}
	if caseType == model.CaseWrongTransfer && ambiguousTransfer(scores, amounts) {
		return nil, true, nil, nil
	}
	if caseType == model.CaseWrongTransfer && len(scores) > 1 && scores[1].score >= scores[0].score-5 && scores[0].score >= 65 && !hasCounterpartyHint(norm, scores[0].tx) {
		return nil, true, nil, nil
	}

	return scores[0].tx, false, nil, nil
}

func scoreTransaction(caseType, norm string, amounts []float64, phones []string, tx *model.Transaction) (int, bool, bool) {
	score := 0
	exactID := false
	amountHit := false

	if tx.TransactionID != "" && strings.Contains(norm, strings.ToLower(tx.TransactionID)) {
		score += 120
		exactID = true
	}
	if amountMatches(tx.Amount.Float64(), amounts) {
		score += 45
		amountHit = true
	}
	if transactionTypeMatches(caseType, tx.Type) {
		score += 30
	}
	score += statusScore(caseType, tx.Status)
	if counterpartyMatches(norm, phones, tx.Counterparty) {
		score += 35
	}
	if caseType == model.CaseMerchantSettlementDelay && strings.Contains(strings.ToLower(tx.Counterparty), "merchant") {
		score += 10
	}
	if caseType == model.CaseAgentCashInIssue && strings.Contains(strings.ToLower(tx.Counterparty), "agent") {
		score += 10
	}
	return score, exactID, amountHit
}

func transactionTypeMatches(caseType, txType string) bool {
	switch caseType {
	case model.CaseWrongTransfer:
		return txType == model.TxTransfer
	case model.CasePaymentFailed, model.CaseDuplicatePayment:
		return txType == model.TxPayment
	case model.CaseRefundRequest:
		return txType == model.TxPayment || txType == model.TxRefund
	case model.CaseMerchantSettlementDelay:
		return txType == model.TxSettlement || txType == model.TxPayment
	case model.CaseAgentCashInIssue:
		return txType == model.TxCashIn || txType == model.TxCashOut
	default:
		return false
	}
}

func statusScore(caseType, status string) int {
	switch caseType {
	case model.CaseWrongTransfer:
		if status == model.StatusCompleted {
			return 18
		}
		if status == model.StatusPending || status == model.StatusFailed {
			return 5
		}
		return -10
	case model.CasePaymentFailed:
		if status == model.StatusFailed || status == model.StatusPending {
			return 25
		}
		return 5
	case model.CaseRefundRequest:
		if status == model.StatusCompleted {
			return 22
		}
		if status == model.StatusReversed {
			return 10
		}
	case model.CaseMerchantSettlementDelay:
		if status == model.StatusPending {
			return 25
		}
		if status == model.StatusCompleted {
			return -5
		}
	case model.CaseAgentCashInIssue:
		if status == model.StatusPending || status == model.StatusFailed {
			return 25
		}
		if status == model.StatusCompleted {
			return 5
		}
	case model.CaseDuplicatePayment:
		if status == model.StatusCompleted {
			return 20
		}
	}
	return 0
}

func counterpartyMatches(norm string, phones []string, counterparty string) bool {
	cp := strings.ToLower(counterparty)
	if cp == "" {
		return false
	}
	if strings.Contains(norm, cp) {
		return true
	}
	cpDigits := digitsOnly(cp)
	for _, phone := range phones {
		if phone == "" || cpDigits == "" {
			continue
		}
		if strings.HasSuffix(phone, cpDigits) || strings.HasSuffix(cpDigits, phone) {
			return true
		}
		if len(phone) >= 8 && len(cpDigits) >= 8 && phone[len(phone)-8:] == cpDigits[len(cpDigits)-8:] {
			return true
		}
	}
	return false
}

func hasCounterpartyHint(norm string, tx *model.Transaction) bool {
	return counterpartyMatches(norm, extractPhones(norm), tx.Counterparty)
}

func ambiguousTransfer(scores []scoredTx, amounts []float64) bool {
	if len(amounts) == 0 || len(scores) < 2 {
		return false
	}
	top := scores[0]
	if !top.amountHit {
		return false
	}
	closeCount := 0
	for _, score := range scores {
		if score.tx.Type != model.TxTransfer || !score.amountHit {
			continue
		}
		if score.score >= top.score-20 {
			closeCount++
		}
	}
	return closeCount >= 2
}

func findDuplicatePayment(history []model.Transaction, amounts []float64) (*model.Transaction, *model.Transaction) {
	groups := map[string][]*model.Transaction{}
	for i := range history {
		tx := &history[i]
		if tx.Type != model.TxPayment || tx.Status != model.StatusCompleted {
			continue
		}
		if len(amounts) > 0 && !amountMatches(tx.Amount.Float64(), amounts) {
			continue
		}
		key := tx.Counterparty + "|" + fmtAmount(tx.Amount.Float64())
		groups[key] = append(groups[key], tx)
	}

	var best []*model.Transaction
	for _, group := range groups {
		if len(group) < 2 {
			continue
		}
		sort.SliceStable(group, func(i, j int) bool { return group[i].Timestamp < group[j].Timestamp })
		firstTime, okFirst := parseTime(group[0].Timestamp)
		lastTime, okLast := parseTime(group[len(group)-1].Timestamp)
		if okFirst && okLast && lastTime.Sub(firstTime) > 2*time.Minute {
			continue
		}
		if best == nil || group[len(group)-1].Timestamp > best[len(best)-1].Timestamp {
			best = group
		}
	}
	if best == nil {
		return nil, nil
	}
	return best[0], best[len(best)-1]
}

func latestMatchingPayment(history []model.Transaction, amounts []float64) *model.Transaction {
	var best *model.Transaction
	for i := range history {
		tx := &history[i]
		if tx.Type != model.TxPayment {
			continue
		}
		if len(amounts) > 0 && !amountMatches(tx.Amount.Float64(), amounts) {
			continue
		}
		if best == nil || tx.Timestamp > best.Timestamp {
			best = tx
		}
	}
	return best
}

func singleObviousCandidate(caseType string, history []model.Transaction) *model.Transaction {
	var candidates []*model.Transaction
	for i := range history {
		tx := &history[i]
		switch caseType {
		case model.CaseWrongTransfer:
			if tx.Type == model.TxTransfer {
				candidates = append(candidates, tx)
			}
		case model.CasePaymentFailed:
			if tx.Type == model.TxPayment && (tx.Status == model.StatusFailed || tx.Status == model.StatusPending || tx.Status == model.StatusReversed) {
				candidates = append(candidates, tx)
			}
		case model.CaseRefundRequest:
			if tx.Type == model.TxRefund || tx.Type == model.TxPayment {
				candidates = append(candidates, tx)
			}
		case model.CaseAgentCashInIssue:
			if (tx.Type == model.TxCashIn || tx.Type == model.TxCashOut) && (tx.Status == model.StatusPending || tx.Status == model.StatusFailed || tx.Status == model.StatusCompleted) {
				candidates = append(candidates, tx)
			}
		}
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	return nil
}

func parseTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func hasEstablishedRecipientPattern(relevant *model.Transaction, history []model.Transaction) bool {
	if relevant == nil || relevant.Type != model.TxTransfer || relevant.Counterparty == "" {
		return false
	}
	count := 0
	for i := range history {
		tx := history[i]
		if tx.Type == model.TxTransfer && tx.Status == model.StatusCompleted && tx.Counterparty == relevant.Counterparty {
			count++
		}
	}
	return count >= 3
}

func evidenceVerdict(ctx analysis) string {
	if ctx.relevant == nil {
		return model.EvidenceInsufficientData
	}

	switch ctx.caseType {
	case model.CaseDuplicatePayment:
		if ctx.duplicateSecond != nil {
			return model.EvidenceConsistent
		}
		return model.EvidenceInconsistent
	case model.CaseWrongTransfer:
		if ctx.relevant.Type != model.TxTransfer {
			return model.EvidenceInconsistent
		}
		if ctx.established || timeSensitiveTransferContradiction(ctx) {
			return model.EvidenceInconsistent
		}
		if ctx.relevant.Status == model.StatusCompleted || ctx.relevant.Status == model.StatusPending {
			return model.EvidenceConsistent
		}
		if ctx.relevant.Status == model.StatusFailed && containsAny(ctx.norm, "failed", "fail") {
			return model.EvidenceConsistent
		}
		return model.EvidenceInconsistent
	case model.CasePaymentFailed:
		if ctx.relevant.Status == model.StatusFailed || ctx.relevant.Status == model.StatusPending || ctx.relevant.Status == model.StatusReversed {
			return model.EvidenceConsistent
		}
		return model.EvidenceInconsistent
	case model.CaseRefundRequest:
		if ctx.relevant.Status == model.StatusCompleted || ctx.relevant.Status == model.StatusReversed || ctx.relevant.Type == model.TxRefund {
			return model.EvidenceConsistent
		}
		return model.EvidenceInsufficientData
	case model.CaseMerchantSettlementDelay:
		if ctx.relevant.Status == model.StatusPending {
			return model.EvidenceConsistent
		}
		if ctx.relevant.Status == model.StatusCompleted {
			return model.EvidenceInconsistent
		}
		return model.EvidenceInsufficientData
	case model.CaseAgentCashInIssue:
		if ctx.relevant.Status == model.StatusPending || ctx.relevant.Status == model.StatusFailed {
			return model.EvidenceConsistent
		}
		return model.EvidenceInconsistent
	default:
		return model.EvidenceInsufficientData
	}
}

func timeSensitiveTransferContradiction(ctx analysis) bool {
	if ctx.relevant == nil || !hasCurrentTimeSignal(ctx.norm) {
		return false
	}
	matchedAt, ok := parseTime(ctx.relevant.Timestamp)
	if !ok {
		return false
	}
	latest, ok := latestTransactionTime(ctx.req.TransactionHistory)
	if !ok || !latest.After(matchedAt) {
		if containsAny(ctx.norm, "just now", "right now", "a moment ago", "few minutes", "few mins") && matchedAt.Before(time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC)) {
			return true
		}
		return false
	}

	age := latest.Sub(matchedAt)
	if containsAny(ctx.norm, "just now", "right now", "a moment ago", "few minutes", "few mins", "ekhon", "এখন", "এইমাত্র") {
		return age > 3*time.Hour
	}
	if containsAny(ctx.norm, "today", "this morning", "this afternoon", "this evening", "tonight", "aj", "আজ", "সকালে", "বিকেলে", "রাতে") {
		return age > 24*time.Hour
	}
	return false
}

func latestTransactionTime(history []model.Transaction) (time.Time, bool) {
	var latest time.Time
	ok := false
	for _, tx := range history {
		t, parsed := parseTime(tx.Timestamp)
		if !parsed {
			continue
		}
		if !ok || t.After(latest) {
			latest = t
			ok = true
		}
	}
	return latest, ok
}
