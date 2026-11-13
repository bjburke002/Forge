package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

// Config defines the configuration YAML that is passed in from a config file
type Config struct {
	GitOpsRepo    Repo   `json:"gitopsRepo"`
	GitOpsBranch  string `json:"gitopsBranch"`
	Domain        string `json:"domain"`
	ArgoNamespace string `json:"argoNamespace"`
	KubeContext   string `json:"kubeContext"`
	AMPEndpoint   string `json:"ampEndpoint"`
	AWSRegion     string `json:"awsRegion"`
	GrafanaURL    string `json:"grafanaURL"`
}

// Repo details the two different parts of a GitHub link: owner/name e.g. bjburke002/Forge
type Repo struct {
	Owner string
	Name  string
}

// UnmarshalJSON method splits the repo string on owner/name
func (r *Repo) UnmarshalJSON(data []byte) error {
	var s string

	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("gitopsRepo must be a string like owner/name: %w", err)
	}

	parsed, err := ParseRepo(s)
	if err != nil {
		return err
	}

	*r = parsed
	return nil
}

// MarshalJSON puts the repo string back together on owner/name
func (r Repo) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.String())
}

// Stringer method to rejoin repo parts for messages/URL generation
func (r Repo) String() string {
	if r.Owner == "" {
		return ""
	}
	return r.Owner + "/" + r.Name
}

// ParseRepo parses an "owner/name" string into a Repo.
func ParseRepo(s string) (Repo, error) {
	owner, name, ok := strings.Cut(s, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return Repo{}, fmt.Errorf("invalid repository %q: want owner/name", s)
	}
	return Repo{Owner: owner, Name: name}, nil
}

// Set default configs
func defaults() Config {
	return Config{
		GitOpsBranch:  "main",
		ArgoNamespace: "argocd",
		AWSRegion:     "us-east-1",
	}
}

// DefaultPath sets the fallback path for the CLI's config.yaml if it isn't explicitly set by a user
func DefaultPath() (string, error) {
	// Mac fallback path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding user home directory: %w", err)
	}

	xdgFile := filepath.Join(homeDir, ".config", "forge", "config.yaml")
	if _, err := os.Stat(xdgFile); err == nil {
		return xdgFile, nil
	}
	// Config dir for other OS
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("finding config directory: %w", err)
	}

	configFile := filepath.Join(configDir, "forge", "config.yaml")
	return configFile, nil
}

// applyEnv overrides config values with any FORGE_ user Envs
// Possible env vars: FORGE_GITOPS_BRANCH, FORGE_GITOPS_REPO, FORGE_DOMAIN, FORGE_ARGO_NAMESPACE, FORGE_KUBE_CONTEXT,
// FORGE_AMP_ENDPOINT, FORGE_AWS_REGION, FORGE_GRAFANA_URL
func applyEnv(c *Config) error {
	if r := os.Getenv("FORGE_GITOPS_REPO"); r != "" {
		repo, err := ParseRepo(r)
		if err != nil {
			return fmt.Errorf("parsing FORGE_GITOPS_REPO: %w", err)
		}
		c.GitOpsRepo = repo
	}

	if v := os.Getenv("FORGE_GITOPS_BRANCH"); v != "" {
		c.GitOpsBranch = v
	}

	if v := os.Getenv("FORGE_DOMAIN"); v != "" {
		c.Domain = v
	}

	if v := os.Getenv("FORGE_ARGO_NAMESPACE"); v != "" {
		c.ArgoNamespace = v
	}

	if v := os.Getenv("FORGE_KUBE_CONTEXT"); v != "" {
		c.KubeContext = v
	}

	if v := os.Getenv("FORGE_AMP_ENDPOINT"); v != "" {
		c.AMPEndpoint = v
	}

	if v := os.Getenv("FORGE_AWS_REGION"); v != "" {
		c.AWSRegion = v
	}

	if v := os.Getenv("FORGE_GRAFANA_URL"); v != "" {
		c.GrafanaURL = v
	}
	return nil
}

// normalize trims trailing slashes from the AMP and Grafana URLs
func normalize(c *Config) {
	c.AMPEndpoint = strings.TrimRight(strings.TrimSpace(c.AMPEndpoint), "/")
	c.GrafanaURL = strings.TrimRight(strings.TrimSpace(c.GrafanaURL), "/")
}

// Load reads the config file at path or the default location if no path given, applies FORGE env overrides and
// normalizes the result
func Load(path string) (Config, error) {
	c := defaults()

	explicit := path != "" // Checking for presence of filepath

	if !explicit {
		p, err := DefaultPath()
		if err != nil {
			return Config{}, err
		}
		path = p
	}

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := yaml.UnmarshalStrict(data, &c); err != nil {
			return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
		}
	case errors.Is(err, fs.ErrNotExist) && !explicit:
		// No config file at default location: keep defaults
	default:
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}

	if err := applyEnv(&c); err != nil {
		return Config{}, err
	}

	normalize(&c)

	return c, nil
}
