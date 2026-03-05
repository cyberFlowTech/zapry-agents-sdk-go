package zapry

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ProfileSource 是扩展 setMyProfile 的主权快照结构。
type ProfileSource struct {
	Version    string               `json:"version"`
	Source     string               `json:"source,omitempty"`
	AgentKey   string               `json:"agentKey,omitempty"`
	SnapshotID string               `json:"snapshotId"`
	SoulMD     string               `json:"soulMd"`
	Skills     []ProfileSourceSkill `json:"skills"`
}

// ProfileSourceSkill 对应单个 skills/*/SKILL.md 文件快照。
type ProfileSourceSkill struct {
	SkillKey     string `json:"skillKey"`
	SkillVersion string `json:"skillVersion,omitempty"`
	Source       string `json:"source,omitempty"`
	Path         string `json:"path"`
	Content      string `json:"content"`
	SHA256       string `json:"sha256"`
	Bytes        int    `json:"bytes"`
}

// DerivedProfile 是 setMyProfile 扩展响应中的扁平画像结构。
type DerivedProfile struct {
	Name             string   `json:"name,omitempty"`
	Role             string   `json:"role,omitempty"`
	Vibe             string   `json:"vibe,omitempty"`
	Emoji            string   `json:"emoji,omitempty"`
	Avatar           string   `json:"avatar,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	Summary          string   `json:"summary,omitempty"`
	Skills           []string `json:"skills,omitempty"`
	RouteTags        []string `json:"routeTags,omitempty"`
	DerivedVersion   string   `json:"derivedVersion,omitempty"`
	DerivedAt        string   `json:"derivedAt,omitempty"`
	OverrideRevision int      `json:"overrideRevision,omitempty"`
}

var (
	skillKeyPattern       = regexp.MustCompile(`(?m)^\s*skillKey:\s*["']?([^"'\n]+)["']?\s*$`)
	skillVersionPattern   = regexp.MustCompile(`(?m)^\s*skillVersion:\s*["']?([^"'\n]+)["']?\s*$`)
	genericVersionPattern = regexp.MustCompile(`(?m)^\s*version:\s*["']?([^"'\n]+)["']?\s*$`)
	sourcePattern         = regexp.MustCompile(`(?m)^\s*source:\s*["']?([^"'\n]+)["']?\s*$`)
)

// BuildProfileSourceFromDir 从 demo 根目录构建 profileSource：
// - 读取 SOUL.md
// - 递归读取 skills/*/SKILL.md
// - 计算每个 SKILL 的 raw-bytes sha256
// - 按规范计算 snapshotId
func BuildProfileSourceFromDir(baseDir, agentKey string) (*ProfileSource, error) {
	baseDir = filepath.Clean(baseDir)
	soulPath := filepath.Join(baseDir, "SOUL.md")
	soulBytes, err := os.ReadFile(soulPath)
	if err != nil {
		return nil, fmt.Errorf("read SOUL.md failed: %w", err)
	}

	skillPaths, err := collectSkillMarkdownFiles(filepath.Join(baseDir, "skills"))
	if err != nil {
		return nil, err
	}
	if len(skillPaths) == 0 {
		return nil, fmt.Errorf("%w under %s", ErrNoSkillMarkdownFound, filepath.Join(baseDir, "skills"))
	}

	skills := make([]ProfileSourceSkill, 0, len(skillPaths))
	for _, absolutePath := range skillPaths {
		raw, readErr := os.ReadFile(absolutePath)
		if readErr != nil {
			return nil, fmt.Errorf("read skill file failed: %w", readErr)
		}
		content := string(raw)
		frontmatter := extractFrontmatter(content)

		relPath, relErr := filepath.Rel(baseDir, absolutePath)
		if relErr != nil {
			return nil, fmt.Errorf("resolve relative skill path failed: %w", relErr)
		}
		relPath = filepath.ToSlash(relPath)

		skillKey := extractSkillKey(frontmatter, relPath)
		skillVersion := extractSkillVersion(frontmatter)
		skillSource := extractSkillSource(frontmatter)

		skills = append(skills, ProfileSourceSkill{
			SkillKey:     skillKey,
			SkillVersion: skillVersion,
			Source:       skillSource,
			Path:         relPath,
			Content:      content,
			SHA256:       sha256Hex(raw),
			Bytes:        len(raw),
		})
	}

	if agentKey = strings.TrimSpace(agentKey); agentKey == "" {
		agentKey = filepath.Base(baseDir)
	}

	snapshotID, err := ComputeSnapshotID(string(soulBytes), skills)
	if err != nil {
		return nil, err
	}

	return &ProfileSource{
		Version:    "v1",
		Source:     "code",
		AgentKey:   agentKey,
		SnapshotID: snapshotID,
		SoulMD:     string(soulBytes),
		Skills:     skills,
	}, nil
}

// ComputeSnapshotID 计算 profileSource.snapshotId。
func ComputeSnapshotID(soulMD string, skills []ProfileSourceSkill) (string, error) {
	if len(skills) == 0 {
		return "", fmt.Errorf("skills cannot be empty")
	}

	normalizedSoul := normalizeSoulMarkdown(soulMD)
	normalizedIndex, err := normalizeSkillsIndex(skills)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(normalizedSoul + "\n" + normalizedIndex))
	return hex.EncodeToString(sum[:]), nil
}

// SkillKeysFromProfileSource 返回去重后的 skill key 列表。
func SkillKeysFromProfileSource(source *ProfileSource) []string {
	if source == nil || len(source.Skills) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(source.Skills))
	out := make([]string, 0, len(source.Skills))
	for _, skill := range source.Skills {
		key := strings.TrimSpace(skill.SkillKey)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

// BuildRuntimeSystemPromptFromSource 生成运行时 system prompt：
// - SOUL.md 全文（保留）
// - 每个 SKILL.md 的 Markdown 正文（去 frontmatter）
func BuildRuntimeSystemPromptFromSource(source *ProfileSource) string {
	if source == nil {
		return ""
	}
	parts := make([]string, 0, len(source.Skills)+1)
	soul := strings.TrimSpace(source.SoulMD)
	if soul != "" {
		parts = append(parts, soul)
	}

	for _, skill := range source.Skills {
		body := strings.TrimSpace(extractSkillMarkdownBody(skill.Content))
		if body == "" {
			continue
		}
		parts = append(parts, body)
	}
	return strings.Join(parts, "\n\n")
}

func collectSkillMarkdownFiles(skillsRoot string) ([]string, error) {
	entries := make([]string, 0, 8)
	if _, err := os.Stat(skillsRoot); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrSkillsDirectoryNotFound, skillsRoot)
		}
		return nil, fmt.Errorf("stat skills directory failed: %w", err)
	}
	err := filepath.WalkDir(skillsRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), "SKILL.md") {
			entries = append(entries, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk skills directory failed: %w", err)
	}
	sort.Strings(entries)
	return entries, nil
}

func extractFrontmatter(content string) string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return ""
	}
	lines := strings.Split(normalized, "\n")
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n")
		}
	}
	return ""
}

func extractSkillMarkdownBody(content string) string {
	normalized := strings.TrimPrefix(content, "\uFEFF")
	normalized = strings.ReplaceAll(normalized, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return strings.TrimSpace(normalized)
	}
	lines := strings.Split(normalized, "\n")
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
		}
	}
	return strings.TrimSpace(normalized)
}

func extractSkillKey(frontmatter, fallbackPath string) string {
	if key := firstRegexMatch(skillKeyPattern, frontmatter); key != "" {
		return key
	}
	// fallback: skills/<skillKey>/SKILL.md
	parts := strings.Split(filepath.ToSlash(fallbackPath), "/")
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[len(parts)-2])
	}
	return "unknown-skill"
}

func extractSkillVersion(frontmatter string) string {
	if v := firstRegexMatch(skillVersionPattern, frontmatter); v != "" {
		return v
	}
	if v := firstRegexMatch(genericVersionPattern, frontmatter); v != "" {
		return v
	}
	return "1.0.0"
}

func extractSkillSource(frontmatter string) string {
	if v := firstRegexMatch(sourcePattern, frontmatter); v != "" {
		return v
	}
	return "local"
}

func firstRegexMatch(re *regexp.Regexp, text string) string {
	if re == nil {
		return ""
	}
	matches := re.FindStringSubmatch(text)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func normalizeSoulMarkdown(content string) string {
	content = strings.TrimPrefix(content, "\uFEFF")
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	lines := strings.Split(content, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	content = strings.Join(lines, "\n")
	return strings.TrimRight(content, " \t\n")
}

func normalizeSkillsIndex(skills []ProfileSourceSkill) (string, error) {
	if len(skills) == 0 {
		return "", fmt.Errorf("skills cannot be empty")
	}
	copied := append([]ProfileSourceSkill(nil), skills...)
	sort.Slice(copied, func(i, j int) bool {
		if copied[i].SkillKey == copied[j].SkillKey {
			return copied[i].SkillVersion < copied[j].SkillVersion
		}
		return copied[i].SkillKey < copied[j].SkillKey
	})

	seenKeys := make(map[string]struct{}, len(copied))
	lines := make([]string, 0, len(copied))
	for _, skill := range copied {
		key := strings.TrimSpace(skill.SkillKey)
		if key == "" {
			return "", fmt.Errorf("skillKey cannot be empty")
		}
		if _, ok := seenKeys[key]; ok {
			return "", fmt.Errorf("duplicate_skill_key: %s", key)
		}
		seenKeys[key] = struct{}{}
		version := strings.TrimSpace(skill.SkillVersion)
		if version == "" {
			version = "1.0.0"
		}
		sha := strings.TrimSpace(skill.SHA256)
		if sha == "" {
			return "", fmt.Errorf("skill sha256 cannot be empty: %s", key)
		}
		lines = append(lines, key+"|"+version+"|"+sha)
	}
	return strings.Join(lines, "\n"), nil
}

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
