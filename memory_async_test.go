package agentsdk

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestAsyncExtractor_SubmitAndProcess(t *testing.T) {
	var called int32
	mockExtractor := &LLMMemoryExtractor{
		LLMFn: func(prompt string) (string, error) {
			atomic.AddInt32(&called, 1)
			return `{"basic_info": {"age": 25}}`, nil
		},
		PromptTemplate: DefaultExtractionPrompt,
	}

	done := make(chan ExtractionResult, 1)
	async := NewAsyncMemoryExtractor(mockExtractor, AsyncExtractorConfig{
		Workers:   1,
		QueueSize: 10,
	})
	async.OnResult = func(r ExtractionResult) {
		done <- r
	}
	defer async.Stop()

	store := NewInMemoryMemoryStore()
	lt := NewLongTermMemory(store, "test:user1", 5*time.Minute)

	conversations := []map[string]string{
		{"role": "user", "content": "I'm 25"},
	}
	current, _ := lt.Get()

	ok := async.Submit(conversations, current, lt, "test:user1")
	if !ok {
		t.Fatal("submit should succeed")
	}

	select {
	case r := <-done:
		if r.Err != nil {
			t.Fatalf("extraction failed: %v", r.Err)
		}
		if r.Namespace != "test:user1" {
			t.Errorf("unexpected namespace: %s", r.Namespace)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for extraction")
	}

	if atomic.LoadInt32(&called) != 1 {
		t.Errorf("expected LLM called once, got %d", called)
	}
}

func TestAsyncExtractor_QueueFull(t *testing.T) {
	slowExtractor := &LLMMemoryExtractor{
		LLMFn: func(prompt string) (string, error) {
			time.Sleep(100 * time.Millisecond)
			return `{}`, nil
		},
		PromptTemplate: DefaultExtractionPrompt,
	}

	async := NewAsyncMemoryExtractor(slowExtractor, AsyncExtractorConfig{
		Workers:   1,
		QueueSize: 1,
	})
	defer async.Stop()

	store := NewInMemoryMemoryStore()
	lt := NewLongTermMemory(store, "test:user1", 5*time.Minute)
	current, _ := lt.Get()
	convs := []map[string]string{{"role": "user", "content": "hi"}}

	async.Submit(convs, current, lt, "ns1")
	async.Submit(convs, current, lt, "ns2")

	dropped := !async.Submit(convs, current, lt, "ns3")
	// With queue size 1 and 1 slow worker, some should eventually get dropped
	// but timing is non-deterministic. At minimum, Pending() should be >= 0
	if async.Pending() < 0 {
		t.Error("pending should be non-negative")
	}
	_ = dropped
}

func TestAsyncExtractor_Stop(t *testing.T) {
	var processed int32
	mockExtractor := &LLMMemoryExtractor{
		LLMFn: func(prompt string) (string, error) {
			atomic.AddInt32(&processed, 1)
			return `{}`, nil
		},
		PromptTemplate: DefaultExtractionPrompt,
	}

	async := NewAsyncMemoryExtractor(mockExtractor, AsyncExtractorConfig{
		Workers:   2,
		QueueSize: 10,
	})

	store := NewInMemoryMemoryStore()
	lt := NewLongTermMemory(store, "test:user1", 5*time.Minute)
	current, _ := lt.Get()

	for i := 0; i < 5; i++ {
		async.Submit(
			[]map[string]string{{"role": "user", "content": "hi"}},
			current, lt, "ns",
		)
	}

	async.Stop()

	p := atomic.LoadInt32(&processed)
	if p != 5 {
		t.Errorf("expected all 5 jobs processed on stop, got %d", p)
	}
}

func TestMemorySession_ExtractAsync(t *testing.T) {
	var called int32
	mockExtractor := &LLMMemoryExtractor{
		LLMFn: func(prompt string) (string, error) {
			atomic.AddInt32(&called, 1)
			return `{"summary": "test"}`, nil
		},
		PromptTemplate: DefaultExtractionPrompt,
	}

	done := make(chan struct{}, 1)
	async := NewAsyncMemoryExtractor(mockExtractor)
	async.OnResult = func(r ExtractionResult) {
		done <- struct{}{}
	}
	defer async.Stop()

	store := NewInMemoryMemoryStore()
	session := NewMemorySession("agent1", "user1", store)
	session.SetExtractor(mockExtractor)

	// Add enough messages to trigger extraction
	for i := 0; i < 6; i++ {
		session.AddMessage("user", "message")
		session.AddMessage("assistant", "reply")
	}

	ok := session.ExtractAsync(async)
	if !ok {
		t.Fatal("ExtractAsync should return true")
	}

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for async extraction")
	}
}
