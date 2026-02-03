package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/Applesauce-Labs/key-bringer/internal/core"
)

type Config struct {
	Version int `yaml:"version"`

	GCP struct {
		ProjectID string `yaml:"projectId"`
		Region    string `yaml:"region"`
	} `yaml:"gcp"`

	Runtime struct {
		RequireIAMAuth   bool   `yaml:"requireIamAuth"`
		MaxPendingMinutes int   `yaml:"maxPendingMinutes"`
		IAMAudience      string `yaml:"iamAudience"`
	} `yaml:"runtime"`

	HTTP struct {
		PublicURL string `yaml:"publicURL"`
	} `yaml:"http"`

	Telnyx struct {
		MessagingProfileID string `yaml:"messagingProfileId"`
	} `yaml:"telnyx"`

	Secrets struct {
		ZFSMasterKey     core.SecretRef `yaml:"zfsMasterKey"`
		TOTPSeed         core.SecretRef `yaml:"totpSeed"`
		AgentSecret      *core.SecretRef `yaml:"agentSecret"`
		TelnyxAPIKey     core.SecretRef `yaml:"telnyxApiKey"`
		TelnyxFromNumber core.SecretRef `yaml:"telnyxFromNumber"`
		TelnyxPublicKey  core.SecretRef `yaml:"telnyxPublicKey"`
		AdminPhone       core.SecretRef `yaml:"adminPhone"`
	} `yaml:"secrets"`

	Machines []string `yaml:"machines"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	dec := newDecoder(b)

	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("config: unsupported version %d", c.Version)
	}
	if c.GCP.ProjectID == "" {
		return errors.New("config: gcp.projectId is required")
	}
	if c.Runtime.MaxPendingMinutes <= 0 {
		c.Runtime.MaxPendingMinutes = 10
	}
	if c.Runtime.RequireIAMAuth && c.Runtime.IAMAudience == "" {
		return errors.New("config: runtime.iamAudience is required when requireIamAuth is true")
	}
	if c.HTTP.PublicURL == "" {
		return errors.New("config: http.publicURL is required")
	}
	if c.Telnyx.MessagingProfileID == "" {
		return errors.New("config: telnyx.messagingProfileId is required")
	}

	refs := []struct {
		name string
		ref  core.SecretRef
	}{
		{"secrets.zfsMasterKey", c.Secrets.ZFSMasterKey},
		{"secrets.totpSeed", c.Secrets.TOTPSeed},
		{"secrets.telnyxApiKey", c.Secrets.TelnyxAPIKey},
		{"secrets.telnyxFromNumber", c.Secrets.TelnyxFromNumber},
		{"secrets.telnyxPublicKey", c.Secrets.TelnyxPublicKey},
		{"secrets.adminPhone", c.Secrets.AdminPhone},
	}
	for _, item := range refs {
		if err := item.ref.Validate(); err != nil {
			return fmt.Errorf("config: %s: %w", item.name, err)
		}
	}
	if c.Secrets.AgentSecret != nil {
		if err := c.Secrets.AgentSecret.Validate(); err != nil {
			return fmt.Errorf("config: secrets.agentSecret: %w", err)
		}
	}
	if len(c.Machines) == 0 {
		return errors.New("config: machines allow-list must not be empty")
	}
	return nil
}
