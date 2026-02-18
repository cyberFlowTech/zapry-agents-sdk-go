package agentsdk

// ──────────────────────────────────────────────
// Persona re-exports — stable public API
// ──────────────────────────────────────────────
//
// Re-exports the most commonly used Persona types so users can access them
// from the root package:
//
//	spec := agentsdk.PersonaSpec{Name: "林晚晴", Traits: []string{"温柔"}}
//
// For the full Persona API (compiler, ticker, store, violation, MBTI),
// import the sub-package directly:
//
//	import "github.com/cyberFlowTech/zapry-agents-sdk-go/persona"

import "github.com/cyberFlowTech/zapry-agents-sdk-go/persona"

// ─── Core types ───

// PersonaSpec is the developer input for creating a persona.
type PersonaSpec = persona.PersonaSpec

// PersonaRuntimeConfig is the compiled output of a PersonaSpec.
type PersonaRuntimeConfig = persona.RuntimeConfig

// PersonaTick is the per-turn runtime context (state, mood, style constraints).
type PersonaTick = persona.PersonaTick

// PersonaStylePolicy controls conversational behavior constraints.
type PersonaStylePolicy = persona.StylePolicy

// PersonaStore is the interface for persisting persona configs.
type PersonaStore = persona.PersonaStore

// ─── Constructors ───

// NewPersonaCompiler creates a new Persona compiler.
var NewPersonaCompiler = persona.NewCompiler

// NewPersonaTicker creates a new local ticker.
var NewPersonaTicker = persona.NewLocalTicker

// NewPersonaFileStore creates a file-based persona store.
var NewPersonaFileStore = persona.NewFileStore

// NewInMemoryPersonaStore creates an in-memory persona store (for testing).
var NewInMemoryPersonaStore = persona.NewInMemoryPersonaStore
