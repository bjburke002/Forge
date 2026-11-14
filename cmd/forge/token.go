package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
)

func githubToken(ctx context.Context) (string, error) {
	if v := os.Getenv("FORGE_GITHUB_TOKEN"); v != "" {
		token := strings.TrimSpace(v)
		return token, nil
	}
	if v := os.Getenv("GITHUB_TOKEN"); v != "" {
		token := strings.TrimSpace(v)
		return token, nil
	}
	cmd := exec.CommandContext(ctx, "gh", "auth", "token")
	out, err := cmd.Output()

	if err == nil {
		token := strings.TrimSpace(string(out))
		if token != "" {
			return token, nil
		}
	}
	return "", errors.New("no github token found: set FORGE_GITHUB, GITHUB_TOKEN or run gh auth login")
}
