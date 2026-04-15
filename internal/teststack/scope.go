package teststack

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	dockerNameMaxLen  = 63
	defaultScopeSpec  = "systemtest"
	defaultScopeRun   = "local"
	dockerSpecMax     = 6
	dockerWorktreeMax = 8
	dockerRunMax      = 6
	hashPrefixLen     = 12
)

type Scope struct {
	Spec     string
	Worktree string
	Mode     Mode
	Run      string
}

type ScopeInput struct {
	ProjectRoot string
	Spec        string
	Worktree    string
	Mode        Mode
	Run         string
}

type ScopeReport struct {
	Spec      string `json:"spec"`
	Worktree  string `json:"worktree"`
	Mode      string `json:"mode"`
	Run       string `json:"run"`
	Hash      string `json:"hash"`
	Canonical string `json:"canonical,omitempty"`
}

func ResolveScope(input ScopeInput) Scope {
	scope, err := scopeFromValues(input.ProjectRoot, Scope{
		Spec:     input.Spec,
		Worktree: input.Worktree,
		Mode:     input.Mode,
		Run:      input.Run,
	}, "", "", "", input.Mode)
	if err != nil {
		return Scope{
			Spec:     defaultScopeSpec,
			Worktree: defaultWorktree(input.ProjectRoot),
			Mode:     defaultMode(input.Mode),
			Run:      defaultScopeRun,
		}
	}
	return scope
}

func resolveScope(opts Options) (Scope, error) {
	scope, err := scopeFromValues(opts.ProjectRoot, opts.Scope, opts.ScopeSpec, opts.ScopeWorktree, opts.ScopeRun, opts.Mode)
	if err != nil {
		return Scope{}, err
	}
	if scope.Mode != ModePR && scope.Mode != ModeNightly {
		return Scope{}, fmt.Errorf("unsupported mode %q", scope.Mode)
	}
	return scope, nil
}

func scopeFromValues(projectRoot string, explicit Scope, specField, worktreeField, runField string, fallbackMode Mode) (Scope, error) {
	mode := explicit.Mode
	if mode == "" {
		mode = fallbackMode
	}
	mode = defaultMode(mode)

	scope := Scope{
		Spec:     firstNonEmpty(explicit.Spec, specField, defaultScopeSpec),
		Worktree: firstNonEmpty(explicit.Worktree, worktreeField, defaultWorktree(projectRoot)),
		Mode:     mode,
		Run:      firstNonEmpty(explicit.Run, runField, defaultScopeRun),
	}
	return scope.normalize(), nil
}

func defaultMode(mode Mode) Mode {
	if strings.TrimSpace(string(mode)) == "" {
		return ModePR
	}
	return Mode(strings.ToLower(strings.TrimSpace(string(mode))))
}

func defaultWorktree(projectRoot string) string {
	base := filepath.Base(filepath.Clean(strings.TrimSpace(projectRoot)))
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "default"
	}
	return strings.TrimSpace(base)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (s Scope) normalize() Scope {
	return Scope{
		Spec:     strings.TrimSpace(s.Spec),
		Worktree: strings.TrimSpace(s.Worktree),
		Mode:     defaultMode(s.Mode),
		Run:      strings.TrimSpace(s.Run),
	}
}

func (s Scope) Canonical() string {
	normalized := s.normalize()
	return fmt.Sprintf(
		"spec=%s|worktree=%s|mode=%s|run=%s",
		normalized.Spec,
		normalized.Worktree,
		normalized.Mode,
		normalized.Run,
	)
}

func (s Scope) Hash() string {
	sum := sha256.Sum256([]byte(s.Canonical()))
	return hex.EncodeToString(sum[:])[:hashPrefixLen]
}

func (s Scope) RuntimeReport() ScopeReport {
	normalized := s.normalize()
	return ScopeReport{
		Spec:      normalized.Spec,
		Worktree:  normalized.Worktree,
		Mode:      string(normalized.Mode),
		Run:       normalized.Run,
		Hash:      normalized.Hash(),
		Canonical: normalized.Canonical(),
	}
}

func (s Scope) Report(_ string) ScopeReport {
	return s.RuntimeReport()
}

func ScopeFromReport(report ScopeReport) Scope {
	return Scope{
		Spec:     strings.TrimSpace(report.Spec),
		Worktree: strings.TrimSpace(report.Worktree),
		Mode:     defaultMode(Mode(report.Mode)),
		Run:      strings.TrimSpace(report.Run),
	}.normalize()
}

func (s Scope) ArtifactRoot(repoRoot string) string {
	normalized := s.normalize()
	dirName := fmt.Sprintf(
		"%s-%s-%s-%s",
		dockerSafeToken(normalized.Spec, 16),
		dockerSafeToken(normalized.Worktree, 16),
		dockerModeToken(normalized.Mode),
		normalized.Hash(),
	)
	return filepath.Join(filepath.Clean(repoRoot), "artifacts", "system", dirName)
}

func (s Scope) EnvReportPath(repoRoot string) string {
	return filepath.Join(s.ArtifactRoot(repoRoot), "env-report.json")
}

func (s Scope) DockerName(kind string) string {
	names := makeResourceNames(s)
	switch strings.TrimSpace(kind) {
	case "net", "network":
		return names.Network
	case "postgres":
		return names.Postgres
	case "keycloak":
		return names.Keycloak
	case "mailpit":
		return names.Mailpit
	case "aws-sim":
		return names.AWSSim
	case "aws-sim-back", "aws-sim-backend":
		return names.AWSSimBackend
	case "app", "senda":
		return names.App
	default:
		return trimDockerName(names.Network + "-" + dockerSafeToken(kind, 12))
	}
}

func dockerSafeToken(input string, max int) string {
	var b strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(strings.TrimSpace(input)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastHyphen = false
		case !lastHyphen:
			b.WriteByte('-')
			lastHyphen = true
		}
	}

	token := strings.Trim(b.String(), "-")
	if token == "" {
		return "x"
	}
	if max > 0 && len(token) > max {
		token = strings.Trim(token[:max], "-")
		if token == "" {
			return "x"
		}
	}
	return token
}

func dockerModeToken(mode Mode) string {
	switch defaultMode(mode) {
	case ModeNightly:
		return "ntl"
	case ModePR:
		return "pr"
	default:
		return dockerSafeToken(string(mode), 4)
	}
}

func trimDockerName(name string) string {
	clean := strings.Trim(dockerSafeToken(name, 0), "-")
	if len(clean) <= dockerNameMaxLen {
		return clean
	}
	hash := shortHash(clean)
	prefix := strings.Trim(clean[:dockerNameMaxLen-len(hash)-1], "-")
	if prefix == "" {
		return hash
	}
	return prefix + "-" + hash
}

func shortHash(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])[:8]
}
