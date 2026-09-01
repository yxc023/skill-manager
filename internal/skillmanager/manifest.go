package skillmanager

import (
	"encoding/json"
	"fmt"
	"os"
)

// ManifestVersion is the schema version for skills-manage.json.
const ManifestVersion = 2

// Manifest is the top-level structure of skills-manage.json.
type Manifest struct {
	Version int                  `json:"version"`
	Targets []Target             `json:"targets"`
	Skills  map[string]*SkillDef `json:"skills"`
}

// SkillDef describes a single skill.
type SkillDef struct {
	Category    string   `json:"category,omitempty"`
	Description string   `json:"description,omitempty"`
	Enabled     *bool    `json:"enabled,omitempty"` // pointer so we can detect absence (default true)
	Source      Source   `json:"source"`
	Targets     []Target `json:"targets,omitempty"`
}

// IsEnabled returns whether this skill should be installed. Defaults to true
// when the field is absent from JSON.
func (sd *SkillDef) IsEnabled() bool {
	if sd.Enabled == nil {
		return true
	}
	return *sd.Enabled
}

// EffectiveTargets returns sd.Targets if non-empty, otherwise fallback.
func (sd *SkillDef) EffectiveTargets(fallback []Target) []Target {
	if len(sd.Targets) > 0 {
		return sd.Targets
	}
	return fallback
}

// LoadManifest reads skills-manage.json from path and validates schema version.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	if m.Version != ManifestVersion {
		return nil, fmt.Errorf("manifest version %d != supported %d", m.Version, ManifestVersion)
	}
	return &m, nil
}

// SaveManifest writes m as JSON to path with 2-space indent and trailing newline.
func SaveManifest(m *Manifest, path string) error {
	if m.Version == 0 {
		m.Version = ManifestVersion
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// LockEntry is one entry of skills-manage.lock.json.
type LockEntry struct {
	Source          Source `json:"source"`
	Category        string `json:"category"`
	SkillFolderHash string `json:"skillFolderHash"`
}

// Lock is the top-level structure of skills-manage.lock.json.
type Lock struct {
	Version int                   `json:"version"`
	Skills  map[string]*LockEntry `json:"skills"`
}

// LoadLock reads skills-manage.lock.json from path. If the file does not exist,
// returns an empty Lock with Version=ManifestVersion.
func LoadLock(path string) (*Lock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Lock{Version: ManifestVersion, Skills: map[string]*LockEntry{}}, nil
		}
		return nil, fmt.Errorf("read lock %s: %w", path, err)
	}
	var l Lock
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("parse lock %s: %w", path, err)
	}
	if l.Skills == nil {
		l.Skills = map[string]*LockEntry{}
	}
	return &l, nil
}

// SaveLock writes l as JSON to path.
func SaveLock(l *Lock, path string) error {
	if l.Version == 0 {
		l.Version = ManifestVersion
	}
	if l.Skills == nil {
		l.Skills = map[string]*LockEntry{}
	}
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal lock: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}