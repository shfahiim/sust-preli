package analyzer

import "strings"

var unsafeReplacements = []struct {
	old string
	new string
}{
	{"we will refund you", "any eligible amount will be returned through official channels"},
	{"refund is confirmed", "refund eligibility will be reviewed through official channels"},
	{"your money has been recovered", "the case will be reviewed through official channels"},
	{"we reversed", "we will review"},
	{"account unblocked", "account status will be reviewed"},
	{"send your pin", "do not share your PIN"},
	{"share your pin", "do not share your PIN"},
	{"send otp", "do not share your OTP"},
	{"share otp", "do not share your OTP"},
	{"share your otp", "do not share your OTP"},
	{"send your otp", "do not share your OTP"},
}

func sanitizeText(s string) string {
	out := s
	lower := strings.ToLower(out)
	for _, repl := range unsafeReplacements {
		if strings.Contains(lower, repl.old) {
			out = strings.ReplaceAll(out, repl.old, repl.new)
			out = strings.ReplaceAll(out, strings.Title(repl.old), repl.new)
			lower = strings.ToLower(out)
		}
	}
	return out
}
