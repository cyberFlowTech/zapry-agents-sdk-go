package agentsdk

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// MemoryExtractorFunc is called by LLMMemoryExtractor to invoke the LLM.
// Signature: func(prompt string) (string, error)
type MemoryExtractorFunc func(prompt string) (string, error)

// MemoryExtractorInterface defines the interface for memory extraction.
type MemoryExtractorInterface interface {
	Extract(conversations []map[string]string, currentMemory map[string]interface{}) (map[string]interface{}, error)
}

// LLMMemoryExtractor uses an LLM to extract structured info from conversations.
type LLMMemoryExtractor struct {
	LLMFn          MemoryExtractorFunc
	PromptTemplate string
}

// DefaultExtractionPrompt is the built-in Chinese extraction prompt.
const DefaultExtractionPrompt = `你是一个记忆提取助手。请从以下对话中提取关于用户的结构化信息。

规则：
1. 只提取用户自己说的信息，不要把 AI 助手的信息当作用户的
2. 不要推测或编造，只提取明确提到的信息
3. 如果没有新信息，对应字段留空或返回空对象
4. 输出严格的 JSON 格式

当前已有的用户档案：
%s

待提取的对话：
%s

请提取以下字段（只返回有新信息的字段）：
{
  "basic_info": {"age": null, "gender": null, "location": null, "occupation": null},
  "personality": {"traits": [], "values": []},
  "life_context": {"concerns": [], "goals": [], "recent_events": []},
  "interests": [],
  "summary": ""
}

只返回 JSON，不要其他文字：`

// NewLLMMemoryExtractor creates an extractor with the given LLM function.
func NewLLMMemoryExtractor(llmFn MemoryExtractorFunc, promptTemplate string) *LLMMemoryExtractor {
	if promptTemplate == "" {
		promptTemplate = DefaultExtractionPrompt
	}
	return &LLMMemoryExtractor{LLMFn: llmFn, PromptTemplate: promptTemplate}
}

// Extract calls the LLM to extract structured memory from conversations.
func (e *LLMMemoryExtractor) Extract(conversations []map[string]string, currentMemory map[string]interface{}) (map[string]interface{}, error) {
	if len(conversations) == 0 {
		return map[string]interface{}{}, nil
	}

	convText := formatConversations(conversations)
	memJSON, _ := json.MarshalIndent(currentMemory, "", "  ")

	prompt := fmt.Sprintf(e.PromptTemplate, string(memJSON), convText)

	response, err := e.LLMFn(prompt)
	if err != nil {
		log.Printf("[LLMMemoryExtractor] LLM call failed: %v", err)
		return map[string]interface{}{}, err
	}

	return ParseJSONResponse(response), nil
}

func formatConversations(conversations []map[string]string) string {
	var lines []string
	for _, msg := range conversations {
		role := msg["role"]
		content := msg["content"]
		label := "用户"
		if role != "user" {
			label = "助手"
		}
		lines = append(lines, fmt.Sprintf("%s: %s", label, content))
	}
	return strings.Join(lines, "\n")
}

// ParseJSONResponse extracts a JSON object from LLM response text.
func ParseJSONResponse(text string) map[string]interface{} {
	text = strings.TrimSpace(text)

	// Remove code block markers
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

	// Try direct parse
	var result map[string]interface{}
	if json.Unmarshal([]byte(text), &result) == nil {
		return result
	}

	// Try to find JSON object
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		if json.Unmarshal([]byte(text[start:end+1]), &result) == nil {
			return result
		}
	}

	return map[string]interface{}{}
}
