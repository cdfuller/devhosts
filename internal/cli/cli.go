package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/cdfuller/devhosts/internal/caddy"
	"github.com/cdfuller/devhosts/internal/cmdutil"
	"github.com/cdfuller/devhosts/internal/config"
	"github.com/cdfuller/devhosts/internal/filesystem"
	"github.com/cdfuller/devhosts/internal/hostsfile"
	"github.com/cdfuller/devhosts/internal/state"
)

const defaultHostsPath = "/etc/hosts"

// App bundles the services backing the CLI.
type App struct {
	Loader    config.Loader
	Hosts     hostsfile.Manager
	Caddy     caddy.Manager
	Stdout    io.Writer
	Stderr    io.Writer
	HostsPath string
}

// Execute is the entrypoint invoked by main.
func Execute(ctx context.Context, args []string) error {
	app := &App{
		Loader:    config.NewLoader(filesystem.OS{}),
		Hosts:     hostsfile.NewManager(filesystem.OS{}),
		Caddy:     caddy.NewManager(filesystem.OS{}, cmdutil.ExecRunner{}),
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		HostsPath: defaultHostsPath,
	}
	return app.Run(ctx, args)
}

// Run parses CLI arguments and dispatches subcommands.
func (a *App) Run(ctx context.Context, args []string) error {
	if a.Stdout == nil {
		a.Stdout = os.Stdout
	}
	if a.Stderr == nil {
		a.Stderr = os.Stderr
	}
	if a.HostsPath == "" {
		a.HostsPath = defaultHostsPath
	}

	root := flag.NewFlagSet("devhosts", flag.ContinueOnError)
	root.SetOutput(a.Stderr)
	var configPath string
	var baseOverride string
	var includeOverride string
	root.StringVar(&configPath, "config", "", "path to devhosts.json")
	root.StringVar(&baseOverride, "caddyfile", "", "path to base Caddyfile")
	root.StringVar(&includeOverride, "include", "", "path to managed include Caddyfile")
	root.Usage = func() {
		fmt.Fprintf(a.Stderr, "Usage: devhosts [global flags] <command> [args]\n\n")
		fmt.Fprintln(a.Stderr, "Commands:")
		fmt.Fprintln(a.Stderr, "  list                 Show managed hostnames, upstreams, and TLS state")
		fmt.Fprintln(a.Stderr, "  add [flags] <spec>   Create/update hosts; TLS on by default (spec = host:port or host=upstream)")
		fmt.Fprintln(a.Stderr, "  remove <host> [...]   Delete one or more managed hosts")
		fmt.Fprintln(a.Stderr, "  apply                Regenerate files from devhosts.json and reload Caddy")
		fmt.Fprintln(a.Stderr, "  path                 Print resolved configuration and Caddyfile paths")
		fmt.Fprintln(a.Stderr)
		fmt.Fprintln(a.Stderr, "Global flags:")
		root.PrintDefaults()
		fmt.Fprintln(a.Stderr)
		fmt.Fprintln(a.Stderr, "Examples:")
		fmt.Fprintln(a.Stderr, "  devhosts add staff:8080 admin=127.0.0.1:9090 --tls")
		fmt.Fprintln(a.Stderr, "  devhosts remove staff admin")
	}

	if err := root.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	remaining := root.Args()
	if len(remaining) == 0 {
		root.Usage()
		return fmt.Errorf("command required")
	}

	loaded, err := a.Loader.Load(config.LoadOptions{
		ConfigPath:               configPath,
		BaseCaddyfileOverride:    baseOverride,
		IncludeCaddyfileOverride: includeOverride,
	})
	if err != nil {
		return err
	}

	cmd := remaining[0]
	cmdArgs := remaining[1:]

	switch cmd {
	case "list":
		return a.handleList(loaded.Snapshot)
	case "add":
		return a.handleAdd(ctx, loaded, cmdArgs)
	case "remove":
		return a.handleRemove(ctx, loaded, cmdArgs)
	case "apply":
		return a.handleApply(ctx, loaded.Snapshot)
	case "path":
		a.printPaths(loaded)
		return nil
	case "help", "--help", "-h":
		root.Usage()
		return nil
	default:
		root.Usage()
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func (a *App) handleList(snapshot state.Snapshot) error {
	if len(snapshot.Hosts) == 0 {
		fmt.Fprintln(a.Stdout, "No hosts managed.")
		return nil
	}
	tw := tabwriter.NewWriter(a.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "HOST\tUPSTREAM\tTLS")
	for _, h := range snapshot.Hosts {
		tlsState := "disabled"
		if h.TLS {
			tlsState = "internal"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", h.Name, h.Upstream, tlsState)
	}
	return tw.Flush()
}

func (a *App) handleAdd(ctx context.Context, loaded config.Loaded, args []string) error {
	addFlags := flag.NewFlagSet("add", flag.ContinueOnError)
	addFlags.SetOutput(a.Stderr)
	var enableTLS bool
	var disableTLS bool
	addFlags.BoolVar(&enableTLS, "tls", false, "enable tls internal for all provided hosts")
	addFlags.BoolVar(&disableTLS, "no-tls", false, "disable tls internal for all provided hosts")
	addFlags.Usage = func() {
		fmt.Fprintf(a.Stderr, "Usage: devhosts add [flags] <host spec> [...]\n\n")
		fmt.Fprintln(a.Stderr, "Host specs:")
		fmt.Fprintln(a.Stderr, "  host:port            short form; upstream becomes http://localhost:port")
		fmt.Fprintln(a.Stderr, "  host=UPSTREAM        explicit URL; adds http:// prefix if missing")
		fmt.Fprintln(a.Stderr)
		fmt.Fprintln(a.Stderr, "Flags:")
		fmt.Fprintln(a.Stderr, "      --tls               ensure tls internal stays enabled for provided hosts")
		fmt.Fprintln(a.Stderr, "      --no-tls            disable tls internal for provided hosts")
	}
	if err := addFlags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if enableTLS && disableTLS {
		return fmt.Errorf("cannot use --tls and --no-tls together")
	}
	hostArgs := addFlags.Args()
	if len(hostArgs) == 0 {
		return fmt.Errorf("at least one host spec is required")
	}

	desired := cloneSnapshot(loaded.Snapshot)
	existing := make(map[string]int, len(desired.Hosts))
	for i, h := range desired.Hosts {
		existing[h.Name] = i
	}

	var forcedTLS *bool
	if enableTLS {
		v := true
		forcedTLS = &v
	} else if disableTLS {
		v := false
		forcedTLS = &v
	}

	for _, spec := range hostArgs {
		name, upstream, err := parseHostSpec(spec)
		if err != nil {
			return err
		}
		host := state.Host{Name: name, Upstream: upstream, TLS: true}
		if forcedTLS != nil {
			host.TLS = *forcedTLS
		} else if idx, ok := existing[name]; ok {
			host.TLS = desired.Hosts[idx].TLS
		}
		if idx, ok := existing[name]; ok {
			desired.Hosts[idx] = host
		} else {
			desired.Hosts = append(desired.Hosts, host)
			existing[name] = len(desired.Hosts) - 1
		}
	}

	if err := state.ValidateSnapshot(desired); err != nil {
		return err
	}

	outcome, err := a.applyState(ctx, desired)
	if err != nil {
		return err
	}
	if err := a.Loader.Save(loaded.Path, desired); err != nil {
		if rbErr := a.rollbackOutcome(outcome); rbErr != nil {
			return fmt.Errorf("save config: %w (rollback failed: %v)", err, rbErr)
		}
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Fprintf(a.Stdout, "Configured %d host(s).\n", len(hostArgs))
	return nil
}

func (a *App) handleRemove(ctx context.Context, loaded config.Loaded, args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(a.Stderr, "Usage: devhosts remove <host> [<host> ...]")
		return fmt.Errorf("at least one host name is required")
	}
	desired := cloneSnapshot(loaded.Snapshot)
	removed := 0
	var missing []string
	for _, raw := range args {
		name := state.NormalizeHostName(raw)
		idx := findHostIndex(desired.Hosts, name)
		if idx == -1 {
			missing = append(missing, name)
			continue
		}
		desired.Hosts = append(desired.Hosts[:idx], desired.Hosts[idx+1:]...)
		removed++
	}

	if len(missing) > 0 {
		for _, name := range missing {
			fmt.Fprintf(a.Stderr, "Warning: host %s not managed.\n", name)
		}
	}
	if removed == 0 {
		return nil
	}

	if err := state.ValidateSnapshot(desired); err != nil {
		return err
	}

	outcome, err := a.applyState(ctx, desired)
	if err != nil {
		return err
	}
	if err := a.Loader.Save(loaded.Path, desired); err != nil {
		if rbErr := a.rollbackOutcome(outcome); rbErr != nil {
			return fmt.Errorf("save config: %w (rollback failed: %v)", err, rbErr)
		}
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Fprintf(a.Stdout, "Removed %d host(s).\n", removed)
	return nil
}

func (a *App) handleApply(ctx context.Context, snapshot state.Snapshot) error {
	if err := state.ValidateSnapshot(snapshot); err != nil {
		return err
	}
	if _, err := a.applyState(ctx, snapshot); err != nil {
		return err
	}
	fmt.Fprintln(a.Stdout, "State applied.")
	return nil
}

func (a *App) printPaths(loaded config.Loaded) {
	fmt.Fprintf(a.Stdout, "Config: %s\n", loaded.Path)
	fmt.Fprintf(a.Stdout, "Base Caddyfile: %s\n", loaded.Snapshot.BaseCaddyfile)
	fmt.Fprintf(a.Stdout, "Include Caddyfile: %s\n", loaded.Snapshot.IncludeCaddyfile)
}

type applyOutcome struct {
	include caddy.UpdateResult
	hosts   hostsfile.ApplyResult
}

func (a *App) applyState(ctx context.Context, snapshot state.Snapshot) (applyOutcome, error) {
	if err := a.Caddy.EnsureBaseReady(snapshot.BaseCaddyfile, snapshot.IncludeCaddyfile, snapshot.Hosts); err != nil {
		return applyOutcome{}, err
	}

	includeRes, err := a.Caddy.UpdateInclude(snapshot.IncludeCaddyfile, a.Caddy.GenerateInclude(snapshot.Hosts))
	if err != nil {
		return applyOutcome{}, err
	}

	hostsRes, err := a.Hosts.Apply(a.HostsPath, snapshot.Hosts)
	if err != nil {
		_ = a.Caddy.RestoreInclude(includeRes)
		return applyOutcome{}, err
	}

	reloadOut, err := a.Caddy.Reload(ctx, snapshot.BaseCaddyfile)
	if err != nil {
		_ = a.Caddy.RestoreInclude(includeRes)
		_ = a.Hosts.Restore(hostsRes)
		details := strings.TrimSpace(string(reloadOut.Stderr))
		if details == "" {
			details = strings.TrimSpace(string(reloadOut.Stdout))
		}
		if details != "" {
			return applyOutcome{}, fmt.Errorf("caddy reload failed: %w: %s", err, details)
		}
		return applyOutcome{}, fmt.Errorf("caddy reload failed: %w", err)
	}

	return applyOutcome{include: includeRes, hosts: hostsRes}, nil
}

func (a *App) rollbackOutcome(outcome applyOutcome) error {
	var errs []string
	if outcome.include.Changed {
		if err := a.Caddy.RestoreInclude(outcome.include); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if outcome.hosts.Changed {
		if err := a.Hosts.Restore(outcome.hosts); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func parseHostSpec(spec string) (string, string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", "", fmt.Errorf("empty host spec")
	}
	if strings.Contains(spec, "=") {
		parts := strings.SplitN(spec, "=", 2)
		name := state.NormalizeHostName(parts[0])
		upstream := strings.TrimSpace(parts[1])
		if name == "" {
			return "", "", fmt.Errorf("host name missing before '='")
		}
		if upstream == "" {
			return "", "", fmt.Errorf("upstream missing after '='")
		}
		if !strings.Contains(upstream, "://") {
			upstream = "http://" + upstream
		}
		return name, upstream, nil
	}
	idx := strings.LastIndex(spec, ":")
	if idx <= 0 || idx == len(spec)-1 {
		return "", "", fmt.Errorf("invalid host spec %q; use host:port or host=upstream", spec)
	}
	name := state.NormalizeHostName(spec[:idx])
	port := spec[idx+1:]
	if name == "" {
		return "", "", fmt.Errorf("host name missing in %q", spec)
	}
	return name, fmt.Sprintf("http://localhost:%s", port), nil
}

func findHostIndex(hosts []state.Host, name string) int {
	for i, h := range hosts {
		if h.Name == name {
			return i
		}
	}
	return -1
}

func cloneSnapshot(s state.Snapshot) state.Snapshot {
	clone := s
	clone.Hosts = append([]state.Host(nil), s.Hosts...)
	return clone
}
