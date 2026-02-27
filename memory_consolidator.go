package agentsdk

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// ──────────────────────────────────────────────
// Memory Consolidator — Mem0-style fact-level ADD/UPDATE/DELETE/NOOP
// ──────────────────────────────────────────────

// MemoryAction represents the type of operation on a memory fact.
type MemoryAction string

const (
	MemoryActionAdd    MemoryAction = "ADD"
	MemoryActionUpdate MemoryAction = "UPDATE"
	MemoryActionDelete MemoryAction = "DELETE"
	MemoryActionNoop   MemoryAction = "NOOP"
)

// MemoryOperation represents a single fact-level memory operation
// classified by an LLM during the consolidation phase.
type MemoryOperation struct {
	Action MemoryAction `json:"action"`
	Key    string       `json:"key"`
	Value  interface{}  `json:"value,omitempty"`
	Reason string       `json:"reason,omitempty"`
}

// MemoryConsolidator applies a set of fact-level operations to existing memory.
type MemoryConsolidator interface {
	Consolidate(ops []MemoryOperation, existing map[string]interface{}) (map[string]interface{}, error)
}

// DefaultMemoryConsolidator applies operations to a nested map using dot-path keys.
type DefaultMemoryConsolidator struct{}

func NewDefaultMemoryConsolidator() *DefaultMemoryConsolidator {
	return &DefaultMemoryConsolidator{}
}

func (c *DefaultMemoryConsolidator) Consolidate(ops []MemoryOperation, existing map[string]interface{}) (map[string]interface{}, error) {
	result := copyMap(existing)
	for _, op := range ops {
		switch op.Action {
		case MemoryActionAdd, MemoryActionUpdate:
			if op.Key != "" && op.Value != nil {
				setNestedValue(result, op.Key, op.Value)
			}
		case MemoryActionDelete:
			if op.Key != "" {
				deleteNestedValue(result, op.Key)
			}
		case MemoryActionNoop:
			// intentionally empty
		}
	}
	return result, nil
}

func setNestedValue(m map[string]interface{}, path string, value interface{}) {
	parts := strings.Split(path, ".")
	current := m
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return
		}
		next, ok := current[part].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			current[part] = next
		}
		current = next
	}
}

func deleteNestedValue(m map[string]interface{}, path string) {
	parts := strings.Split(path, ".")
	current := m
	for i, part := range parts {
		if i == len(parts)-1 {
			delete(current, part)
			return
		}
		next, ok := current[part].(map[string]interface{})
		if !ok {
			return
		}
		current = next
	}
}

// ──────────────────────────────────────────────
// ConsolidatingExtractor — LLM-based extraction + consolidation
// ──────────────────────────────────────────────

// ConsolidationExtractionPrompt asks the LLM for operation-level output.
const ConsolidationExtractionPrompt = `你是一个记忆管理助手。请分析以下对话，对比用户已有档案，逐条判断应执行的操作。

规则：
1. 只处理用户自己说的信息，不要把 AI 助手的信息当作用户的
2. 不要推测或编造，只处理明确提到的信息
3. 对每条事实分类为以下操作之一：
   - ADD: 全新的事实，档案中不存在
   - UPDATE: 已有事实需要更新（用户明确纠正或新信息替代旧信息）
   - DELETE: 用户明确否认或要求删除的旧事实
   - NOOP: 无需变更
4. key 使用点分路径，如 "basic_info.age"、"interests"、"life_context.goals"

当前已有的用户档案：
%s

待分析的对话：
%s

请输出 JSON 数组（只返回 ADD/UPDATE/DELETE 操作，跳过 NOOP）：
[
  {"action": "ADD", "key": "basic_info.age", "value": 25, "reason": "用户说自己25岁"},
  {"action": "UPDATE", "key": "basic_info.location", "value": "北京", "reason": "用户说搬到了北京"},
  {"action": "DELETE", "key": "life_context.concerns.0", "value": null, "reason": "用户说这个问题已经解决了"}
]

只返回 JSON 数组，不要其他文字：`

// ConsolidatingExtractor combines LLM extraction with operation-level
// consolidation, replacing flat DeepMerge with precise fact-level ops.
// Implements MemoryExtractorInterface for drop-in compatibility.
type ConsolidatingExtractor struct {
	LLMFn          MemoryExtractorFunc
	PromptTemplate string
	Consolidator   MemoryConsolidator
}

// NewConsolidatingExtractor creates an extractor with operation-level consolidation.
func NewConsolidatingExtractor(llmFn MemoryExtractorFunc, consolidator MemoryConsolidator) *ConsolidatingExtractor {
	if consolidator == nil {
		consolidator = NewDefaultMemoryConsolidator()
	}
	return &ConsolidatingExtractor{
		LLMFn:          llmFn,
		PromptTemplate: ConsolidationExtractionPrompt,
		Consolidator:   consolidator,
	}
}

// Extract implements MemoryExtractorInterface.
// Falls back to DeepMerge-compatible flat JSON if operation parsing fails.
func (e *ConsolidatingExtractor) Extract(conversations []map[string]string, currentMemory map[string]interface{}) (map[string]interface{}, error) {
	if len(conversations) == 0 {
		return map[string]interface{}{}, nil
	}

	convText := formatConversations(conversations)
	memJSON, _ := json.MarshalIndent(currentMemory, "", "  ")
	prompt := fmt.Sprintf(e.PromptTemplate, string(memJSON), convText)

	response, err := e.LLMFn(prompt)
	if err != nil {
		log.Printf("[ConsolidatingExtractor] LLM call failed: %v", err)
		return map[string]interface{}{}, err
	}

	ops, parseErr := ParseOperationsResponse(response)
	if parseErr != nil || len(ops) == 0 {
		flat := ParseJSONResponse(response)
		if len(flat) > 0 {
			log.Printf("[ConsolidatingExtractor] Falling back to DeepMerge")
			return flat, nil
		}
		return map[string]interface{}{}, parseErr
	}

	result, err := e.Consolidator.Consolidate(ops, currentMemory)
	if err != nil {
		log.Printf("[ConsolidatingExtractor] Consolidation failed: %v", err)
		return map[string]interface{}{}, err
	}
	return result, nil
}

// ParseOperationsResponse parses an LLM response into MemoryOperation slice.
func ParseOperationsResponse(text string) ([]MemoryOperation, error) {
	text = strings.TrimSpace(text)

	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		var cleaned []string
		for _, l := range lines {
			if !strings.HasPrefix(strings.TrimSpace(l), "```") {
				cleaned = append(cleaned, l)
			}
		}
		text = strings.TrimSpace(strings.Join(cleaned, "\n"))
	}

	var ops []MemoryOperation
	if json.Unmarshal([]byte(text), &ops) == nil && len(ops) > 0 {
		return filterValidOps(ops), nil
	}

	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	if start >= 0 && end > start {
		if json.Unmarshal([]byte(text[start:end+1]), &ops) == nil && len(ops) > 0 {
			return filterValidOps(ops), nil
		}
	}

	return nil, fmt.Errorf("failed to parse operations from response")
}

func filterValidOps(ops []MemoryOperation) []MemoryOperation {
	valid := make([]MemoryOperation, 0, len(ops))
	for _, op := range ops {
		action := MemoryAction(strings.ToUpper(string(op.Action)))
		switch action {
		case MemoryActionAdd, MemoryActionUpdate, MemoryActionDelete, MemoryActionNoop:
			op.Action = action
			if op.Key != "" {
				valid = append(valid, op)
			}
		}
	}
	return valid
}
