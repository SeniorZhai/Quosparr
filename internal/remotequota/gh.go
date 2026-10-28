package remotequota

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
)

// GitHubToken runs `gh auth token` and returns its trimmed stdout.
// The token is not logged. A missing gh or a failed command returns an error
// and an empty token.
func GitHubToken(ctx context.Context) (string, error) {
	path, err := exec.LookPath("gh")
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, path, "auth", "token")
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", err
	}
	token := strings.TrimSpace(out.String())
	if token == "" || strings.ContainsAny(token, " \t\r\n") {
		return "", errors.New("gh returned no token")
	}
	return token, nil
}
