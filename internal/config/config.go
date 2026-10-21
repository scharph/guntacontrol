// Package config loads and saves the server configuration (a YAML file that is
// also editable from the web UI, mirroring heliocontrol's layout).
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Device describes how to reach the Guntamatic Biostar Flex on the local network.
type Device struct {
	// Host is the boiler's IP or hostname.
	Host string `yaml:"host" json:"host"`
	// Key is the optional read key. Reads are open on the Flex, so this is usually
	// empty; it is only sent if a firmware variant gates reads. Never serialised to JSON.
	Key string `yaml:"key" json:"-"`
	// WriteKey is the Level-1 key required for control/write requests (/ext/parset.cgi).
	// Never serialised to JSON so the API can never leak it to the browser.
	WriteKey string `yaml:"writeKey" json:"-"`
	// TimeoutSeconds is the per-request timeout to the boiler.
	TimeoutSeconds int `yaml:"timeoutSeconds" json:"timeoutSeconds"`
}

// Loxone configures the optional UDP push to a Loxone Miniserver.
type Loxone struct {
	// UDP is the Miniserver target as host:port (e.g. 192.168.0.10:7000); empty = off.
	UDP string `yaml:"udp" json:"udp"`
}

// Overview configures the dashboard (start page).
type Overview struct {
	// Circuits selects which heating circuits (0-based) are shown on the dashboard.
	// Empty/unset shows all three.
	Circuits []int `yaml:"circuits" json:"circuits"`
}

type Config struct {
	Listen  string `yaml:"listen" json:"listen"`   // HTTP address of the UI and API
	DataDir string `yaml:"dataDir" json:"dataDir"` // SQLite history + audit database
	Device  Device `yaml:"device" json:"device"`
	// PollSeconds is the live refresh interval (minimum 5).
	PollSeconds int `yaml:"pollSeconds" json:"pollSeconds"`
	// RetentionDays is how long history samples are kept (default 35).
	RetentionDays int `yaml:"retentionDays" json:"retentionDays"`
	// AllowWrite is the master capability: when false, control is impossible and the
	// read-only toggle can never be disabled. It is not editable from the UI.
	AllowWrite bool `yaml:"allowWrite" json:"allowWrite"`
	// ReadOnly is the user-facing write protection ("Schreibschutz"); UI-editable and
	// persisted back to this file. Default: true (safe).
	ReadOnly *bool    `yaml:"readOnly" json:"readOnly"`
	Loxone   Loxone   `yaml:"loxone" json:"loxone"`
	Overview Overview `yaml:"overview" json:"overview"`
}

func (c *Config) applyDefaults() {
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if c.DataDir == "" {
		c.DataDir = "data"
	}
	if c.Device.Host == "" {
		c.Device.Host = "192.168.0.69"
	}
	if c.Device.TimeoutSeconds <= 0 {
		c.Device.TimeoutSeconds = 8
	}
	if c.PollSeconds < 5 {
		c.PollSeconds = 10
	}
	if c.RetentionDays <= 0 {
		c.RetentionDays = 35
	}
	if c.ReadOnly == nil {
		on := true
		c.ReadOnly = &on
	}
	if c.Overview.Circuits == nil {
		c.Overview.Circuits = []int{0, 1, 2, 3, 4}
	}
}

// PollInterval is PollSeconds as a duration.
func (c Config) PollInterval() time.Duration { return time.Duration(c.PollSeconds) * time.Second }

// Timeout is the per-request boiler timeout as a duration.
func (c Config) Timeout() time.Duration { return time.Duration(c.Device.TimeoutSeconds) * time.Second }

// Retention is RetentionDays as a duration.
func (c Config) Retention() time.Duration { return time.Duration(c.RetentionDays) * 24 * time.Hour }

// DBPath is the SQLite file inside the data directory.
func (c Config) DBPath() string { return filepath.Join(c.DataDir, "guntacontrol.db") }

// IsReadOnly reports whether writes to the boiler are currently disabled.
func (c Config) IsReadOnly() bool { return c.ReadOnly == nil || *c.ReadOnly }

func (c Config) Validate() error {
	if c.Device.Host == "" {
		return errors.New("device.host must be set")
	}
	return nil
}

// Store holds the current configuration and persists changes back to the YAML file.
type Store struct {
	mu   sync.RWMutex
	path string
	cfg  Config
}

// Load reads path; a missing file yields the defaults (and the file is created on the
// first Save). Environment variables GUNTAMATIC_KEY / GUNTAMATIC_WRITE_KEY fill the
// device keys when the file leaves them empty, so an existing .env keeps working.
func Load(path string) (*Store, error) {
	s := &Store{path: path}
	b, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if err := yaml.Unmarshal(b, &s.cfg); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	if s.cfg.Device.Key == "" {
		s.cfg.Device.Key = os.Getenv("GUNTAMATIC_KEY")
	}
	if s.cfg.Device.WriteKey == "" {
		s.cfg.Device.WriteKey = os.Getenv("GUNTAMATIC_WRITE_KEY")
	}
	s.cfg.applyDefaults()
	return s, s.cfg.Validate()
}

func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// Update applies fn to a copy, validates and saves it back to the YAML file.
func (s *Store) Update(fn func(*Config)) (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.cfg
	fn(&c)
	c.applyDefaults()
	if err := c.Validate(); err != nil {
		return s.cfg, err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return s.cfg, err
	}
	if dir := filepath.Dir(s.path); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return s.cfg, err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return s.cfg, err
	}
	s.cfg = c
	return c, nil
}

// --- SettingsStore for the API ------------------------------------------------

// AllowWrite reports the master write capability.
func (s *Store) AllowWrite() bool { return s.Get().AllowWrite }

// ReadOnly reports the current write-protection state.
func (s *Store) ReadOnly() bool { return s.Get().IsReadOnly() }

// SetReadOnly persists the write-protection toggle to the YAML file.
func (s *Store) SetReadOnly(on bool) error {
	_, err := s.Update(func(c *Config) { c.ReadOnly = &on })
	return err
}

// OverviewCircuits returns the heating circuits shown on the dashboard.
func (s *Store) OverviewCircuits() []int { return s.Get().Overview.Circuits }
