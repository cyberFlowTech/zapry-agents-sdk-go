package agentsdk

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/cyberFlowTech/zapry-agents-sdk-go/channel/zapry"
)

//go:embed skills/*/SKILL.md
var builtinSkillsFS embed.FS

// BuiltinSkill 描述 SDK 内置技能模板（参考 OpenClaw 的 SKILL.md 组织方式）。
type BuiltinSkill struct {
	Key          string   `json:"key"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	SkillVersion string   `json:"skillVersion"`
	Source       string   `json:"source"`
	Tags         []string `json:"tags,omitempty"`
	Content      string   `json:"content"`
}

var (
	builtinSkillCatalogOnce sync.Once
	builtinSkillCatalog     map[string]BuiltinSkill
	builtinSkillCatalogErr  error
)

// ListBuiltinSkills 返回按 key 排序的内置技能清单。
func ListBuiltinSkills() []BuiltinSkill {
	catalog, err := loadBuiltinSkillCatalog()
	if err != nil {
		return nil
	}
	keys := make([]string, 0, len(catalog))
	for key := range catalog {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]BuiltinSkill, 0, len(keys))
	for _, key := range keys {
		out = append(out, cloneBuiltinSkill(catalog[key]))
	}
	return out
}

// GetBuiltinSkill 读取指定 key 的内置技能。
func GetBuiltinSkill(key string) (BuiltinSkill, bool) {
	catalog, err := loadBuiltinSkillCatalog()
	if err != nil {
		return BuiltinSkill{}, false
	}
	k := strings.ToLower(strings.TrimSpace(key))
	skill, ok := catalog[k]
	if !ok {
		return BuiltinSkill{}, false
	}
	return cloneBuiltinSkill(skill), true
}

// BuildProfileSourceFromDirWithBuiltinSkills 在本地 SOUL + skills 基础上，额外注入内置技能。
// 规则：
//   - 内置技能 key 可重复输入，内部自动去重。
//   - 若本地 skillKey 与内置 skillKey 冲突，优先保留本地技能（便于项目自定义覆盖）。
func BuildProfileSourceFromDirWithBuiltinSkills(baseDir, agentKey string, builtinSkillKeys ...string) (*ProfileSource, error) {
	baseDir = filepath.Clean(baseDir)

	// 先尝试读取本地 source；没有 skills 目录时允许继续（由内置技能补齐）。
	localSource, localErr := zapry.BuildProfileSourceFromDir(baseDir, agentKey)
	if localErr == nil {
		localSkills := append([]ProfileSourceSkill(nil), localSource.Skills...)
		builtinSkills, err := builtinProfileSkillsFromKeys(builtinSkillKeys)
		if err != nil {
			return nil, err
		}
		merged := mergeBuiltinSkills(localSkills, builtinSkills)
		if len(merged) == len(localSkills) {
			// 内置技能为空或全部被本地覆盖，直接返回原始 source，避免无意义变更。
			return localSource, nil
		}
		snapshotID, err := zapry.ComputeSnapshotID(localSource.SoulMD, merged)
		if err != nil {
			return nil, err
		}
		localSource.Skills = merged
		localSource.SnapshotID = snapshotID
		return localSource, nil
	}

	// 本地失败时，允许“仅 SOUL + 内置技能”模式。
	if !canFallbackToBuiltinSkills(localErr) {
		return nil, localErr
	}
	soulPath := filepath.Join(baseDir, "SOUL.md")
	soulBytes, err := os.ReadFile(soulPath)
	if err != nil {
		return nil, fmt.Errorf("build profile source failed (%v) and read SOUL.md failed: %w", localErr, err)
	}
	builtinSkills, err := builtinProfileSkillsFromKeys(builtinSkillKeys)
	if err != nil {
		return nil, err
	}
	if len(builtinSkills) == 0 {
		return nil, fmt.Errorf("build profile source failed: %v", localErr)
	}

	if agentKey = strings.TrimSpace(agentKey); agentKey == "" {
		agentKey = filepath.Base(baseDir)
	}
	snapshotID, err := zapry.ComputeSnapshotID(string(soulBytes), builtinSkills)
	if err != nil {
		return nil, err
	}
	return &ProfileSource{
		Version:    "v1",
		Source:     "code",
		AgentKey:   agentKey,
		SnapshotID: snapshotID,
		SoulMD:     string(soulBytes),
		Skills:     builtinSkills,
	}, nil
}

func loadBuiltinSkillCatalog() (map[string]BuiltinSkill, error) {
	builtinSkillCatalogOnce.Do(func() {
		files, err := fs.Glob(builtinSkillsFS, "skills/*/SKILL.md")
		if err != nil {
			builtinSkillCatalogErr = fmt.Errorf("glob builtin skills failed: %w", err)
			return
		}
		if len(files) == 0 {
			builtinSkillCatalogErr = fmt.Errorf("no builtin skill files found under skills/*/SKILL.md")
			return
		}
		sort.Strings(files)

		catalog := make(map[string]BuiltinSkill, len(files))
		for _, skillPath := range files {
			raw, readErr := fs.ReadFile(builtinSkillsFS, skillPath)
			if readErr != nil {
				builtinSkillCatalogErr = fmt.Errorf("read builtin skill %s failed: %w", skillPath, readErr)
				return
			}
			skill, parseErr := parseBuiltinSkillFile(skillPath, string(raw))
			if parseErr != nil {
				builtinSkillCatalogErr = parseErr
				return
			}
			key := strings.ToLower(strings.TrimSpace(skill.Key))
			if key == "" {
				builtinSkillCatalogErr = fmt.Errorf("builtin skill key is empty: %s", skillPath)
				return
			}
			if _, exists := catalog[key]; exists {
				builtinSkillCatalogErr = fmt.Errorf("duplicate builtin skill key: %s", key)
				return
			}
			catalog[key] = skill
		}
		builtinSkillCatalog = catalog
	})
	if builtinSkillCatalogErr != nil {
		return nil, builtinSkillCatalogErr
	}
	return builtinSkillCatalog, nil
}

func parseBuiltinSkillFile(skillPath, content string) (BuiltinSkill, error) {
	content = strings.TrimSpace(content)
	frontmatter := extractFrontmatter(content)
	meta := parseSimpleFrontmatter(frontmatter)

	key := strings.TrimSpace(meta["skillKey"])
	if key == "" {
		key = strings.TrimSpace(path.Base(path.Dir(skillPath)))
	}
	if key == "" {
		return BuiltinSkill{}, fmt.Errorf("skillKey is required: %s", skillPath)
	}

	name := strings.TrimSpace(meta["name"])
	if name == "" {
		name = key
	}
	description := strings.TrimSpace(meta["description"])
	if description == "" {
		description = "sdk builtin skill"
	}
	return BuiltinSkill{
		Key:          key,
		Name:         name,
		Description:  description,
		SkillVersion: defaultSkillVersion(meta["skillVersion"]),
		Source:       defaultSkillSource(meta["source"]),
		Tags:         parseSkillTags(meta["tags"]),
		Content:      content,
	}, nil
}

func parseSimpleFrontmatter(frontmatter string) map[string]string {
	out := make(map[string]string)
	if strings.TrimSpace(frontmatter) == "" {
		return out
	}
	lines := strings.Split(frontmatter, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"'`)
		if key != "" && val != "" {
			out[key] = val
		}
	}
	return out
}

func parseSkillTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	items := strings.Split(raw, ",")
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		tag := strings.TrimSpace(item)
		if tag == "" {
			continue
		}
		lower := strings.ToLower(tag)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		out = append(out, tag)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func builtinProfileSkillsFromKeys(keys []string) ([]ProfileSourceSkill, error) {
	orderedKeys := normalizeBuiltinSkillKeys(keys)
	if len(orderedKeys) == 0 {
		return nil, nil
	}

	out := make([]ProfileSourceSkill, 0, len(orderedKeys))
	for _, key := range orderedKeys {
		skill, ok := GetBuiltinSkill(key)
		if !ok {
			return nil, fmt.Errorf("unknown builtin skill: %s", key)
		}
		content := strings.TrimSpace(skill.Content)
		raw := []byte(content)
		out = append(out, ProfileSourceSkill{
			SkillKey:     skill.Key,
			SkillVersion: defaultSkillVersion(skill.SkillVersion),
			Source:       defaultSkillSource(skill.Source),
			Path:         fmt.Sprintf("skills/_builtin/%s/SKILL.md", skill.Key),
			Content:      content,
			SHA256:       sha256Hex(raw),
			Bytes:        len(raw),
		})
	}
	return out, nil
}

func mergeBuiltinSkills(localSkills, builtinSkills []ProfileSourceSkill) []ProfileSourceSkill {
	if len(builtinSkills) == 0 {
		return localSkills
	}
	localKeySet := make(map[string]struct{}, len(localSkills))
	for _, skill := range localSkills {
		key := strings.TrimSpace(skill.SkillKey)
		if key != "" {
			localKeySet[key] = struct{}{}
		}
	}

	merged := make([]ProfileSourceSkill, 0, len(localSkills)+len(builtinSkills))
	merged = append(merged, localSkills...)
	for _, skill := range builtinSkills {
		if _, exists := localKeySet[strings.TrimSpace(skill.SkillKey)]; exists {
			continue
		}
		merged = append(merged, skill)
	}
	return merged
}

func normalizeBuiltinSkillKeys(keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(keys))
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		k := strings.ToLower(strings.TrimSpace(key))
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

func cloneBuiltinSkill(in BuiltinSkill) BuiltinSkill {
	out := in
	if len(in.Tags) > 0 {
		out.Tags = append([]string(nil), in.Tags...)
	}
	return out
}

func defaultSkillVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "1.0.0"
	}
	return v
}

func defaultSkillSource(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "sdk-builtin"
	}
	return v
}

func canFallbackToBuiltinSkills(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "skills directory not found:") ||
		strings.Contains(msg, "no SKILL.md found under")
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

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

