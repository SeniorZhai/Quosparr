package l10n

import (
	"strings"
	"testing"
)

func TestTUITranslations(t *testing.T) {
	for key := range tuiStrings {
		for _, language := range Languages() {
			if value := New(language).TUI(key); strings.TrimSpace(value) == "" || value == key {
				t.Errorf("missing %s translation for %s", language, key)
			}
		}
	}
}
