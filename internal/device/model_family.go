package device

import (
	"regexp"
	"strings"
)

// Families describe recognized model names, never the service that handled a
// request. In particular neither the agent nor Keeper pricing identifies it.
var familyPatterns = []struct {
	family  string
	pattern *regexp.Regexp
}{
	{"openai", regexp.MustCompile(`^(gpt-(3\.5|4o?|4\.[0-9]+|5(\.[0-9]+)?)(-(turbo|mini|nano|pro|codex|chat|latest|preview|instruct|audio|realtime|search|transcribe|tts|max|spark|[0-9]{4}-[0-9]{2}-[0-9]{2}|[0-9]{4}))*|o[134](-(mini|pro|preview|deep-research|[0-9]{4}-[0-9]{2}-[0-9]{2}))*|codex|codex-mini(-latest)?|codex-[0-9]{4}-[0-9]{2}-[0-9]{2})$`)},
	{"anthropic", regexp.MustCompile(`^claude-((opus|sonnet|haiku)-[34]([.-][0-9]+)*|[34]([.-][0-9]+)*-(opus|sonnet|haiku)|(instant-)?[12](\.[0-9]+)?)(-(latest|thinking|[0-9]{8}))?$`)},
	{"minimax", regexp.MustCompile(`^minimax-m[12](\.[0-9]+)?(-(highspeed|lightning))?$`)},
}

// Explicit deployment compatibility names, confirmed by the operator. These
// classify a family label only; they do not assert public model existence or
// identify the service handling requests. Do not generalize their suffixes.
var compatibilityFamilies = map[string]string{
	"gpt-5.6-luna":  "openai",
	"gpt-5.6-sol":   "openai",
	"gpt-5.6-terra": "openai",
	"gpt-6-astra":   "openai",
	"gpt-6-luna":    "openai",
	"gpt-6-sol":     "openai",
	"gpt-6.1-sol":   "openai",
}

func validModelFamily(family string) bool {
	return family == "" || family == "openai" || family == "anthropic" || family == "minimax" || family == "unknown"
}

func modelFamily(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	// Only known client labels are stripped. Arbitrary store labels, paths,
	// aliases and unrecognized provider namespaces remain unknown.
	for _, client := range []string{"pi", "hermes", "codex", "claude", "opencode", "copilot"} {
		if rest, ok := strings.CutPrefix(name, "["+client+"] "); ok {
			name = rest
			break
		}
	}
	provider := ""
	if prefix, rest, ok := strings.Cut(name, "/"); ok {
		switch prefix {
		case "openai", "openai-codex":
			provider = "openai"
		case "anthropic", "minimax":
			provider = prefix
		default:
			return "unknown"
		}
		name = rest
	}
	if family, ok := compatibilityFamilies[name]; ok && (provider == "" || provider == family) {
		return family
	}
	for _, rule := range familyPatterns {
		if rule.pattern.MatchString(name) && (provider == "" || provider == rule.family) {
			return rule.family
		}
	}
	return "unknown"
}

// selectModelFamily preserves the validated, disjoint model categories. The
// snapshot remainder includes categories without a model AND extra total tokens
// without a category. Neither can safely be assigned even to a sole named model.
// A zero-token model row still counts as an uploaded matching day.
func selectModelFamily(snapshot Snapshot, family string) ([]Model, Aggregate, bool) {
	selected := make([]Model, 0, len(snapshot.Models))
	remainder := Aggregate{Tokens: snapshot.Totals, Total: snapshot.TotalTokens}
	for _, m := range snapshot.Models {
		remainder.Tokens.Input -= m.Input
		remainder.Tokens.Output -= m.Output
		remainder.Tokens.CacheRead -= m.CacheRead
		remainder.Tokens.CacheWrite -= m.CacheWrite
		remainder.Total -= m.Input + m.Output + m.CacheRead + m.CacheWrite
		if family == "" || modelFamily(m.Name) == family {
			selected = append(selected, m)
		}
	}
	remainder.Coverage.Unattributed = remainder.Total
	if family != "" && family != "unknown" {
		remainder = Aggregate{}
	}
	matches := family == "" || len(selected) > 0 || remainder.Total > 0 || family == "unknown" && len(snapshot.Models) == 0
	return selected, remainder, matches
}
