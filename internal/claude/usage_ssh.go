package claude

import (
	"context"
	_ "embed"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"
)

//go:embed usage_ssh.py
var sshUsageScript string

var ErrUsageSSH = errors.New("claude: remote usage query failed")

// FetchUsageSSH reads subscription usage through an existing SSH host alias.
// The remote host needs python3 and Claude's file credentials; tokens stay there.
func FetchUsageSSH(ctx context.Context, host string) (UsageSnapshot, error) {
	if !ValidSSHHost(host) {
		return UsageSnapshot{}, ErrUsageSSH
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ssh", "-T", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes", "-o", "ConnectTimeout=10", "-o", "ForwardAgent=no", "-o", "ClearAllForwardings=yes", "--", host, "python3 -")
	cmd.Stdin = strings.NewReader(sshUsageScript)
	cmd.Stderr = io.Discard
	cmd.WaitDelay = time.Second
	out, err := cmd.StdoutPipe()
	if err != nil {
		return UsageSnapshot{}, ErrUsageSSH
	}
	if err = cmd.Start(); err != nil {
		return UsageSnapshot{}, ErrUsageSSH
	}
	data, readErr := io.ReadAll(io.LimitReader(out, usageMaxBody+1))
	if readErr != nil || len(data) > usageMaxBody {
		cancel()
	}
	err = cmd.Wait()
	if readErr != nil || len(data) > usageMaxBody {
		return UsageSnapshot{}, ErrUsageParse
	}
	if ctx.Err() != nil {
		return UsageSnapshot{}, ctx.Err()
	}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			switch exit.ExitCode() {
			case 20:
				return UsageSnapshot{}, ErrUnauthorized
			case 21:
				return UsageSnapshot{}, ErrUsageParse
			case 22:
				return UsageSnapshot{}, &RateLimitedError{}
			}
		}
		return UsageSnapshot{}, ErrUsageSSH
	}
	snapshot, err := ParseUsageSnapshot(data)
	if err != nil || len(snapshot.Windows) == 0 {
		return UsageSnapshot{}, ErrUsageParse
	}
	snapshot.FetchedAt = time.Now()
	return snapshot, nil
}

// ValidSSHHost accepts a destination without shell or option syntax.
func ValidSSHHost(host string) bool {
	return !(host == "" || strings.HasPrefix(host, "-") || strings.ContainsAny(host, "\r\n\t ") || strings.ContainsAny(host, "'\"`;|&$\\/<>()"))
}
