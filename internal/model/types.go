package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

type FlexibleFloat float64

func (f *FlexibleFloat) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*f = 0
		return nil
	}

	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		*f = FlexibleFloat(n)
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if s == "" {
			*f = 0
			return nil
		}
		parsed, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("invalid numeric string %q", s)
		}
		*f = FlexibleFloat(parsed)
		return nil
	}

	return fmt.Errorf("invalid number")
}

func (f FlexibleFloat) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatFloat(float64(f), 'f', -1, 64)), nil
}

func (f FlexibleFloat) Float64() float64 {
	return float64(f)
}

type Request struct {
	TicketID           string                 `json:"ticket_id"`
	Complaint          string                 `json:"complaint"`
	Language           string                 `json:"language,omitempty"`
	Channel            string                 `json:"channel,omitempty"`
	UserType           string                 `json:"user_type,omitempty"`
	CampaignContext    string                 `json:"campaign_context,omitempty"`
	TransactionHistory []Transaction          `json:"transaction_history,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

type Transaction struct {
	TransactionID string        `json:"transaction_id"`
	Timestamp     string        `json:"timestamp"`
	Type          string        `json:"type"`
	Amount        FlexibleFloat `json:"amount"`
	Counterparty  string        `json:"counterparty"`
	Status        string        `json:"status"`
}

type Response struct {
	TicketID              string   `json:"ticket_id"`
	RelevantTransactionID *string  `json:"relevant_transaction_id"`
	EvidenceVerdict       string   `json:"evidence_verdict"`
	CaseType              string   `json:"case_type"`
	Severity              string   `json:"severity"`
	Department            string   `json:"department"`
	AgentSummary          string   `json:"agent_summary"`
	RecommendedNextAction string   `json:"recommended_next_action"`
	CustomerReply         string   `json:"customer_reply"`
	HumanReviewRequired   bool     `json:"human_review_required"`
	Confidence            float64  `json:"confidence"`
	ReasonCodes           []string `json:"reason_codes"`
}
