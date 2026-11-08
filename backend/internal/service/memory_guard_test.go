package service

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// runMemoryGuardInSubprocess keeps process-wide allocation and heap counters
// independent of background services started by other tests. The child runs
// the same test binary (and build tags) with all original assertions enabled.
// A true result tells the parent to return; the child continues the test.
func runMemoryGuardInSubprocess(t *testing.T) bool {
	t.Helper()
	const subprocessEnv = "SUB2API_MEMORY_GUARD_TEST"
	if os.Getenv(subprocessEnv) == t.Name() {
		return false
	}
	executable, err := os.Executable()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^"+regexp.QuoteMeta(t.Name())+"$", "-test.count=1", "-test.timeout=25s")
	cmd.Env = append(os.Environ(), subprocessEnv+"="+t.Name())
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "isolated memory guard failed: %s", output)
	return true
}
