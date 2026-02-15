package domain_test

import (
	"byto/internal/domain"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// helper: creates a temp config dir, sets env so getSettingsFilePath uses it,
// and returns a cleanup function.
func setupTempConfigDir(t *testing.T) (configDir string, cleanup func()) {
	t.Helper()
	tmpDir := t.TempDir()

	// On Windows, UserConfigDir() uses %AppData%.
	// On Unix, it uses $XDG_CONFIG_HOME or ~/.config.
	origAppData := os.Getenv("AppData")
	origXDG := os.Getenv("XDG_CONFIG_HOME")

	os.Setenv("AppData", tmpDir)
	os.Setenv("XDG_CONFIG_HOME", tmpDir)

	cleanup = func() {
		os.Setenv("AppData", origAppData)
		os.Setenv("XDG_CONFIG_HOME", origXDG)
	}
	return tmpDir, cleanup
}

// writeSettingsFile writes a JSON settings file into the byto config directory.
func writeSettingsFile(t *testing.T, configDir string, content []byte) string {
	t.Helper()
	bytoDir := filepath.Join(configDir, "byto")
	if err := os.MkdirAll(bytoDir, 0755); err != nil {
		t.Fatalf("failed to create byto config dir: %v", err)
	}
	filePath := filepath.Join(bytoDir, "settings.json")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("failed to write settings file: %v", err)
	}
	return filePath
}

// --- NewSetting tests ---

func TestNewSetting_DefaultValues_WhenNoFile(t *testing.T) {
	_, cleanup := setupTempConfigDir(t)
	defer cleanup()

	s := domain.NewSetting()
	if s == nil {
		t.Fatal("expected non-nil Setting")
	}
	if s.ParallelDownloads != 1 {
		t.Errorf("expected ParallelDownloads=1, got %d", s.ParallelDownloads)
	}
}

func TestNewSetting_LoadsFromFile(t *testing.T) {
	configDir, cleanup := setupTempConfigDir(t)
	defer cleanup()

	data, _ := json.Marshal(map[string]int{"parallel_downloads": 5})
	writeSettingsFile(t, configDir, data)

	s := domain.NewSetting()
	if s == nil {
		t.Fatal("expected non-nil Setting")
	}
	if s.ParallelDownloads != 5 {
		t.Errorf("expected ParallelDownloads=5, got %d", s.ParallelDownloads)
	}
}

func TestNewSetting_InvalidJSON_ReturnsDefault(t *testing.T) {
	configDir, cleanup := setupTempConfigDir(t)
	defer cleanup()

	writeSettingsFile(t, configDir, []byte("not valid json{{{"))

	s := domain.NewSetting()
	if s == nil {
		t.Fatal("expected non-nil Setting from defaults")
	}
	if s.ParallelDownloads != 1 {
		t.Errorf("expected default ParallelDownloads=1, got %d", s.ParallelDownloads)
	}
}

func TestNewSetting_EmptyJSON_ReturnsZeroValues(t *testing.T) {
	configDir, cleanup := setupTempConfigDir(t)
	defer cleanup()

	writeSettingsFile(t, configDir, []byte("{}"))

	s := domain.NewSetting()
	if s == nil {
		t.Fatal("expected non-nil Setting")
	}
	// Empty JSON unmarshals to zero values for int fields
	if s.ParallelDownloads != 0 {
		t.Errorf("expected ParallelDownloads=0 from empty JSON, got %d", s.ParallelDownloads)
	}
}

func TestNewSetting_ExtraFields_IgnoredGracefully(t *testing.T) {
	configDir, cleanup := setupTempConfigDir(t)
	defer cleanup()

	data := []byte(`{"parallel_downloads": 3, "unknown_field": "value", "another": 42}`)
	writeSettingsFile(t, configDir, data)

	s := domain.NewSetting()
	if s == nil {
		t.Fatal("expected non-nil Setting")
	}
	if s.ParallelDownloads != 3 {
		t.Errorf("expected ParallelDownloads=3, got %d", s.ParallelDownloads)
	}
}

func TestNewSetting_NegativeParallelDownloads(t *testing.T) {
	configDir, cleanup := setupTempConfigDir(t)
	defer cleanup()

	data, _ := json.Marshal(map[string]int{"parallel_downloads": -1})
	writeSettingsFile(t, configDir, data)

	s := domain.NewSetting()
	if s == nil {
		t.Fatal("expected non-nil Setting")
	}
	if s.ParallelDownloads != -1 {
		t.Errorf("expected ParallelDownloads=-1 (loaded as-is), got %d", s.ParallelDownloads)
	}
}

func TestNewSetting_ZeroParallelDownloads(t *testing.T) {
	configDir, cleanup := setupTempConfigDir(t)
	defer cleanup()

	data, _ := json.Marshal(map[string]int{"parallel_downloads": 0})
	writeSettingsFile(t, configDir, data)

	s := domain.NewSetting()
	if s == nil {
		t.Fatal("expected non-nil Setting")
	}
	if s.ParallelDownloads != 0 {
		t.Errorf("expected ParallelDownloads=0, got %d", s.ParallelDownloads)
	}
}

func TestNewSetting_LargeParallelDownloads(t *testing.T) {
	configDir, cleanup := setupTempConfigDir(t)
	defer cleanup()

	data, _ := json.Marshal(map[string]int{"parallel_downloads": 1000})
	writeSettingsFile(t, configDir, data)

	s := domain.NewSetting()
	if s == nil {
		t.Fatal("expected non-nil Setting")
	}
	if s.ParallelDownloads != 1000 {
		t.Errorf("expected ParallelDownloads=1000, got %d", s.ParallelDownloads)
	}
}

// --- Save tests ---

func TestSave_CreatesFile(t *testing.T) {
	configDir, cleanup := setupTempConfigDir(t)
	defer cleanup()

	s := &domain.Setting{ParallelDownloads: 4}
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	filePath := filepath.Join(configDir, "byto", "settings.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("expected settings file to exist after Save()")
	}
}

func TestSave_WritesCorrectJSON(t *testing.T) {
	configDir, cleanup := setupTempConfigDir(t)
	defer cleanup()

	s := &domain.Setting{ParallelDownloads: 7}
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	filePath := filepath.Join(configDir, "byto", "settings.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}

	var loaded domain.Setting
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to unmarshal saved JSON: %v", err)
	}
	if loaded.ParallelDownloads != 7 {
		t.Errorf("expected saved ParallelDownloads=7, got %d", loaded.ParallelDownloads)
	}
}

func TestSave_OverwritesExistingFile(t *testing.T) {
	configDir, cleanup := setupTempConfigDir(t)
	defer cleanup()

	// Write initial settings
	s := &domain.Setting{ParallelDownloads: 2}
	if err := s.Save(); err != nil {
		t.Fatalf("first Save() error: %v", err)
	}

	// Overwrite with new settings
	s.ParallelDownloads = 10
	if err := s.Save(); err != nil {
		t.Fatalf("second Save() error: %v", err)
	}

	filePath := filepath.Join(configDir, "byto", "settings.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}

	var loaded domain.Setting
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to unmarshal saved JSON: %v", err)
	}
	if loaded.ParallelDownloads != 10 {
		t.Errorf("expected saved ParallelDownloads=10, got %d", loaded.ParallelDownloads)
	}
}

func TestSave_ProducesIndentedJSON(t *testing.T) {
	configDir, cleanup := setupTempConfigDir(t)
	defer cleanup()

	s := &domain.Setting{ParallelDownloads: 3}
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	filePath := filepath.Join(configDir, "byto", "settings.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}

	// MarshalIndent should produce formatted JSON
	expected, _ := json.MarshalIndent(s, "", "  ")
	if string(data) != string(expected) {
		t.Errorf("expected indented JSON:\n%s\ngot:\n%s", expected, data)
	}
}

func TestSave_ZeroValue(t *testing.T) {
	configDir, cleanup := setupTempConfigDir(t)
	defer cleanup()

	s := &domain.Setting{ParallelDownloads: 0}
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	filePath := filepath.Join(configDir, "byto", "settings.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}

	var loaded domain.Setting
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if loaded.ParallelDownloads != 0 {
		t.Errorf("expected ParallelDownloads=0, got %d", loaded.ParallelDownloads)
	}
}

// --- Update tests ---

func TestUpdate_ChangesParallelDownloads(t *testing.T) {
	s := &domain.Setting{ParallelDownloads: 1}
	s.Update(5)
	if s.ParallelDownloads != 5 {
		t.Errorf("expected ParallelDownloads=5 after Update, got %d", s.ParallelDownloads)
	}
}

func TestUpdate_ToZero(t *testing.T) {
	s := &domain.Setting{ParallelDownloads: 3}
	s.Update(0)
	if s.ParallelDownloads != 0 {
		t.Errorf("expected ParallelDownloads=0 after Update, got %d", s.ParallelDownloads)
	}
}

func TestUpdate_ToNegative(t *testing.T) {
	s := &domain.Setting{ParallelDownloads: 1}
	s.Update(-5)
	if s.ParallelDownloads != -5 {
		t.Errorf("expected ParallelDownloads=-5 after Update, got %d", s.ParallelDownloads)
	}
}

func TestUpdate_MultipleTimes(t *testing.T) {
	s := &domain.Setting{ParallelDownloads: 1}
	s.Update(3)
	s.Update(7)
	s.Update(2)
	if s.ParallelDownloads != 2 {
		t.Errorf("expected ParallelDownloads=2 after multiple updates, got %d", s.ParallelDownloads)
	}
}

func TestUpdate_SameValue(t *testing.T) {
	s := &domain.Setting{ParallelDownloads: 5}
	s.Update(5)
	if s.ParallelDownloads != 5 {
		t.Errorf("expected ParallelDownloads=5 (unchanged), got %d", s.ParallelDownloads)
	}
}

// --- Round-trip tests (Save then Load) ---

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	_, cleanup := setupTempConfigDir(t)
	defer cleanup()

	original := &domain.Setting{ParallelDownloads: 8}
	if err := original.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded := domain.NewSetting()
	if loaded == nil {
		t.Fatal("expected non-nil Setting after loading")
	}
	if loaded.ParallelDownloads != original.ParallelDownloads {
		t.Errorf("round-trip mismatch: saved %d, loaded %d",
			original.ParallelDownloads, loaded.ParallelDownloads)
	}
}

func TestUpdateSaveAndLoad_RoundTrip(t *testing.T) {
	_, cleanup := setupTempConfigDir(t)
	defer cleanup()

	s := domain.NewSetting() // defaults
	s.Update(12)
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded := domain.NewSetting()
	if loaded == nil {
		t.Fatal("expected non-nil Setting after loading")
	}
	if loaded.ParallelDownloads != 12 {
		t.Errorf("expected ParallelDownloads=12, got %d", loaded.ParallelDownloads)
	}
}

func TestMultipleSaveAndLoad_LastWins(t *testing.T) {
	_, cleanup := setupTempConfigDir(t)
	defer cleanup()

	for _, v := range []int{1, 5, 10, 3} {
		s := &domain.Setting{ParallelDownloads: v}
		if err := s.Save(); err != nil {
			t.Fatalf("Save(%d) error: %v", v, err)
		}
	}

	loaded := domain.NewSetting()
	if loaded == nil {
		t.Fatal("expected non-nil Setting")
	}
	if loaded.ParallelDownloads != 3 {
		t.Errorf("expected last saved value 3, got %d", loaded.ParallelDownloads)
	}
}

// --- JSON serialization tests ---

func TestSetting_JSONTags(t *testing.T) {
	s := domain.Setting{ParallelDownloads: 4}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if _, ok := raw["parallel_downloads"]; !ok {
		t.Error("expected JSON key 'parallel_downloads' to exist")
	}
	if v, ok := raw["parallel_downloads"].(float64); !ok || int(v) != 4 {
		t.Errorf("expected parallel_downloads=4, got %v", raw["parallel_downloads"])
	}
}

func TestSetting_JSONUnmarshal(t *testing.T) {
	jsonStr := `{"parallel_downloads": 6}`
	var s domain.Setting
	if err := json.Unmarshal([]byte(jsonStr), &s); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if s.ParallelDownloads != 6 {
		t.Errorf("expected ParallelDownloads=6, got %d", s.ParallelDownloads)
	}
}

func TestSetting_JSONUnmarshal_WrongFieldName(t *testing.T) {
	// Using Go field name instead of JSON tag should not populate the field
	jsonStr := `{"ParallelDownloads": 9}`
	var s domain.Setting
	if err := json.Unmarshal([]byte(jsonStr), &s); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	// JSON is case-insensitive for struct field matching, so this will match
	// This test documents the behavior
	_ = s.ParallelDownloads
}

// --- Edge case: corrupted file ---

func TestNewSetting_PartialJSON(t *testing.T) {
	configDir, cleanup := setupTempConfigDir(t)
	defer cleanup()

	// Truncated JSON
	writeSettingsFile(t, configDir, []byte(`{"parallel_downloads": 5`))

	s := domain.NewSetting()
	if s == nil {
		t.Fatal("expected non-nil Setting (should fall back to defaults)")
	}
	// Should get default because JSON is invalid
	if s.ParallelDownloads != 1 {
		t.Errorf("expected default ParallelDownloads=1, got %d", s.ParallelDownloads)
	}
}

func TestNewSetting_EmptyFile(t *testing.T) {
	configDir, cleanup := setupTempConfigDir(t)
	defer cleanup()

	writeSettingsFile(t, configDir, []byte(""))

	s := domain.NewSetting()
	if s == nil {
		t.Fatal("expected non-nil Setting (should fall back to defaults)")
	}
	if s.ParallelDownloads != 1 {
		t.Errorf("expected default ParallelDownloads=1, got %d", s.ParallelDownloads)
	}
}

func TestNewSetting_NullJSON(t *testing.T) {
	configDir, cleanup := setupTempConfigDir(t)
	defer cleanup()

	writeSettingsFile(t, configDir, []byte("null"))

	s := domain.NewSetting()
	if s == nil {
		t.Fatal("expected non-nil Setting (should fall back to defaults)")
	}
}

func TestNewSetting_ArrayJSON(t *testing.T) {
	configDir, cleanup := setupTempConfigDir(t)
	defer cleanup()

	writeSettingsFile(t, configDir, []byte("[1, 2, 3]"))

	s := domain.NewSetting()
	if s == nil {
		t.Fatal("expected non-nil Setting (should fall back to defaults)")
	}
	if s.ParallelDownloads != 1 {
		t.Errorf("expected default ParallelDownloads=1, got %d", s.ParallelDownloads)
	}
}

func TestNewSetting_WrongTypeJSON(t *testing.T) {
	configDir, cleanup := setupTempConfigDir(t)
	defer cleanup()

	// parallel_downloads as string instead of int
	writeSettingsFile(t, configDir, []byte(`{"parallel_downloads": "five"}`))

	s := domain.NewSetting()
	if s == nil {
		t.Fatal("expected non-nil Setting (should fall back to defaults)")
	}
	if s.ParallelDownloads != 1 {
		t.Errorf("expected default ParallelDownloads=1, got %d", s.ParallelDownloads)
	}
}
