package analyzer

import (
	"fmt"

	"github.com/sust-cse/queuestorm-investigator/internal/model"
)

func renderTemplates(ctx analysis) (string, string, string) {
	switch ctx.caseType {
	case model.CasePhishingSocialEngineering:
		return phishingTemplates(ctx)
	case model.CaseDuplicatePayment:
		return duplicateTemplates(ctx)
	case model.CaseMerchantSettlementDelay:
		return merchantSettlementTemplates(ctx)
	case model.CaseAgentCashInIssue:
		return agentCashInTemplates(ctx)
	case model.CasePaymentFailed:
		return paymentFailedTemplates(ctx)
	case model.CaseRefundRequest:
		return refundTemplates(ctx)
	case model.CaseWrongTransfer:
		return wrongTransferTemplates(ctx)
	default:
		return otherTemplates(ctx)
	}
}

func wrongTransferTemplates(ctx analysis) (string, string, string) {
	if ctx.relevant == nil {
		amount := firstAmount(ctx.amounts, "the reported")
		summary := fmt.Sprintf("Customer reports a %s BDT transfer was not received or may involve the wrong recipient, but multiple or no matching transactions prevent reliable identification.", amount)
		action := "Ask the customer for the recipient number or transaction ID before initiating any dispute workflow."
		reply := fmt.Sprintf("Thank you for reaching out. We could not safely identify the exact transaction from the provided history. Please share the recipient number or transaction ID so we can check the right transfer. Please do not share your PIN or OTP with anyone.")
		return summary, action, reply
	}

	amount := fmtAmount(ctx.relevant.Amount.Float64())
	if ctx.verdict == model.EvidenceInconsistent {
		summary := fmt.Sprintf("Customer claims %s (%s BDT to %s) was a wrong transfer, but transaction history shows repeated prior transfers to the same counterparty, suggesting an established recipient.", ctx.relevant.TransactionID, amount, ctx.relevant.Counterparty)
		action := "Flag for human review. Verify with the customer whether this was genuinely a wrong transfer given the established transaction pattern with this recipient."
		reply := fmt.Sprintf("We have received your request regarding transaction %s. Please do not share your PIN or OTP with anyone. Our dispute team will review the case carefully and contact you through official support channels.", ctx.relevant.TransactionID)
		return summary, action, reply
	}

	summary := fmt.Sprintf("Customer reports sending %s BDT via %s to %s, which they believe may be the wrong recipient.", amount, ctx.relevant.TransactionID, ctx.relevant.Counterparty)
	action := fmt.Sprintf("Verify %s details with the customer and start the wrong-transfer dispute workflow according to policy.", ctx.relevant.TransactionID)
	reply := fmt.Sprintf("We have noted your concern about transaction %s. Please do not share your PIN or OTP with anyone. Our dispute team will review the case and contact you through official support channels.", ctx.relevant.TransactionID)
	return summary, action, reply
}

func paymentFailedTemplates(ctx analysis) (string, string, string) {
	if ctx.relevant == nil {
		return "Customer reports a failed payment or deduction issue, but no matching transaction can be identified from the provided history.",
			"Ask the customer for the transaction ID, amount, and merchant or biller name before routing the case.",
			"Thank you for reaching out. Please share the transaction ID, amount, and merchant or biller name so we can check the payment. Please do not share your PIN or OTP with anyone."
	}
	amount := fmtAmount(ctx.relevant.Amount.Float64())
	summary := fmt.Sprintf("Customer attempted a %s BDT payment (%s) which is marked %s, and reports balance deduction.", amount, ctx.relevant.TransactionID, ctx.relevant.Status)
	action := fmt.Sprintf("Investigate %s ledger status. If balance was deducted on a failed or pending payment, follow the eligible reversal workflow within the standard SLA.", ctx.relevant.TransactionID)
	reply := fmt.Sprintf("We have noted that transaction %s may have caused an unexpected balance deduction. Our payments team will review the case and any eligible amount will be returned through official channels. Please do not share your PIN or OTP with anyone.", ctx.relevant.TransactionID)
	return summary, action, reply
}

func refundTemplates(ctx analysis) (string, string, string) {
	if ctx.relevant == nil {
		return "Customer requests a refund, but no matching payment can be identified from the provided transaction history.",
			"Ask for the transaction ID and merchant details before advising on refund eligibility.",
			"Thank you for reaching out. Please share the transaction ID and merchant details so we can guide you on the next step. Please do not share your PIN or OTP with anyone."
	}
	amount := fmtAmount(ctx.relevant.Amount.Float64())
	summary := fmt.Sprintf("Customer requests refund of %s BDT for %s due to a customer-side request. Transaction status is %s.", amount, ctx.relevant.TransactionID, ctx.relevant.Status)
	action := "Inform the customer that refund eligibility depends on policy and merchant confirmation. Do not promise a refund before verification."
	reply := "Thank you for reaching out. Refunds for completed merchant payments depend on the merchant's own policy. We recommend contacting the merchant directly. If you need help reaching them, please reply and we will guide you. Please do not share your PIN or OTP with anyone."
	return summary, action, reply
}

func phishingTemplates(ctx analysis) (string, string, string) {
	summary := "Customer reports a suspicious contact or message involving PIN, OTP, password, account blocking, or credential pressure. Likely social engineering attempt."
	if containsAny(ctx.norm, "haven't shared", "have not shared", "did not share", "not shared") {
		summary = "Customer reports an unsolicited contact asking for OTP or credentials and says they have not shared the information. Likely social engineering attempt."
	}
	action := "Escalate to fraud_risk immediately. Confirm that official support never asks for PIN, OTP, password, or full card details, and log any reported suspicious contact details."
	reply := "Thank you for reaching out before sharing any information. We never ask for your PIN, OTP, or password under any circumstances. Please do not share these with anyone, even if they claim to be from us. Our fraud team has been notified of this incident."
	if ctx.language == "bn" {
		reply = "তথ্য শেয়ার করার আগে আমাদের জানানোর জন্য ধন্যবাদ। আমরা কখনোই আপনার পিন, ওটিপি বা পাসওয়ার্ড চাই না। এগুলো কারো সাথে শেয়ার করবেন না, কেউ আমাদের পরিচয় দিলেও নয়। আমাদের ফ্রড টিম ঘটনাটি দেখবে।"
	}
	return summary, action, reply
}

func agentCashInTemplates(ctx analysis) (string, string, string) {
	if ctx.relevant == nil {
		return "Customer reports an agent cash-in issue, but no matching cash-in transaction can be identified.",
			"Ask for the transaction ID, agent ID, amount, and approximate time before routing to agent_operations.",
			"Thank you for reaching out. Please share the transaction ID, amount, agent ID, and approximate time so we can check the cash-in. Please do not share your PIN or OTP with anyone."
	}
	amount := fmtAmount(ctx.relevant.Amount.Float64())
	summary := fmt.Sprintf("Customer reports %s BDT cash-in via %s (%s) not reflected in balance. Transaction status is %s.", amount, ctx.relevant.Counterparty, ctx.relevant.TransactionID, ctx.relevant.Status)
	action := fmt.Sprintf("Investigate %s status with agent_operations. Confirm settlement state and resolve within the standard cash-in SLA.", ctx.relevant.TransactionID)
	reply := fmt.Sprintf("We have noted your concern about cash-in transaction %s. Our agent operations team will verify it through official channels. Please do not share your PIN or OTP with anyone.", ctx.relevant.TransactionID)
	if ctx.language == "bn" {
		reply = fmt.Sprintf("আপনার লেনদেন %s এর বিষয়ে আমরা অবগত হয়েছি। আমাদের এজেন্ট অপারেশন্স দল এটি দ্রুত যাচাই করবে এবং অফিসিয়াল চ্যানেলে আপনাকে জানাবে। অনুগ্রহ করে কারো সাথে আপনার পিন বা ওটিপি শেয়ার করবেন না।", ctx.relevant.TransactionID)
	}
	return summary, action, reply
}

func merchantSettlementTemplates(ctx analysis) (string, string, string) {
	if ctx.relevant == nil {
		return "Merchant reports a settlement delay, but no matching settlement transaction is available in the provided history.",
			"Ask for the settlement batch or transaction ID and route to merchant_operations after identification.",
			"Thank you for reaching out. Please share the settlement or transaction ID so our merchant operations team can check the batch status."
	}
	amount := fmtAmount(ctx.relevant.Amount.Float64())
	summary := fmt.Sprintf("Merchant reports %s BDT settlement (%s) is delayed. Settlement status is %s.", amount, ctx.relevant.TransactionID, ctx.relevant.Status)
	action := "Route to merchant_operations to verify settlement batch status and communicate the expected settlement time through official channels."
	reply := fmt.Sprintf("We have noted your concern about settlement %s. Our merchant operations team will check the batch status and update you on the expected settlement time through official channels.", ctx.relevant.TransactionID)
	return summary, action, reply
}

func duplicateTemplates(ctx analysis) (string, string, string) {
	if ctx.duplicateFirst != nil && ctx.duplicateSecond != nil {
		amount := fmtAmount(ctx.duplicateSecond.Amount.Float64())
		summary := fmt.Sprintf("Customer reports duplicate payment. Two identical %s BDT payments to %s were completed close together (%s and %s). The second is likely the duplicate.", amount, ctx.duplicateSecond.Counterparty, ctx.duplicateFirst.TransactionID, ctx.duplicateSecond.TransactionID)
		action := fmt.Sprintf("Verify the duplicate with payments_ops and the biller. If only one payment should stand, follow the eligible reversal workflow for %s.", ctx.duplicateSecond.TransactionID)
		reply := fmt.Sprintf("We have noted the possible duplicate payment for transaction %s. Our payments team will verify with the biller and any eligible amount will be returned through official channels. Please do not share your PIN or OTP with anyone.", ctx.duplicateSecond.TransactionID)
		return summary, action, reply
	}
	return "Customer reports a duplicate payment, but the provided history does not show a clear duplicate pair.",
		"Ask for more details and verify the biller ledger before taking any payment action.",
		"Thank you for reaching out. We need to verify the payment details before any next step. Please do not share your PIN or OTP with anyone."
}

func otherTemplates(ctx analysis) (string, string, string) {
	summary := "Customer reports a vague concern without enough transaction, amount, or issue details to identify the relevant transaction."
	action := "Reply to customer asking for specific details: transaction ID, amount, what went wrong, and approximate time."
	reply := "Thank you for reaching out. To help you faster, please share the transaction ID, the amount involved, and a short description of what went wrong. Please do not share your PIN or OTP with anyone."
	if ctx.language == "bn" {
		reply = "যোগাযোগ করার জন্য ধন্যবাদ। দ্রুত সহায়তার জন্য অনুগ্রহ করে লেনদেন আইডি, টাকার পরিমাণ এবং কী সমস্যা হয়েছে তা জানান। আপনার পিন বা ওটিপি কারো সাথে শেয়ার করবেন না।"
	}
	return summary, action, reply
}

func firstAmount(amounts []float64, fallback string) string {
	if len(amounts) == 0 {
		return fallback
	}
	return fmtAmount(amounts[0])
}
