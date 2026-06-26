package model

const (
	EvidenceConsistent       = "consistent"
	EvidenceInconsistent     = "inconsistent"
	EvidenceInsufficientData = "insufficient_data"
)

const (
	CaseWrongTransfer             = "wrong_transfer"
	CasePaymentFailed             = "payment_failed"
	CaseRefundRequest             = "refund_request"
	CaseDuplicatePayment          = "duplicate_payment"
	CaseMerchantSettlementDelay   = "merchant_settlement_delay"
	CaseAgentCashInIssue          = "agent_cash_in_issue"
	CasePhishingSocialEngineering = "phishing_or_social_engineering"
	CaseOther                     = "other"
)

const (
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

const (
	DepartmentCustomerSupport    = "customer_support"
	DepartmentDisputeResolution  = "dispute_resolution"
	DepartmentPaymentsOps        = "payments_ops"
	DepartmentMerchantOperations = "merchant_operations"
	DepartmentAgentOperations    = "agent_operations"
	DepartmentFraudRisk          = "fraud_risk"
)

const (
	TxTransfer   = "transfer"
	TxPayment    = "payment"
	TxCashIn     = "cash_in"
	TxCashOut    = "cash_out"
	TxSettlement = "settlement"
	TxRefund     = "refund"
)

const (
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusPending   = "pending"
	StatusReversed  = "reversed"
)
