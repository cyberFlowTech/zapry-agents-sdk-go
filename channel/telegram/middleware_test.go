package telegram

import (
	"testing"
)

func TestMiddlewarePipeline_Empty(t *testing.T) {
	p := NewMiddlewarePipeline()
	called := false
	p.Execute(&MiddlewareContext{}, func() { called = true })
	if !called {
		t.Fatal("core handler should be called with empty pipeline")
	}
}

func TestMiddlewarePipeline_SingleBeforeAfter(t *testing.T) {
	var order []string
	p := NewMiddlewarePipeline()
	p.Use(func(ctx *MiddlewareContext, next NextFunc) {
		order = append(order, "before")
		next()
		order = append(order, "after")
	})
	p.Execute(&MiddlewareContext{}, func() { order = append(order, "core") })

	expected := []string{"before", "core", "after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("at index %d: expected %q, got %q", i, v, order[i])
		}
	}
}

func TestMiddlewarePipeline_OnionOrder(t *testing.T) {
	var order []string
	p := NewMiddlewarePipeline()

	p.Use(func(ctx *MiddlewareContext, next NextFunc) {
		order = append(order, "mw1>")
		next()
		order = append(order, "<mw1")
	})
	p.Use(func(ctx *MiddlewareContext, next NextFunc) {
		order = append(order, "mw2>")
		next()
		order = append(order, "<mw2")
	})

	p.Execute(&MiddlewareContext{}, func() { order = append(order, "CORE") })

	expected := []string{"mw1>", "mw2>", "CORE", "<mw2", "<mw1"}
	if len(order) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("at index %d: expected %q, got %q", i, v, order[i])
		}
	}
}

func TestMiddlewarePipeline_Intercept(t *testing.T) {
	coreCalled := false
	p := NewMiddlewarePipeline()
	p.Use(func(ctx *MiddlewareContext, next NextFunc) {
		// do NOT call next
	})
	p.Execute(&MiddlewareContext{}, func() { coreCalled = true })
	if coreCalled {
		t.Fatal("core should NOT be called when middleware intercepts")
	}
}

func TestMiddlewarePipeline_ContextShared(t *testing.T) {
	p := NewMiddlewarePipeline()
	p.Use(func(ctx *MiddlewareContext, next NextFunc) {
		ctx.Extra["user"] = "admin"
		next()
	})
	p.Use(func(ctx *MiddlewareContext, next NextFunc) {
		if ctx.Extra["user"] != "admin" {
			t.Fatal("expected user=admin in context")
		}
		ctx.Extra["checked"] = true
		next()
	})

	ctx := &MiddlewareContext{Extra: make(map[string]interface{})}
	p.Execute(ctx, func() {})
	if ctx.Extra["checked"] != true {
		t.Fatal("expected checked=true")
	}
}

func TestMiddlewarePipeline_ThreeMiddlewares(t *testing.T) {
	var order []string
	p := NewMiddlewarePipeline()
	p.Use(func(ctx *MiddlewareContext, next NextFunc) {
		order = append(order, "a>")
		next()
		order = append(order, "<a")
	})
	p.Use(func(ctx *MiddlewareContext, next NextFunc) {
		order = append(order, "b>")
		next()
		order = append(order, "<b")
	})
	p.Use(func(ctx *MiddlewareContext, next NextFunc) {
		order = append(order, "c>")
		next()
		order = append(order, "<c")
	})

	p.Execute(&MiddlewareContext{}, func() { order = append(order, "CORE") })

	expected := []string{"a>", "b>", "c>", "CORE", "<c", "<b", "<a"}
	if len(order) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("at %d: expected %q, got %q", i, v, order[i])
		}
	}
}

func TestMiddlewarePipeline_Len(t *testing.T) {
	p := NewMiddlewarePipeline()
	if p.Len() != 0 {
		t.Fatal("expected 0")
	}
	p.Use(func(ctx *MiddlewareContext, next NextFunc) { next() })
	if p.Len() != 1 {
		t.Fatal("expected 1")
	}
}
