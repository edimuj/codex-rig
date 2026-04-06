package rig

import "testing"

func TestBuildLaunchEnvOverridesAndAdds(t *testing.T) {
	base := []string{
		"PATH=/usr/bin",
		"CODEX_HOME=/old/home",
		"CODEX_RIG=old",
	}
	env := BuildLaunchEnv(base, "/tmp/rig-root", "alpha", "/tmp/rig-root/rigs/alpha/codex-home")

	assertHas := func(expected string) {
		t.Helper()
		for _, value := range env {
			if value == expected {
				return
			}
		}
		t.Fatalf("expected env to contain %q", expected)
	}

	assertHas("PATH=/usr/bin")
	assertHas("CODEX_HOME=/tmp/rig-root/rigs/alpha/codex-home")
	assertHas("CODEX_RIG=alpha")
	assertHas("CODEX_RIG_ROOT=/tmp/rig-root")
}
