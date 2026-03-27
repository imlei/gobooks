package numbering

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const storeFileName = "display_numbering.json"

// persistedFile is the on-disk JSON shape (versioned for future migrations).
type persistedFile struct {
	Version int           `json:"version"`
	Rules   []DisplayRule `json:"rules"`
}

// DefaultStorePath returns the path under the process working directory.
func DefaultStorePath() string {
	return filepath.Join("data", storeFileName)
}

// LoadMerged reads rules from disk when present and merges them onto defaults by module_key.
func LoadMerged(path string) ([]DisplayRule, error) {
	defaults := DefaultDisplayRules()
	byKey := map[string]DisplayRule{}
	for _, r := range defaults {
		byKey[r.ModuleKey] = r
	}

	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaults, nil
		}
		return nil, err
	}

	var pf persistedFile
	if err := json.Unmarshal(b, &pf); err != nil {
		return nil, err
	}
	for _, r := range pf.Rules {
		r = NormalizeRule(r)
		if _, ok := byKey[r.ModuleKey]; !ok {
			continue
		}
		base := byKey[r.ModuleKey]
		if r.ModuleName == "" {
			r.ModuleName = base.ModuleName
		}
		byKey[r.ModuleKey] = r
	}

	out := make([]DisplayRule, 0, len(defaults))
	for _, d := range defaults {
		out = append(out, byKey[d.ModuleKey])
	}
	return out, nil
}

// Save writes all rules for known modules to disk.
func Save(path string, rules []DisplayRule) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	pf := persistedFile{Version: 1, Rules: rules}
	b, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
