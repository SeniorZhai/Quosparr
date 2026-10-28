// Package remotequota reads Kimi Code, DeepSeek and GitHub Copilot quotas.
// Keys stay in the Quosparr data directory and are never written to a provider CLI.
package remotequota

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/licoy/quosparr/internal/platform"
)

// FileName is the key file inside the Quosparr data directory.
const FileName = "remote-quota.json"

// RegionChina and RegionIntl select the Kimi Code API host.
const (
	RegionChina = "cn"
	RegionIntl  = "intl"
)

// Keys are the saved credentials. Empty fields are absent.
type Keys struct {
	KimiKey      string `json:"kimiKey,omitempty"`
	KimiRegion   string `json:"kimiRegion,omitempty"`
	DeepSeekKey  string `json:"deepseekKey,omitempty"`
	CopilotToken string `json:"copilotToken,omitempty"`
}

// Path is the key file inside dir.
func Path(dir string) string { return filepath.Join(dir, FileName) }

// Load reads the key file. A missing file is an empty set of keys.
func Load(dir string) (Keys, error) {
	if dir == "" {
		return Keys{}, errors.New("remote quota directory is empty")
	}
	data, err := os.ReadFile(Path(dir))
	if errors.Is(err, fs.ErrNotExist) {
		return Keys{}, nil
	}
	if err != nil {
		return Keys{}, err
	}
	var keys Keys
	if err := json.Unmarshal(data, &keys); err != nil {
		return Keys{}, err
	}
	keys.KimiRegion = normalizeRegion(keys.KimiRegion)
	return keys, nil
}

// Save writes keys atomically with mode 0600. The directory is private.
func Save(dir string, keys Keys) error {
	if dir == "" {
		return errors.New("remote quota directory is empty")
	}
	keys.KimiKey = strings.TrimSpace(keys.KimiKey)
	keys.DeepSeekKey = strings.TrimSpace(keys.DeepSeekKey)
	keys.CopilotToken = strings.TrimSpace(keys.CopilotToken)
	keys.KimiRegion = normalizeRegion(keys.KimiRegion)
	if err := validateSecret(keys.KimiKey); err != nil {
		return fmt.Errorf("kimi key: %w", err)
	}
	if err := validateSecret(keys.DeepSeekKey); err != nil {
		return fmt.Errorf("deepseek key: %w", err)
	}
	if err := validateSecret(keys.CopilotToken); err != nil {
		return fmt.Errorf("copilot token: %w", err)
	}
	if keys.KimiRegion == RegionChina {
		keys.KimiRegion = ""
	}
	data, err := json.Marshal(keys)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return platform.WriteFileAtomic(Path(dir), data, 0o600)
}

func normalizeRegion(region string) string {
	if strings.EqualFold(strings.TrimSpace(region), RegionIntl) {
		return RegionIntl
	}
	return RegionChina
}

func validateSecret(secret string) error {
	if secret == "" {
		return nil
	}
	for _, r := range secret {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return errors.New("contains spaces or control characters")
		}
	}
	return nil
}
