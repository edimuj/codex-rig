package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/edimuj/codex-rig/internal/rig"
)

var version = "dev"

type commandContext struct {
	store *rig.Store
	paths rig.Paths
	cwd   string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "codex-rig:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "--version", "-version", "version":
		fmt.Println(version)
		return nil
	case "create":
		return runCreate(args[1:])
	case "list":
		return runList(args[1:])
	case "use":
		return runUse(args[1:])
	case "status":
		return runStatus(args[1:])
	case "launch":
		return runLaunch(args[1:])
	case "doctor":
		return runDoctor(args[1:])
	case "diff":
		return runDiff(args[1:])
	case "share":
		return runSetMode(args[1:], rig.PolicyShared, "share")
	case "isolate":
		return runSetMode(args[1:], rig.PolicyIsolated, "isolate")
	case "inherit":
		return runSetMode(args[1:], rig.PolicyInherited, "inherit")
	case "auth":
		return runAuth(args[1:])
	case "instructions":
		return runInstructions(args[1:])
	case "rc":
		return runRC(args[1:])
	case "help", "--help", "-h":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runCreate(args []string) error {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: codex-rig create <name>")
	}

	ctx, err := newCommandContext()
	if err != nil {
		return err
	}
	name := fs.Arg(0)
	cfg, err := ctx.store.CreateRig(name)
	if err != nil {
		return err
	}
	fmt.Printf("created rig %q at %s\n", cfg.Name, ctx.store.RigDir(cfg.Name))
	fmt.Printf("codex_home=%s\n", ctx.store.RigCodexHome(cfg.Name))
	return nil
}

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, err := newCommandContext()
	if err != nil {
		return err
	}
	rigs, err := ctx.store.ListRigs()
	if err != nil {
		return err
	}
	if len(rigs) == 0 {
		fmt.Println("no rigs")
		return nil
	}

	current, _ := ctx.store.CurrentRig()
	_, marker, foundMarker, _ := rig.FindMarker(ctx.cwd)

	for _, name := range rigs {
		tags := make([]string, 0, 2)
		if name == current {
			tags = append(tags, "current")
		}
		if foundMarker && name == marker.Rig {
			tags = append(tags, "marker")
		}
		if len(tags) == 0 {
			fmt.Println(name)
			continue
		}
		fmt.Printf("%s [%s]\n", name, strings.Join(tags, ","))
	}
	return nil
}

func runUse(args []string) error {
	fs := flag.NewFlagSet("use", flag.ContinueOnError)
	noMarker := fs.Bool("no-marker", false, "set current rig only; do not write project marker")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: codex-rig use [--no-marker] <name>")
	}

	ctx, err := newCommandContext()
	if err != nil {
		return err
	}
	name := fs.Arg(0)
	if _, err := ctx.store.LoadRig(name); err != nil {
		return err
	}
	if err := ctx.store.SetCurrentRig(name); err != nil {
		return err
	}

	if !*noMarker {
		repoRoot, findErr := rig.FindRepoRoot(ctx.cwd)
		if findErr == nil {
			if writeErr := rig.WriteMarker(repoRoot, name); writeErr != nil {
				return writeErr
			}
			fmt.Printf("project marker: %s\n", filepath.Join(repoRoot, rig.MarkerFileName))
		} else {
			fmt.Fprintf(os.Stderr, "warning: %v\n", findErr)
		}
	}

	fmt.Printf("current rig: %s\n", name)
	return nil
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, err := newCommandContext()
	if err != nil {
		return err
	}

	rigs, err := ctx.store.ListRigs()
	if err != nil {
		return err
	}
	current, err := ctx.store.CurrentRig()
	if err != nil {
		return err
	}
	markerPath, marker, markerFound, err := rig.FindMarker(ctx.cwd)
	if err != nil {
		return err
	}

	effectiveRig := ""
	effectiveSource := "none"
	if markerFound {
		effectiveRig = marker.Rig
		effectiveSource = "marker"
	} else if current != "" {
		effectiveRig = current
		effectiveSource = "current"
	}

	fmt.Printf("rig_root=%s\n", ctx.paths.RigRoot)
	fmt.Printf("global_codex_home=%s\n", ctx.paths.GlobalCodexHome)
	fmt.Printf("rig_count=%d\n", len(rigs))
	fmt.Printf("current_rig=%s\n", defaultValue(current, "<none>"))
	if markerFound {
		fmt.Printf("project_marker=%s\n", markerPath)
		fmt.Printf("project_rig=%s\n", marker.Rig)
	} else {
		fmt.Println("project_marker=<none>")
	}
	fmt.Printf("effective_rig=%s\n", defaultValue(effectiveRig, "<none>"))
	fmt.Printf("effective_source=%s\n", effectiveSource)
	if effectiveRig != "" {
		fmt.Printf("effective_codex_home=%s\n", ctx.store.RigCodexHome(effectiveRig))
		cfg, loadErr := ctx.store.LoadRig(effectiveRig)
		if loadErr == nil {
			fmt.Printf("auth_mode=%s\n", cfg.Policy[rig.CategoryAuth])
			if cfg.Policy[rig.CategoryAuth] == rig.PolicyShared {
				fmt.Printf("auth_source=%s\n", cfg.AuthLinkSource())
			}
		}
	}
	return nil
}

func runLaunch(args []string) error {
	fs := flag.NewFlagSet("launch", flag.ContinueOnError)
	rigName := fs.String("rig", "", "rig name override")
	codexBin := fs.String("codex-bin", "codex", "codex binary path/name")
	dryRun := fs.Bool("dry-run", false, "print launch info without executing codex")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, err := newCommandContext()
	if err != nil {
		return err
	}

	resolvedRig, markerPath, err := rig.ResolveLaunchRig(ctx.store, ctx.cwd, *rigName)
	if err != nil {
		return err
	}
	cfg, err := ctx.store.LoadRig(resolvedRig)
	if err != nil {
		return err
	}

	rigCodexHome := ctx.store.RigCodexHome(resolvedRig)
	if err := os.MkdirAll(rigCodexHome, 0o755); err != nil {
		return err
	}
	if err := rig.EnsureRigBootstrap(ctx.store, cfg); err != nil {
		return err
	}

	passThroughArgs := fs.Args()
	env := rig.BuildLaunchEnv(os.Environ(), ctx.paths.RigRoot, resolvedRig, rigCodexHome)
	if *dryRun {
		fmt.Printf("rig=%s\n", resolvedRig)
		if markerPath != "" {
			fmt.Printf("marker=%s\n", markerPath)
		}
		fmt.Printf("codex_bin=%s\n", *codexBin)
		fmt.Printf("codex_home=%s\n", rigCodexHome)
		fmt.Printf("args=%s\n", strings.Join(passThroughArgs, " "))
		return nil
	}

	cmd := exec.Command(*codexBin, passThroughArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return err
		}
		return fmt.Errorf("launch failed: %w", err)
	}
	return nil
}

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, err := newCommandContext()
	if err != nil {
		return err
	}

	issues := 0
	reportIssue := func(format string, values ...any) {
		issues++
		fmt.Printf("issue: "+format+"\n", values...)
	}

	if _, err := os.Stat(ctx.paths.RigRoot); err != nil {
		if os.IsNotExist(err) {
			reportIssue("rig root missing: %s", ctx.paths.RigRoot)
		} else {
			reportIssue("failed reading rig root %s: %v", ctx.paths.RigRoot, err)
		}
	}

	rigs, err := ctx.store.ListRigs()
	if err != nil {
		return err
	}

	for _, name := range rigs {
		cfg, loadErr := ctx.store.LoadRig(name)
		if loadErr != nil {
			reportIssue("invalid config for rig %s: %v", name, loadErr)
			continue
		}

		rigCodexHome := ctx.store.RigCodexHome(name)
		if _, statErr := os.Stat(rigCodexHome); statErr != nil {
			reportIssue("missing codex home for rig %s: %s", name, rigCodexHome)
			continue
		}

		diffs, diffErr := rig.DiffPolicyState(ctx.store, cfg)
		if diffErr != nil {
			reportIssue("failed policy diff for rig %s: %v", name, diffErr)
			continue
		}
		for _, diff := range diffs {
			if diff.Match {
				continue
			}
			reportIssue("rig %s category %s drift at %s (desired=%s actual=%s)", name, diff.Category, diff.LocalPath, diff.Desired, diff.Actual)
		}
	}

	current, currentErr := ctx.store.CurrentRig()
	if currentErr != nil {
		reportIssue("failed to read current rig: %v", currentErr)
	} else if current != "" && !ctx.store.RigExists(current) {
		reportIssue("current rig %q does not exist", current)
	}

	markerPath, marker, markerFound, markerErr := rig.FindMarker(ctx.cwd)
	if markerErr != nil {
		reportIssue("invalid marker from cwd %s: %v", ctx.cwd, markerErr)
	} else if markerFound && !ctx.store.RigExists(marker.Rig) {
		reportIssue("marker %s points to missing rig %q", markerPath, marker.Rig)
	}

	if issues == 0 {
		fmt.Println("doctor: OK")
		return nil
	}
	return fmt.Errorf("doctor: found %d issue(s)", issues)
}

func runDiff(args []string) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	rigName := fs.String("rig", "", "rig name override")
	showAll := fs.Bool("all", false, "show all managed entries (default only drift)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, err := newCommandContext()
	if err != nil {
		return err
	}

	resolvedRig, _, err := rig.ResolveLaunchRig(ctx.store, ctx.cwd, *rigName)
	if err != nil {
		return err
	}
	cfg, err := ctx.store.LoadRig(resolvedRig)
	if err != nil {
		return err
	}

	diffs, err := rig.DiffPolicyState(ctx.store, cfg)
	if err != nil {
		return err
	}

	driftCount := 0
	for _, diff := range diffs {
		if !diff.Match {
			driftCount++
		}
		if !*showAll && diff.Match {
			continue
		}
		status := "OK"
		if !diff.Match {
			status = "DRIFT"
		}
		fmt.Printf("[%s] %-12s mode=%-10s path=%s desired=%s actual=%s\n", status, diff.Category, diff.Mode, diff.LocalPath, diff.Desired, diff.Actual)
	}

	if driftCount == 0 {
		fmt.Printf("no drift for rig %s\n", resolvedRig)
		return nil
	}
	return fmt.Errorf("found %d drift item(s) for rig %s", driftCount, resolvedRig)
}

func runSetMode(args []string, mode rig.PolicyMode, command string) error {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	rigName := fs.String("rig", "", "rig name override")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("usage: codex-rig %s [--rig <name>] <category...>", command)
	}

	ctx, err := newCommandContext()
	if err != nil {
		return err
	}

	targetRig, err := resolveTargetRig(ctx, *rigName)
	if err != nil {
		return err
	}
	cfg, err := ctx.store.LoadRig(targetRig)
	if err != nil {
		return err
	}

	changed := make([]string, 0, fs.NArg())
	for _, rawCategory := range fs.Args() {
		category := rig.NormalizeCategory(rawCategory)
		if !rig.IsManagedCategory(category) {
			return fmt.Errorf("unknown category %q", rawCategory)
		}
		if mode == rig.PolicyInherited && !rig.SupportsInherited(category) {
			return fmt.Errorf("category %q does not support inherited mode", category)
		}
		cfg.Policy[category] = mode
		if category == rig.CategoryAuth {
			if mode == rig.PolicyShared {
				if cfg.Links == nil {
					cfg.Links = map[string]string{}
				}
				if strings.TrimSpace(cfg.Links[rig.CategoryAuth]) == "" {
					cfg.Links[rig.CategoryAuth] = rig.AuthSourceGlobal
				}
			} else if cfg.Links != nil {
				delete(cfg.Links, rig.CategoryAuth)
			}
		}
		changed = append(changed, category)
	}

	if err := ctx.store.SaveRigConfig(cfg); err != nil {
		return err
	}
	sort.Strings(changed)
	fmt.Printf("rig=%s mode=%s categories=%s\n", targetRig, mode, strings.Join(changed, ","))
	return nil
}

func runAuth(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: codex-rig auth <status|link|unlink> [flags]")
	}
	switch args[0] {
	case "status":
		return runAuthStatus(args[1:])
	case "link":
		return runAuthLink(args[1:])
	case "unlink":
		return runAuthUnlink(args[1:])
	default:
		return fmt.Errorf("unknown auth subcommand %q", args[0])
	}
}

func runInstructions(args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return runInstructionsShow(args)
	}
	switch args[0] {
	case "show", "status":
		return runInstructionsShow(args[1:])
	case "sync":
		return runInstructionsSync(args[1:])
	default:
		return fmt.Errorf("unknown instructions subcommand %q", args[0])
	}
}

func runInstructionsShow(args []string) error {
	fs := flag.NewFlagSet("instructions", flag.ContinueOnError)
	rigName := fs.String("rig", "", "rig name override")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: codex-rig instructions [show] [--rig <name>]")
	}

	ctx, err := newCommandContext()
	if err != nil {
		return err
	}
	targetRig, err := resolveTargetRig(ctx, *rigName)
	if err != nil {
		return err
	}
	cfg, err := ctx.store.LoadRig(targetRig)
	if err != nil {
		return err
	}
	meta, err := rig.GetInstructionMetadata(ctx.store, cfg.Name)
	if err != nil {
		return err
	}

	fmt.Printf("rig=%s\n", cfg.Name)
	fmt.Printf("global_source=%s\n", defaultValue(meta.GlobalSourcePath, "<none>"))
	fmt.Printf("rig_fragment=%s\n", meta.RigFragmentPath)
	fmt.Printf("generated_override=%s\n", meta.GeneratedOverridePath)
	return nil
}

func runInstructionsSync(args []string) error {
	fs := flag.NewFlagSet("instructions sync", flag.ContinueOnError)
	rigName := fs.String("rig", "", "rig name override")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: codex-rig instructions sync [--rig <name>]")
	}

	ctx, err := newCommandContext()
	if err != nil {
		return err
	}
	targetRig, err := resolveTargetRig(ctx, *rigName)
	if err != nil {
		return err
	}
	cfg, err := ctx.store.LoadRig(targetRig)
	if err != nil {
		return err
	}
	if err := rig.EnsureRigInstructions(ctx.store, cfg); err != nil {
		return err
	}
	meta, err := rig.GetInstructionMetadata(ctx.store, cfg.Name)
	if err != nil {
		return err
	}
	fmt.Printf("rig=%s\n", cfg.Name)
	fmt.Printf("generated_override=%s\n", meta.GeneratedOverridePath)
	fmt.Println("instructions_state=synced")
	return nil
}

func runRC(args []string) error {
	if len(args) == 0 {
		return runRCShow(args)
	}
	switch args[0] {
	case "show", "status":
		return runRCShow(args[1:])
	case "set":
		return runRCSet(args[1:])
	case "clear", "unset":
		return runRCClear(args[1:])
	case "init":
		return runRCInit(args[1:])
	default:
		return fmt.Errorf("unknown rc subcommand %q", args[0])
	}
}

func runRCShow(args []string) error {
	fs := flag.NewFlagSet("rc", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: codex-rig rc [show]")
	}

	ctx, err := newCommandContext()
	if err != nil {
		return err
	}

	repoRoot, markerPath, err := repoMarkerPath(ctx)
	if err != nil {
		return err
	}
	current, err := ctx.store.CurrentRig()
	if err != nil {
		return err
	}
	markerRig := ""
	if marker, readErr := rig.ReadMarker(markerPath); readErr == nil {
		markerRig = marker.Rig
	} else if !os.IsNotExist(readErr) {
		return readErr
	}

	effectiveRig := markerRig
	effectiveSource := "marker"
	if strings.TrimSpace(effectiveRig) == "" {
		effectiveRig = current
		effectiveSource = "current"
	}
	if strings.TrimSpace(effectiveRig) == "" {
		effectiveSource = "none"
	}

	fmt.Printf("repo_root=%s\n", repoRoot)
	fmt.Printf("marker_path=%s\n", markerPath)
	fmt.Printf("marker_rig=%s\n", defaultValue(markerRig, "<none>"))
	fmt.Printf("current_rig=%s\n", defaultValue(current, "<none>"))
	fmt.Printf("effective_rig=%s\n", defaultValue(effectiveRig, "<none>"))
	fmt.Printf("effective_source=%s\n", effectiveSource)
	return nil
}

func runRCSet(args []string) error {
	fs := flag.NewFlagSet("rc set", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: codex-rig rc set <rig>")
	}

	ctx, err := newCommandContext()
	if err != nil {
		return err
	}
	rigName := fs.Arg(0)
	if _, err := ctx.store.LoadRig(rigName); err != nil {
		return err
	}
	_, markerPath, err := repoMarkerPath(ctx)
	if err != nil {
		return err
	}
	if err := rig.WriteMarker(filepath.Dir(markerPath), rigName); err != nil {
		return err
	}
	fmt.Printf("marker_path=%s\n", markerPath)
	fmt.Printf("marker_rig=%s\n", rigName)
	return nil
}

func runRCClear(args []string) error {
	fs := flag.NewFlagSet("rc clear", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: codex-rig rc clear")
	}

	ctx, err := newCommandContext()
	if err != nil {
		return err
	}
	_, markerPath, err := repoMarkerPath(ctx)
	if err != nil {
		return err
	}
	if err := os.Remove(markerPath); err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("marker_path=%s\n", markerPath)
			fmt.Println("marker_state=absent")
			return nil
		}
		return err
	}
	fmt.Printf("marker_path=%s\n", markerPath)
	fmt.Println("marker_state=removed")
	return nil
}

func runRCInit(args []string) error {
	fs := flag.NewFlagSet("rc init", flag.ContinueOnError)
	rigName := fs.String("rig", "", "rig name to initialize in marker")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: codex-rig rc init [--rig <name>]")
	}

	ctx, err := newCommandContext()
	if err != nil {
		return err
	}
	_, markerPath, err := repoMarkerPath(ctx)
	if err != nil {
		return err
	}
	if marker, readErr := rig.ReadMarker(markerPath); readErr == nil {
		fmt.Printf("marker_path=%s\n", markerPath)
		fmt.Printf("marker_rig=%s\n", marker.Rig)
		fmt.Println("marker_state=kept")
		return nil
	} else if !os.IsNotExist(readErr) {
		return readErr
	}

	targetRig := strings.TrimSpace(*rigName)
	if targetRig == "" {
		current, currentErr := ctx.store.CurrentRig()
		if currentErr != nil {
			return currentErr
		}
		targetRig = current
	}
	if strings.TrimSpace(targetRig) == "" {
		return errors.New("no rig specified and no current rig set")
	}
	if _, err := ctx.store.LoadRig(targetRig); err != nil {
		return err
	}
	if err := rig.WriteMarker(filepath.Dir(markerPath), targetRig); err != nil {
		return err
	}
	fmt.Printf("marker_path=%s\n", markerPath)
	fmt.Printf("marker_rig=%s\n", targetRig)
	fmt.Println("marker_state=created")
	return nil
}

func runAuthStatus(args []string) error {
	fs := flag.NewFlagSet("auth status", flag.ContinueOnError)
	rigName := fs.String("rig", "", "rig name override")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, err := newCommandContext()
	if err != nil {
		return err
	}

	targetRig, err := resolveTargetRig(ctx, *rigName)
	if err != nil {
		return err
	}
	cfg, err := ctx.store.LoadRig(targetRig)
	if err != nil {
		return err
	}

	localAuthPath := filepath.Join(ctx.store.RigCodexHome(targetRig), "auth.json")
	localState, localErr := describePath(localAuthPath)
	if localErr != nil {
		return localErr
	}

	fmt.Printf("rig=%s\n", targetRig)
	fmt.Printf("mode=%s\n", cfg.Policy[rig.CategoryAuth])
	fmt.Printf("local_path=%s\n", localAuthPath)
	fmt.Printf("local_state=%s\n", localState)
	if cfg.Policy[rig.CategoryAuth] == rig.PolicyShared {
		source := cfg.AuthLinkSource()
		targetPath, sourceErr := ctx.store.ResolveAuthSourcePath(cfg)
		if sourceErr != nil {
			return sourceErr
		}
		fmt.Printf("source=%s\n", source)
		fmt.Printf("target_path=%s\n", targetPath)
	}
	return nil
}

func runAuthLink(args []string) error {
	fs := flag.NewFlagSet("auth link", flag.ContinueOnError)
	rigName := fs.String("rig", "", "rig name override")
	source := fs.String("source", rig.AuthSourceGlobal, "link source: global or rig:<name>")
	fromRig := fs.String("from-rig", "", "shortcut for --source rig:<name>")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, err := newCommandContext()
	if err != nil {
		return err
	}

	targetRig, err := resolveTargetRig(ctx, *rigName)
	if err != nil {
		return err
	}
	cfg, err := ctx.store.LoadRig(targetRig)
	if err != nil {
		return err
	}

	normalizedSource := rig.NormalizeAuthLinkSource(*source)
	if strings.TrimSpace(*fromRig) != "" {
		normalizedSource = rig.AuthSourceRig + strings.TrimSpace(*fromRig)
	}
	kind, sourceRig, err := rig.ParseAuthLinkSource(normalizedSource)
	if err != nil {
		return err
	}
	if kind == rig.AuthSourceRig {
		if sourceRig == targetRig {
			return errors.New("auth source rig cannot be same as target rig")
		}
		if _, loadErr := ctx.store.LoadRig(sourceRig); loadErr != nil {
			return fmt.Errorf("auth source rig %q: %w", sourceRig, loadErr)
		}
	}

	if cfg.Links == nil {
		cfg.Links = map[string]string{}
	}
	cfg.Policy[rig.CategoryAuth] = rig.PolicyShared
	cfg.Links[rig.CategoryAuth] = normalizedSource

	if err := ctx.store.SaveRigConfig(cfg); err != nil {
		return err
	}

	targetPath, err := ctx.store.ResolveAuthSourcePath(cfg)
	if err != nil {
		return err
	}
	fmt.Printf("rig=%s auth_mode=%s source=%s target=%s\n", targetRig, cfg.Policy[rig.CategoryAuth], cfg.AuthLinkSource(), targetPath)
	return nil
}

func runAuthUnlink(args []string) error {
	fs := flag.NewFlagSet("auth unlink", flag.ContinueOnError)
	rigName := fs.String("rig", "", "rig name override")
	discard := fs.Bool("discard", false, "do not copy current effective auth file into isolated auth.json")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, err := newCommandContext()
	if err != nil {
		return err
	}

	targetRig, err := resolveTargetRig(ctx, *rigName)
	if err != nil {
		return err
	}
	cfg, err := ctx.store.LoadRig(targetRig)
	if err != nil {
		return err
	}

	localAuthPath := filepath.Join(ctx.store.RigCodexHome(targetRig), "auth.json")
	var snapshot []byte
	if !*discard {
		snapshot, _ = os.ReadFile(localAuthPath)
	}

	cfg.Policy[rig.CategoryAuth] = rig.PolicyIsolated
	if cfg.Links != nil {
		delete(cfg.Links, rig.CategoryAuth)
	}
	if err := ctx.store.SaveRigConfig(cfg); err != nil {
		return err
	}

	if !*discard && len(snapshot) > 0 {
		if err := os.WriteFile(localAuthPath, snapshot, 0o600); err != nil {
			return err
		}
	}

	fmt.Printf("rig=%s auth_mode=%s local_path=%s\n", targetRig, cfg.Policy[rig.CategoryAuth], localAuthPath)
	if !*discard && len(snapshot) > 0 {
		fmt.Println("auth_snapshot=preserved")
	}
	return nil
}

func resolveTargetRig(ctx *commandContext, explicitRig string) (string, error) {
	rigName, _, err := rig.ResolveLaunchRig(ctx.store, ctx.cwd, explicitRig)
	if err != nil {
		return "", err
	}
	return rigName, nil
}

func describePath(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "missing", nil
		}
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, readErr := os.Readlink(path)
		if readErr != nil {
			return "", readErr
		}
		if !filepath.IsAbs(target) {
			target = filepath.Clean(filepath.Join(filepath.Dir(path), target))
		}
		return "symlink:" + filepath.Clean(target), nil
	}
	if info.IsDir() {
		return "dir", nil
	}
	if info.Mode().IsRegular() {
		return "file", nil
	}
	return "other", nil
}

func repoMarkerPath(ctx *commandContext) (repoRoot string, markerPath string, err error) {
	repoRoot, err = rig.FindRepoRoot(ctx.cwd)
	if err != nil {
		return "", "", err
	}
	return repoRoot, filepath.Join(repoRoot, rig.MarkerFileName), nil
}

func newCommandContext() (*commandContext, error) {
	paths, err := rig.ResolvePathsForCurrentUser()
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	ctx := &commandContext{
		store: rig.NewStore(paths),
		paths: paths,
		cwd:   cwd,
	}
	if err := os.MkdirAll(ctx.paths.RigRoot, 0o755); err != nil {
		return nil, err
	}
	return ctx, nil
}

func defaultValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func printUsage() {
	fmt.Println(`codex-rig - thin orchestration layer for multi-rig Codex workflows

Usage:
  codex-rig create <name>
  codex-rig list
  codex-rig use [--no-marker] <name>
  codex-rig status
  codex-rig launch [--rig <name>] [--codex-bin <path>] [-- <codex args...>]
  codex-rig share [--rig <name>] <category...>
  codex-rig isolate [--rig <name>] <category...>
  codex-rig inherit [--rig <name>] <category...>
  codex-rig auth status [--rig <name>]
  codex-rig auth link [--rig <name>] [--source global|rig:<name>] [--from-rig <name>]
  codex-rig auth unlink [--rig <name>] [--discard]
  codex-rig instructions [show] [--rig <name>]
  codex-rig instructions sync [--rig <name>]
  codex-rig rc [show]
  codex-rig rc set <rig>
  codex-rig rc init [--rig <name>]
  codex-rig rc clear
  codex-rig doctor
  codex-rig diff [--rig <name>] [--all]
  codex-rig version

Managed categories:
  auth, skills, plugins, mcp, history/logs`)
}
