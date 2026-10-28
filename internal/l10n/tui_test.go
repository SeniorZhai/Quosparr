package l10n

import (
	"strings"
	"testing"
)

func TestTUITranslations(t *testing.T) {
	for _, key := range []string{"overview", "auto", "refreshing", "next", "last", "sync", "desktop", "standalone", "live", "balance", "page", "navigate", "idle", "updated", "setup", "configure", "save", "cancel", "providers", "interval", "language"} {
		if _, ok := tuiStrings[key]; !ok {
			t.Errorf("missing TUI key %s", key)
		}
	}
	for key := range tuiStrings {
		for _, language := range Languages() {
			if value := New(language).TUI(key); strings.TrimSpace(value) == "" || value == key {
				t.Errorf("missing %s translation for %s", language, key)
			}
		}
	}
	for _, key := range []string{"loginCLI", "expiredCLI", "expiredImported"} {
		for _, language := range Languages() {
			if strings.Count(New(language).TUI(key), "%s") != 1 {
				t.Errorf("%s translation for %s must contain one command placeholder", language, key)
			}
		}
	}
}
