package agentsdk

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
)

// AutoSessionIDResolver extracts a stable session/user identifier from Update.
type AutoSessionIDResolver func(update Update) string

// AutoUserInputResolver extracts user input text from Update.
type AutoUserInputResolver func(update Update) string

// AutoPersonaSpecBuilder builds PersonaSpec from profileSource.
type AutoPersonaSpecBuilder func(source *ProfileSource) *PersonaSpec

// AutoConversationOptions controls SDK automatic conversation orchestration.
//
// Design goal:
// Developers provide only model + optional business tools; SDK handles
// memory, natural conversation enhancement, and persona state automatically.
type AutoConversationOptions struct {
	// Profile source (optional). If empty, uses agent.Config.ProfileSource.
	ProfileSource *ProfileSource

	// One of LLMFn / LLMFnCtx must be provided.
	LLMFn    LLMFunc
	LLMFnCtx LLMFuncWithContext

	// Optional runtime dependencies.
	ToolRegistry *ToolRegistry
	MemoryStore  MemoryStore

	Guardrails   *GuardrailManager
	Tracer       *AgentTracer
	LoopDetector *LoopDetector
	Capabilities *AgentCapabilities

	// Loop tuning.
	LoopMaxTurns int

	// Natural conversation tuning.
	NaturalConfig         *NaturalConversationConfig
	EnableContextCompress bool
	SummarizeFn           SummarizeFn

	// Memory extraction tuning.
	EnableMemoryExtraction bool
	// DisableMemoryExtraction has higher priority than EnableMemoryExtraction.
	DisableMemoryExtraction bool
	MemoryExtractor         MemoryExtractorInterface
	MemoryExtractorFn       MemoryExtractorFunc

	// Persona mapping from SOUL.md + skills.
	PersonaBuilder AutoPersonaSpecBuilder

	// Update parsing strategy.
	SessionIDResolver AutoSessionIDResolver
	UserInputResolver AutoUserInputResolver

	// Middleware behavior.
	AutoHandlePrivate bool
	// DisableAutoHandlePrivate has higher priority than AutoHandlePrivate.
	DisableAutoHandlePrivate bool
	EmptyReply               string
}

// DefaultAutoConversationOptions returns production-friendly defaults.
func DefaultAutoConversationOptions() AutoConversationOptions {
	return AutoConversationOptions{
		LoopMaxTurns:           10,
		EnableMemoryExtraction: true,
		AutoHandlePrivate:      true,
		EnableContextCompress:  false,
		EmptyReply:             "我在，稍等我整理一下再回复你。",
	}
}

// AutoConversationRuntime is the high-level auto orchestration runtime.
type AutoConversationRuntime struct {
	agent    *ZapryAgent
	options  AutoConversationOptions
	source   *ProfileSource
	agentKey string

	store     MemoryStore
	extractor MemoryExtractorInterface

	loop        *AgentLoop
	naturalLoop *NaturalAgentLoop

	sessions sync.Map // userID -> *MemorySession
}

var autoConversationRegistry sync.Map // key: fmt.Sprintf("%p", agent) -> *AutoConversationRuntime

// EnableAutoConversation enables SDK-managed automatic conversation orchestration:
// memory + natural conversation + persona tick.
//
// It installs a fallback private-message middleware:
// when update is not handled by existing handlers, SDK will auto-reply.
func EnableAutoConversation(agent *ZapryAgent, opts AutoConversationOptions) (*AutoConversationRuntime, error) {
	if agent == nil {
		return nil, errors.New("auto conversation: agent is nil")
	}

	regKey := fmt.Sprintf("%p", agent)
	if existing, ok := autoConversationRegistry.Load(regKey); ok {
		if rt, castOK := existing.(*AutoConversationRuntime); castOK {
			return rt, nil
		}
	}

	normalized := normalizeAutoConversationOptions(opts)
	if normalized.LLMFn == nil && normalized.LLMFnCtx == nil {
		return nil, errors.New("auto conversation: one of LLMFn/LLMFnCtx is required")
	}

	source := normalized.ProfileSource
	if source == nil && agent.Config != nil {
		source = agent.Config.ProfileSource
	}
	if source == nil {
		return nil, errors.New("auto conversation: profileSource is required (call SetProfileSource first or pass options.ProfileSource)")
	}

	agentKey := strings.TrimSpace(source.AgentKey)
	if agentKey == "" {
		agentKey = "auto-agent"
	}

	runtime := &AutoConversationRuntime{
		agent:    agent,
		options:  normalized,
		source:   source,
		agentKey: agentKey,
		store:    normalized.MemoryStore,
	}

	if err := runtime.initLoopPipeline(); err != nil {
		return nil, err
	}
	runtime.initMemoryExtractor()

	if normalized.AutoHandlePrivate {
		agent.Use(runtime.autoPrivateMiddleware())
	}

	autoConversationRegistry.Store(regKey, runtime)
	return runtime, nil
}

// HandlePrivateMessage processes one private text update via auto runtime.
// Useful when developers want explicit routing but still use SDK auto orchestration.
func (r *AutoConversationRuntime) HandlePrivateMessage(ctx context.Context, bot *AgentAPI, update Update) (*AgentLoopResult, error) {
	if r == nil {
		return nil, errors.New("auto conversation runtime is nil")
	}
	if bot == nil {
		return nil, errors.New("auto conversation: bot is nil")
	}
	if !r.shouldAutoHandle(update) {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	userID := strings.TrimSpace(r.options.SessionIDResolver(update))
	if userID == "" {
		return nil, errors.New("auto conversation: resolved user/session id is empty")
	}
	input := strings.TrimSpace(r.options.UserInputResolver(update))
	if input == "" {
		return nil, nil
	}

	session := r.sessionFor(userID)
	history := r.loadHistory(session)

	// Persist user turn first; history passed to loop excludes current turn.
	if err := session.AddMessage("user", input); err != nil {
		log.Printf("[AutoConversation] add user message failed: %v", err)
	}

	result := r.naturalLoop.RunContext(ctx, session, input, history)
	if result == nil {
		return nil, errors.New("auto conversation: loop result is nil")
	}

	reply := strings.TrimSpace(result.FinalOutput)
	if reply == "" {
		reply = strings.TrimSpace(r.options.EmptyReply)
	}

	if reply != "" && update.Message != nil && update.Message.Chat != nil {
		if _, err := bot.Send(NewMessage(update.Message.Chat.ID, reply)); err != nil {
			return result, fmt.Errorf("auto conversation: send reply failed: %w", err)
		}
	}

	if reply != "" {
		if err := session.AddMessage("assistant", reply); err != nil {
			log.Printf("[AutoConversation] add assistant message failed: %v", err)
		}
	}

	if r.extractor != nil {
		_ = session.ExtractIfNeeded()
	}

	return result, nil
}

func (r *AutoConversationRuntime) autoPrivateMiddleware() MiddlewareFunc {
	return func(ctx *MiddlewareContext, next NextFunc) {
		next()

		// Preserve explicit business handlers: only fallback when not handled.
		if ctx == nil || ctx.Handled {
			return
		}
		if !r.shouldAutoHandle(ctx.Update) {
			return
		}

		if _, err := r.HandlePrivateMessage(context.Background(), ctx.Agent, ctx.Update); err != nil {
			log.Printf("[AutoConversation] auto private fallback failed: %v", err)
			return
		}
		ctx.Handled = true
	}
}

func (r *AutoConversationRuntime) shouldAutoHandle(update Update) bool {
	if update.Message == nil || update.Message.Chat == nil {
		return false
	}
	if !update.Message.Chat.IsPrivate() {
		return false
	}
	if update.Message.IsCommand() {
		return false
	}
	if strings.TrimSpace(update.Message.Text) == "" {
		return false
	}
	if update.Message.From != nil && update.Message.From.IsBot {
		return false
	}
	return true
}

func (r *AutoConversationRuntime) initLoopPipeline() error {
	systemPrompt := strings.TrimSpace(BuildRuntimeSystemPromptFromSource(r.source))

	llmFn := r.options.LLMFn
	if llmFn == nil && r.options.LLMFnCtx != nil {
		llmFn = func(messages []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
			return r.options.LLMFnCtx(context.Background(), messages, tools)
		}
	}
	if llmFn == nil {
		return errors.New("auto conversation: no usable llm function")
	}

	loop := NewAgentLoop(
		llmFn,
		r.options.ToolRegistry,
		systemPrompt,
		r.options.LoopMaxTurns,
		nil,
	)
	loop.LLMFnCtx = r.options.LLMFnCtx
	loop.Guardrails = r.options.Guardrails
	loop.Tracer = r.options.Tracer
	loop.LoopDetector = r.options.LoopDetector
	loop.Capabilities = r.options.Capabilities

	ncCfg := DefaultNaturalConversationConfig()
	if r.options.NaturalConfig != nil {
		ncCfg = *r.options.NaturalConfig
	}

	if r.options.EnableContextCompress && r.options.SummarizeFn != nil {
		ncCfg.ContextCompress = true
		ncCfg.SummarizeFn = r.options.SummarizeFn
	}

	if ncCfg.PersonaConfig == nil {
		builder := r.options.PersonaBuilder
		if builder == nil {
			builder = BuildPersonaSpecFromProfileSource
		}
		if spec := builder(r.source); spec != nil {
			if personaCfg, err := NewPersonaCompiler().Compile(spec); err == nil {
				personaCfg.SystemPrompt = mergeSystemPrompt(systemPrompt, personaCfg.SystemPrompt)
				ncCfg.PersonaConfig = personaCfg
				if ncCfg.PersonaTicker == nil {
					ncCfg.PersonaTicker = NewPersonaTicker()
				}
			} else {
				log.Printf("[AutoConversation] build persona runtime failed: %v", err)
			}
		}
	} else if systemPrompt != "" {
		ncCfg.PersonaConfig.SystemPrompt = mergeSystemPrompt(systemPrompt, ncCfg.PersonaConfig.SystemPrompt)
	}

	nc := NewNaturalConversation(ncCfg)
	r.loop = loop
	r.naturalLoop = nc.WrapLoop(loop)
	return nil
}

func (r *AutoConversationRuntime) initMemoryExtractor() {
	if r.options.MemoryExtractor != nil {
		r.extractor = r.options.MemoryExtractor
		return
	}
	if !r.options.EnableMemoryExtraction {
		return
	}

	extractorFn := r.options.MemoryExtractorFn
	if extractorFn == nil {
		extractorFn = r.defaultMemoryExtractorFn
	}
	r.extractor = NewConsolidatingExtractor(extractorFn, nil)
}

func (r *AutoConversationRuntime) defaultMemoryExtractorFn(prompt string) (string, error) {
	msgs := []map[string]interface{}{
		{"role": "system", "content": "你是记忆抽取助手。请严格输出 JSON，不要输出解释性文本。"},
		{"role": "user", "content": prompt},
	}
	resp, err := r.callLLM(context.Background(), msgs, nil)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", errors.New("memory extractor llm response is nil")
	}
	return strings.TrimSpace(resp.Content), nil
}

func (r *AutoConversationRuntime) callLLM(ctx context.Context, messages []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
	if r.options.LLMFnCtx != nil {
		return r.options.LLMFnCtx(ctx, messages, tools)
	}
	if r.options.LLMFn != nil {
		return r.options.LLMFn(messages, tools)
	}
	return nil, errors.New("no llm function configured")
}

func (r *AutoConversationRuntime) sessionFor(userID string) *MemorySession {
	if existing, ok := r.sessions.Load(userID); ok {
		if session, castOK := existing.(*MemorySession); castOK {
			return session
		}
	}

	created := NewMemorySession(r.agentKey, userID, r.store)
	if r.extractor != nil {
		created.SetExtractor(r.extractor)
	}

	actual, _ := r.sessions.LoadOrStore(userID, created)
	session, ok := actual.(*MemorySession)
	if !ok {
		return created
	}
	return session
}

func (r *AutoConversationRuntime) loadHistory(session *MemorySession) []map[string]interface{} {
	history, err := session.ShortTerm.GetHistory(0)
	if err != nil || len(history) == 0 {
		return nil
	}

	result := make([]map[string]interface{}, 0, len(history))
	for _, msg := range history {
		role := strings.TrimSpace(msg.Role)
		content := strings.TrimSpace(msg.Content)
		if role == "" || content == "" {
			continue
		}
		result = append(result, map[string]interface{}{
			"role":    role,
			"content": content,
		})
	}
	return result
}

func normalizeAutoConversationOptions(opts AutoConversationOptions) AutoConversationOptions {
	normalized := opts
	defaults := DefaultAutoConversationOptions()

	if normalized.ToolRegistry == nil {
		normalized.ToolRegistry = NewToolRegistry()
	}
	if normalized.MemoryStore == nil {
		normalized.MemoryStore = NewInMemoryMemoryStore()
	}
	if normalized.SessionIDResolver == nil {
		normalized.SessionIDResolver = defaultAutoSessionIDResolver
	}
	if normalized.UserInputResolver == nil {
		normalized.UserInputResolver = defaultAutoUserInputResolver
	}
	if normalized.LoopMaxTurns <= 0 {
		normalized.LoopMaxTurns = defaults.LoopMaxTurns
	}

	// EnableMemoryExtraction defaults to true.
	normalized.EnableMemoryExtraction = defaults.EnableMemoryExtraction
	if opts.EnableMemoryExtraction {
		normalized.EnableMemoryExtraction = true
	}
	if opts.DisableMemoryExtraction {
		normalized.EnableMemoryExtraction = false
	}

	// AutoHandlePrivate defaults to true.
	normalized.AutoHandlePrivate = defaults.AutoHandlePrivate
	if opts.AutoHandlePrivate {
		normalized.AutoHandlePrivate = true
	}
	if opts.DisableAutoHandlePrivate {
		normalized.AutoHandlePrivate = false
	}

	if strings.TrimSpace(normalized.EmptyReply) == "" {
		normalized.EmptyReply = defaults.EmptyReply
	}
	return normalized
}

func defaultAutoSessionIDResolver(update Update) string {
	if from := update.SentFrom(); from != nil {
		if id := strings.TrimSpace(from.ID); id != "" {
			return id
		}
	}
	if chat := update.FromChat(); chat != nil {
		if id := strings.TrimSpace(chat.ID); id != "" {
			return id
		}
	}
	return ""
}

func defaultAutoUserInputResolver(update Update) string {
	if update.Message == nil {
		return ""
	}
	return strings.TrimSpace(update.Message.Text)
}

// BuildPersonaSpecFromProfileSource creates a default PersonaSpec from SOUL.md + skills.
func BuildPersonaSpecFromProfileSource(source *ProfileSource) *PersonaSpec {
	if source == nil {
		return nil
	}

	name := extractNameFromSoul(source.SoulMD)
	if name == "" {
		name = strings.TrimSpace(source.AgentKey)
	}
	if name == "" {
		name = "Zapry Assistant"
	}

	skillKeys := SkillKeysFromProfileSource(source)
	hobbies := pickTop(skillKeys, 5)
	if len(hobbies) == 0 {
		hobbies = []string{"对话", "帮助用户"}
	}

	signature := map[string]any{
		"agent_key": strings.TrimSpace(source.AgentKey),
	}
	if len(skillKeys) > 0 {
		signature["skills"] = skillKeys
	}

	return &PersonaSpec{
		Name:              name,
		Traits:            []string{"专业", "稳定", "真诚"},
		Hobbies:           hobbies,
		RelationshipStyle: "friend",
		Tone:              "warm",
		Locale:            "zh-CN",
		SignatureDetails:  signature,
	}
}

var (
	soulNameLinePattern = regexp.MustCompile(`(?mi)^name\s*[:：]\s*(.+)$`)
	soulTitlePattern    = regexp.MustCompile(`(?m)^#\s+(.+)$`)
)

func extractNameFromSoul(soul string) string {
	text := strings.TrimSpace(soul)
	if text == "" {
		return ""
	}

	if matches := soulNameLinePattern.FindStringSubmatch(text); len(matches) >= 2 {
		if name := cleanPersonaName(matches[1]); name != "" {
			return name
		}
	}
	if matches := soulTitlePattern.FindStringSubmatch(text); len(matches) >= 2 {
		if name := cleanPersonaName(matches[1]); name != "" {
			return name
		}
	}
	return ""
}

func cleanPersonaName(raw string) string {
	name := strings.TrimSpace(raw)
	name = strings.Trim(name, "\"'#*`")
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	runes := []rune(name)
	if len(runes) > 32 {
		return string(runes[:32])
	}
	return name
}

func pickTop(values []string, n int) []string {
	if len(values) == 0 || n <= 0 {
		return nil
	}
	out := make([]string, 0, n)
	seen := make(map[string]struct{}, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
		if len(out) >= n {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mergeSystemPrompt(basePrompt, personaPrompt string) string {
	basePrompt = strings.TrimSpace(basePrompt)
	personaPrompt = strings.TrimSpace(personaPrompt)
	switch {
	case basePrompt == "" && personaPrompt == "":
		return ""
	case basePrompt == "":
		return personaPrompt
	case personaPrompt == "":
		return basePrompt
	case strings.Contains(personaPrompt, basePrompt):
		return personaPrompt
	default:
		return basePrompt + "\n\n" + personaPrompt
	}
}
