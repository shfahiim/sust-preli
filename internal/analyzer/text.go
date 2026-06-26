package analyzer

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var numberRe = regexp.MustCompile(`\d+(?:\.\d+)?`)
var phoneRe = regexp.MustCompile(`\+?\d{7,15}`)

func normalize(s string) string {
	s = strings.ToLower(convertBanglaDigits(s))
	s = strings.NewReplacer(
		"\u00a0", " ",
		"’", "'",
		"‘", "'",
		"“", "\"",
		"”", "\"",
	).Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func convertBanglaDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '০':
			b.WriteRune('0')
		case '১':
			b.WriteRune('1')
		case '২':
			b.WriteRune('2')
		case '৩':
			b.WriteRune('3')
		case '৪':
			b.WriteRune('4')
		case '৫':
			b.WriteRune('5')
		case '৬':
			b.WriteRune('6')
		case '৭':
			b.WriteRune('7')
		case '৮':
			b.WriteRune('8')
		case '৯':
			b.WriteRune('9')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func containsAny(s string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(s, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func extractAmounts(norm string) []float64 {
	raw := numberRe.FindAllString(norm, -1)
	seen := make(map[float64]bool)
	out := make([]float64, 0, len(raw))
	for _, token := range raw {
		amount, err := strconv.ParseFloat(token, 64)
		if err != nil {
			continue
		}
		// Long phone/account-like numbers are not useful amount hints.
		if amount <= 0 || amount > 1000000 {
			continue
		}
		if seen[amount] {
			continue
		}
		seen[amount] = true
		out = append(out, amount)
	}
	return out
}

func extractPhones(norm string) []string {
	raw := phoneRe.FindAllString(norm, -1)
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, token := range raw {
		d := digitsOnly(token)
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	return out
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func amountMatches(txAmount float64, amounts []float64) bool {
	for _, amount := range amounts {
		if abs(txAmount-amount) < 0.001 {
			return true
		}
	}
	return false
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func detectLanguage(declared, complaint string) string {
	if hasBangla(complaint) {
		if hasLatin(complaint) {
			return "mixed"
		}
		return "bn"
	}
	switch strings.ToLower(declared) {
	case "bn", "mixed":
		return strings.ToLower(declared)
	default:
		return "en"
	}
}

func hasBangla(s string) bool {
	for _, r := range s {
		if r >= '\u0980' && r <= '\u09ff' {
			return true
		}
	}
	return false
}

func hasLatin(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

func fmtAmount(v float64) string {
	if abs(v-float64(int64(v))) < 0.001 {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}
