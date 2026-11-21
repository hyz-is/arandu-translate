package translate

import (
	"strings"
	"unicode"
)

// tag is a BCP 47 language tag read into its three useful parts.
type tag struct {
	lang   string // "pt"
	script string // "Hans"
	region string // "BR" or "419"
}

// parseTag reads "pt-BR", "zh_Hans" or "es-419". It does not check a tag
// against the registry: whether a provider knows a language is its own answer.
func parseTag(s string) (tag, bool) {
	parts := strings.Split(strings.ReplaceAll(strings.TrimSpace(s), "_", "-"), "-")
	if len(parts) > 3 || !letters(parts[0], 2, 3) {
		return tag{}, false
	}
	t := tag{lang: strings.ToLower(parts[0])}
	for _, part := range parts[1:] {
		switch {
		case letters(part, 4, 4) && t.script == "" && t.region == "":
			t.script = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		case letters(part, 2, 2) && t.region == "":
			t.region = strings.ToUpper(part)
		case digits(part, 3) && t.region == "":
			t.region = part
		default:
			return tag{}, false
		}
	}
	return t, true
}

func letters(s string, min, max int) bool {
	if len(s) < min || len(s) > max {
		return false
	}
	for _, r := range s {
		if r > unicode.MaxASCII || !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func digits(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// traditional reports whether a Chinese tag asks for the traditional script.
func (t tag) traditional() bool {
	return t.script == "Hant" || (t.script == "" && (t.region == "TW" || t.region == "HK" || t.region == "MO"))
}

// String is the tag as it is written in BCP 47: "pt-BR", "zh-Hans".
func (t tag) String() string {
	out := t.lang
	if t.script != "" {
		out += "-" + t.script
	}
	if t.region != "" {
		out += "-" + t.region
	}
	return out
}

// dedupe keeps the first of each language, compared without case.
func dedupe(languages []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(languages))
	for _, language := range languages {
		key := strings.ToLower(language)
		if language == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, language)
	}
	return out
}
