package deviceclient

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Source is a persistent grant for an entire usage root (including future files).
// Paths are canonical absolute paths, never patterns or collector auto-discovery.
type Source struct {
	Agent string `json:"agent"`
	Path  string `json:"path"`
}

type sourceSpec struct {
	env          string
	defaults     []string
	single, file bool
}

// Audited against ccusage v20.0.43 rust/adapters/*/src/paths.rs.
var sourceSpecs = map[string]sourceSpec{
	"claude":   {"CLAUDE_CONFIG_DIR", []string{".claude", ".config/claude"}, false, false},
	"codex":    {"CODEX_HOME", []string{".codex"}, false, false},
	"opencode": {"OPENCODE_DATA_DIR", []string{".local/share/opencode"}, false, false},
	"amp":      {"AMP_DATA_DIR", []string{".local/share/amp"}, false, false},
	"droid":    {"DROID_SESSIONS_DIR", []string{".factory/sessions"}, false, false},
	"codebuff": {"CODEBUFF_DATA_DIR", []string{".config/manicode", ".config/manicode-dev", ".config/manicode-staging"}, false, false},
	"hermes":   {"HERMES_HOME", []string{".hermes"}, false, false},
	"pi":       {"PI_AGENT_DIR", []string{".pi/agent/sessions", ".config/pi-t3/agent/sessions"}, false, false},
	// Goose's override accepts only <root>/data/sessions/sessions.db, not its
	// default database locations. Do not pretend an arbitrary directory works.
	"goose":        {"GOOSE_PATH_ROOT", nil, true, false},
	"kilo":         {"KILO_DATA_DIR", []string{".local/share/kilo"}, false, false},
	"copilot":      {"COPILOT_HOME", []string{".copilot"}, true, false},
	"copilot-otel": {"COPILOT_OTEL_FILE_EXPORTER_PATH", nil, true, true},
	"gemini":       {"GEMINI_DATA_DIR", []string{".gemini/tmp"}, false, false},
	"antigravity":  {"ANTIGRAVITY_DATA_DIR", []string{".gemini/antigravity/conversations", ".gemini/antigravity-cli/conversations", ".gemini/antigravity-ide/conversations", ".gemini/antigravity-backup/conversations", ".gemini/antigravity-acp/conversations", ".config/antigravity/conversations"}, false, false},
	"kimi":         {"KIMI_DATA_DIR", []string{".kimi", ".kimi-code"}, false, false},
	"qwen":         {"QWEN_DATA_DIR", []string{".qwen"}, false, false},
	"openclaw":     {"OPENCLAW_DIR", []string{".openclaw", ".clawdbot", ".moltbot", ".moldbot"}, false, false},
	"grok":         {"GROK_HOME", []string{".grok"}, true, false},
	"zcode":        {"ZCODE_HOME", []string{".zcode"}, false, false},
}

func sourceNames() []string {
	names := make([]string, 0, len(sourceSpecs))
	for name := range sourceSpecs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func canonicalSource(s Source, home string) (Source, error) {
	spec, ok := sourceSpecs[s.Agent]
	if !ok {
		return s, errors.New("unknown agent")
	}
	if strings.HasPrefix(s.Path, "~/") || strings.HasPrefix(s.Path, "~\\") {
		s.Path = filepath.Join(home, s.Path[2:])
	}
	if !filepath.IsAbs(s.Path) || strings.TrimSpace(s.Path) != s.Path || strings.ContainsAny(s.Path, ",\r\n\x00") {
		return s, errors.New("root must be absolute, without commas or surrounding whitespace")
	}
	p, err := filepath.EvalSymlinks(s.Path)
	if err != nil {
		return s, err
	}
	s.Path = filepath.Clean(p)
	if s.Agent == "claude" && filepath.Base(s.Path) == "projects" {
		s.Path = filepath.Dir(s.Path)
	}
	info, err := os.Stat(s.Path)
	if err != nil {
		return s, err
	}
	if spec.file {
		if !info.Mode().IsRegular() {
			return s, errors.New("expected regular telemetry file")
		}
	} else if !info.IsDir() {
		return s, errors.New("expected usage directory")
	}
	check := s.Path
	if s.Agent == "claude" {
		check = filepath.Join(check, "projects")
	}
	if s.Agent == "goose" {
		check = filepath.Join(check, "data", "sessions", "sessions.db")
	}
	f, err := os.Open(check)
	if err != nil {
		return s, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return s, err
	}
	if info.Mode().Perm()&0444 == 0 {
		return s, errors.New("selected path has no read permission")
	}
	if info.IsDir() {
		_, err = f.Readdirnames(1)
		if err != nil && err != io.EOF {
			return s, err
		}
	}
	return s, nil
}

// DiscoverSources only checks known directories and explicit environment paths.
// It does not open credentials, config files, or session contents.
func DiscoverSources(home string, getenv func(string) string) ([]Source, []string) {
	var sources []Source
	var warnings []string
	seen := map[Source]bool{}
	add := func(agent, path string, explicit bool) {
		s, err := canonicalSource(Source{agent, path}, home)
		if err != nil {
			if explicit {
				warnings = append(warnings, fmt.Sprintf("Unavailable %s path %q: %v", agent, path, err))
			}
			return
		}
		if !seen[s] {
			seen[s] = true
			sources = append(sources, s)
		}
	}
	for _, name := range sourceNames() {
		spec := sourceSpecs[name]
		for _, path := range spec.defaults {
			add(name, filepath.Join(home, filepath.FromSlash(path)), false)
		}
		raw := getenv(spec.env)
		if raw != "" {
			paths := []string{raw}
			if !spec.single {
				paths = strings.Split(raw, ",")
			}
			for _, p := range paths {
				add(name, strings.TrimSpace(p), true)
			}
		}
	}
	if p := getenv("PI_CODING_AGENT_DIR"); p != "" {
		add("pi", filepath.Join(p, "sessions"), true)
	}
	if p := getenv("XDG_CONFIG_HOME"); p != "" {
		add("claude", filepath.Join(p, "claude"), false)
		add("pi", filepath.Join(p, "pi-t3", "agent", "sessions"), false)
	}
	if p := getenv("XDG_DATA_HOME"); p != "" {
		add("opencode", filepath.Join(p, "opencode"), false)
	}
	for _, p := range []string{".local/share/goose/sessions/sessions.db", "Library/Application Support/goose/sessions/sessions.db", ".local/share/Block/goose/sessions/sessions.db"} {
		if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(p))); err == nil {
			warnings = append(warnings, "Goose database found at "+filepath.Join(home, filepath.FromSlash(p))+"; pinned collector cannot isolate this default location. Not selected. Only GOOSE_PATH_ROOT layouts are supported.")
		}
	}
	configPaths := []string{filepath.Join(home, ".config", "claude", "ccusage.json"), filepath.Join(home, ".claude", "ccusage.json")}
	if cwd, err := os.Getwd(); err == nil {
		configPaths = append(configPaths, filepath.Join(cwd, ".ccusage", "ccusage.json"))
	}
	if raw := getenv("CLAUDE_CONFIG_DIR"); raw != "" {
		for _, p := range strings.Split(raw, ",") {
			configPaths = append(configPaths, filepath.Join(strings.TrimSpace(p), "ccusage.json"))
		}
	}
	configured, notes := DiscoverConfigSources(home, configPaths)
	warnings = append(warnings, notes...)
	for _, s := range configured {
		if !seen[s] {
			sources = append(sources, s)
			seen[s] = true
		}
	}
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].Agent != sources[j].Agent {
			return sources[i].Agent < sources[j].Agent
		}
		return sources[i].Path < sources[j].Path
	})
	return sources, warnings
}

// DiscoverConfigSources imports only documented Pi/OpenClaw root fields from
// ccusage JSON, not agent credential/config/session files. Never execute config.
func DiscoverConfigSources(home string, paths []string) ([]Source, []string) {
	var sources []Source
	var warnings []string
	seen := map[Source]bool{}
	for _, path := range paths {
		info, statErr := os.Stat(path)
		if statErr != nil {
			continue
		}
		if !info.Mode().IsRegular() || info.Size() > 1024*1024 {
			warnings = append(warnings, "Not a small regular ccusage config: "+path)
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		raw, err := io.ReadAll(io.LimitReader(f, 1024*1024+1))
		f.Close()
		var config map[string]any
		if err != nil || len(raw) > 1024*1024 || json.Unmarshal(raw, &config) != nil {
			warnings = append(warnings, "Could not read ccusage path settings: "+path)
			continue
		}
		object := func(v any) map[string]any { m, _ := v.(map[string]any); return m }
		read := func(v any) {
			m := object(v)
			for key, agent := range map[string]string{"piPath": "pi", "openClawPath": "openclaw"} {
				if raw, ok := m[key].(string); ok {
					for _, p := range strings.Split(raw, ",") {
						s, e := canonicalSource(Source{agent, strings.TrimSpace(p)}, home)
						if e != nil {
							warnings = append(warnings, "Unavailable "+agent+" path in "+path)
							continue
						}
						if !seen[s] {
							seen[s] = true
							sources = append(sources, s)
						}
					}
				}
			}
		}
		read(config["defaults"])
		read(object(config["commands"])["daily"])
		for _, agent := range []string{"pi", "openclaw"} {
			section := object(config[agent])
			read(section["defaults"])
			read(object(section["commands"])["daily"])
			read(object(config["commands"])[agent+":daily"])
			read(object(config["commands"])[agent+" daily"])
		}
		if stores, ok := object(config["pi"])["stores"].([]any); ok && len(stores) > 0 {
			warnings = append(warnings, "Named Pi stores in "+path+" are not imported: add roots manually as pi (custom agent aliases are not preserved).")
		}
	}
	return sources, warnings
}

func ValidateSources(sources []Source) error {
	if len(sources) == 0 {
		return errors.New("no approved sources; run device-sync sources (collection is disabled)")
	}
	seen := map[Source]bool{}
	for _, s := range sources {
		actual, err := canonicalSource(s, "")
		if err != nil {
			return fmt.Errorf("selected %s root unavailable: %w", s.Agent, err)
		}
		if actual != s {
			return errors.New("selected root changed or is not canonical; reconfigure sources")
		}
		if seen[s] {
			return errors.New("duplicate selected root")
		}
		seen[s] = true
	}
	for i, a := range sources {
		for _, b := range sources[i+1:] {
			if a.Agent == b.Agent {
				rel, _ := filepath.Rel(a.Path, b.Path)
				reverse, _ := filepath.Rel(b.Path, a.Path)
				if inside(rel) || inside(reverse) {
					return errors.New("overlapping roots for one agent; select only the outer root")
				}
			}
		}
	}
	return nil
}
func inside(rel string) bool {
	return rel != "" && rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// SelectSources is shared by all installers. Each explicit setup starts with
// every discovered item checked; scheduled runs never call discovery.
func SelectSources(reader *bufio.Reader, out io.Writer, home string, candidates []Source) ([]Source, error) {
	chosen := make([]bool, len(candidates))
	for i := range chosen {
		chosen[i] = true
	}
	fmt.Fprintln(out, "Approve whole usage roots, including future files beneath them. No uploaded history is deleted by deselection.")
	fmt.Fprintln(out, "Manual agents:", strings.Join(sourceNames(), ", "))
	for {
		for i, s := range candidates {
			mark := " "
			if chosen[i] {
				mark = "x"
			}
			fmt.Fprintf(out, "%d. [%s] %s  %s\n", i+1, mark, s.Agent, s.Path)
		}
		fmt.Fprint(out, "Toggle numbers (e.g. 1 3), add <agent> <absolute path>, or Enter to accept all checked: ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "add ") {
			fields := strings.SplitN(strings.TrimSpace(strings.TrimPrefix(line, "add ")), " ", 2)
			if len(fields) != 2 {
				fmt.Fprintln(out, "Use add <agent> <absolute path>")
				continue
			}
			s, e := canonicalSource(Source{fields[0], strings.TrimSpace(fields[1])}, home)
			if e != nil {
				fmt.Fprintln(out, e)
				continue
			}
			duplicate := false
			for _, c := range candidates {
				if c == s {
					duplicate = true
				}
			}
			if !duplicate {
				candidates = append(candidates, s)
				chosen = append(chosen, true)
			}
			continue
		}
		if line != "" {
			valid := true
			var indexes []int
			for _, field := range strings.Fields(line) {
				n, e := strconv.Atoi(field)
				if e != nil || n < 1 || n > len(chosen) {
					valid = false
					break
				}
				indexes = append(indexes, n-1)
			}
			if !valid {
				fmt.Fprintln(out, "Enter valid numbers from the list.")
				continue
			}
			for _, i := range indexes {
				chosen[i] = !chosen[i]
			}
			continue
		}
		selected := []Source{}
		for i, s := range candidates {
			if chosen[i] {
				selected = append(selected, s)
			}
		}
		conflict := false
		for i, excluded := range candidates {
			if chosen[i] {
				continue
			}
			for _, allowed := range selected {
				parent := allowed.Path
				covered := allowed.Agent == excluded.Agent
				if allowed.Agent == "copilot" && excluded.Agent == "copilot-otel" {
					parent = filepath.Join(parent, "otel")
					covered = true
				}
				if covered {
					rel, _ := filepath.Rel(parent, excluded.Path)
					if inside(rel) {
						conflict = true
					}
				}
			}
		}
		if conflict {
			fmt.Fprintln(out, "An unchecked root is inside a checked root. Subtree exclusions are not supported; deselect the outer root instead.")
			continue
		}
		if len(selected) > 0 {
			if err := ValidateSources(selected); err != nil {
				fmt.Fprintln(out, err)
				continue
			}
		}
		fmt.Fprintln(out, "Selected roots:")
		if len(selected) == 0 {
			fmt.Fprintln(out, "  (none — collection will be disabled)")
		}
		for _, s := range selected {
			fmt.Fprintf(out, "  [x] %s  %s\n", s.Agent, s.Path)
		}
		fmt.Fprint(out, "Save this selection? [Y/n]: ")
		answer, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer == "" || answer == "y" || answer == "yes" {
			return selected, nil
		}
	}
}

// collectorEnvironment deliberately does not inherit arbitrary environment or
// config. Every adapter receives either approved roots or an empty private root.
func collectorEnvironment(sources []Source, empty string) []string {
	values := map[string][]string{}
	for _, s := range sources {
		key := sourceSpecs[s.Agent].env
		values[key] = append(values[key], s.Path)
	}
	env := []string{"HOME=" + empty, "USERPROFILE=" + empty, "XDG_CONFIG_HOME=" + empty, "XDG_DATA_HOME=" + empty, "XDG_CACHE_HOME=" + empty, "TMP=" + empty, "TEMP=" + empty, "TMPDIR=" + empty, "NO_COLOR=1"}
	if root := os.Getenv("SystemRoot"); root != "" {
		env = append(env, "SystemRoot="+root)
	}
	for _, name := range sourceNames() {
		spec := sourceSpecs[name]
		value := empty
		if len(values[spec.env]) > 0 {
			value = strings.Join(values[spec.env], ",")
		}
		env = append(env, spec.env+"="+value)
	}
	return env
}
