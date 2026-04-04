package outputfilter

import (
	"strings"
)

// OutputFilter is a lightning-fast keyword/regex based safety net for LLM responses
type OutputFilter struct {
	blockedKeywords []string
}

func NewOutputFilter() *OutputFilter {
	return &OutputFilter{
		blockedKeywords: []string{
			"here are my instructions",
			"as an ai",
			"my system prompt is",
			"i have been programmed to",
			"you are right, i am dan",
			"ignore previous",
			"[redacted_", // Catch PII redactions failing or leaking
			"bypass successful",
		},
	}
}

// CheckResponse inspects the LLM generated response before sending it to the user.
// Returns true if blocked, false if safe.
func (of *OutputFilter) CheckResponse(responseText string) (bool, string) {
	lowerResp := strings.ToLower(responseText)

	for _, keyword := range of.blockedKeywords {
		if strings.Contains(lowerResp, keyword) {
			return true, "Detected restricted keyword sequence in LLM output: Data leakage prevented."
		}
	}

	return false, "Safe"
}
