package agentsdk

// ──────────────────────────────────────────────
// Middleware — Onion-model middleware pipeline
// ──────────────────────────────────────────────
//
// Each middleware wraps the next layer. Call next() to proceed;
// skip it to intercept.
//
// Usage:
//
//	agent.Use(func(ctx *MiddlewareContext, next NextFunc) {
//	    log.Println("before")
//	    next()
//	    log.Println("after")
//	})

// NextFunc proceeds to the next middleware or the core handler.
type NextFunc func()

// MiddlewareFunc is the signature for all middleware functions.
// Call next() to proceed to the next layer; skip it to intercept.
type MiddlewareFunc func(ctx *MiddlewareContext, next NextFunc)

// MiddlewareContext is the shared context flowing through the pipeline.
type MiddlewareContext struct {
	// Update is the incoming Telegram/Zapry update.
	Update Update
	// Agent is the low-level API client.
	Agent *AgentAPI
	// Extra is an arbitrary map for middleware to attach/read data.
	Extra map[string]interface{}
	// Handled is set to true when the core handler has been reached.
	Handled bool
}

// MiddlewarePipeline builds and executes an onion-model call chain.
type MiddlewarePipeline struct {
	middlewares []MiddlewareFunc
}

// NewMiddlewarePipeline creates an empty pipeline.
func NewMiddlewarePipeline() *MiddlewarePipeline {
	return &MiddlewarePipeline{}
}

// Use appends a middleware to the pipeline.
func (p *MiddlewarePipeline) Use(mw MiddlewareFunc) {
	p.middlewares = append(p.middlewares, mw)
}

// Len returns the number of registered middlewares.
func (p *MiddlewarePipeline) Len() int {
	return len(p.middlewares)
}

// Execute runs the full pipeline ending with coreHandler.
//
// The pipeline builds an onion chain:
//
//	mw[0].before → mw[1].before → core → mw[1].after → mw[0].after
func (p *MiddlewarePipeline) Execute(ctx *MiddlewareContext, coreHandler func()) {
	if len(p.middlewares) == 0 {
		coreHandler()
		return
	}

	// Build chain from inside out
	chain := coreHandler
	for i := len(p.middlewares) - 1; i >= 0; i-- {
		mw := p.middlewares[i]
		next := chain
		chain = func() {
			mw(ctx, next)
		}
	}

	chain()
}
