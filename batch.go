package translate

import (
	"fmt"
	"strings"
)

// translateAll sends the non-empty texts through call in batches of at most
// maxItems texts and maxBytes bytes, and puts the answers back where the texts
// were. An empty text is answered with itself and never sent. It returns the
// source language the first batch reported.
func translateAll(texts []string, maxItems, maxBytes int, call func(batch []string) ([]string, string, error)) ([]string, string, error) {
	out := make([]string, len(texts))
	var pending []int
	for i, text := range texts {
		if strings.TrimSpace(text) == "" {
			out[i] = text
			continue
		}
		pending = append(pending, i)
	}

	detected := ""
	for len(pending) > 0 {
		take, size := 0, 0
		for take < len(pending) && take < maxItems {
			size += len(texts[pending[take]])
			if take > 0 && size > maxBytes {
				break
			}
			take++
		}
		group := pending[:take]
		pending = pending[take:]

		batch := make([]string, len(group))
		for i, index := range group {
			batch[i] = texts[index]
		}
		translated, source, err := call(batch)
		if err != nil {
			return nil, "", err
		}
		if len(translated) != len(batch) {
			return nil, "", fmt.Errorf("translate: the provider answered %d texts for %d", len(translated), len(batch))
		}
		for i, index := range group {
			out[index] = translated[i]
		}
		if detected == "" {
			detected = source
		}
	}
	return out, detected, nil
}
