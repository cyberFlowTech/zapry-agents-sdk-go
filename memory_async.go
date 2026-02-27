package agentsdk

import (
	"context"
	"log"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Async Memory Extractor — background extraction pipeline
// ──────────────────────────────────────────────

// AsyncExtractorConfig controls the background extraction pipeline.
type AsyncExtractorConfig struct {
	Workers   int           // background worker goroutines, default 1
	QueueSize int           // buffered channel capacity, default 100
	BatchWait time.Duration // max wait before processing a partial batch, default 5s
}

// DefaultAsyncExtractorConfig returns production defaults.
func DefaultAsyncExtractorConfig() AsyncExtractorConfig {
	return AsyncExtractorConfig{
		Workers:   1,
		QueueSize: 100,
		BatchWait: 5 * time.Second,
	}
}

type extractJob struct {
	Conversations []map[string]string
	CurrentMemory map[string]interface{}
	LongTerm      *LongTermMemory
	Namespace     string
}

// ExtractionResult is emitted after each background extraction completes.
type ExtractionResult struct {
	Namespace string
	Delta     map[string]interface{}
	Err       error
}

// AsyncMemoryExtractor decouples memory extraction from the conversation hot path.
// The caller enqueues jobs via Submit(); background workers execute extraction
// and apply results to LongTermMemory.
type AsyncMemoryExtractor struct {
	extractor MemoryExtractorInterface
	config    AsyncExtractorConfig
	queue     chan extractJob
	wg        sync.WaitGroup
	cancel    context.CancelFunc

	// OnResult is called (from worker goroutine) after each extraction.
	// May be nil.
	OnResult func(ExtractionResult)
}

// NewAsyncMemoryExtractor creates and starts an async extraction pipeline.
// Call Stop() to drain the queue and shut down workers.
func NewAsyncMemoryExtractor(extractor MemoryExtractorInterface, config ...AsyncExtractorConfig) *AsyncMemoryExtractor {
	cfg := DefaultAsyncExtractorConfig()
	if len(config) > 0 {
		cfg = config[0]
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 100
	}

	ctx, cancel := context.WithCancel(context.Background())
	a := &AsyncMemoryExtractor{
		extractor: extractor,
		config:    cfg,
		queue:     make(chan extractJob, cfg.QueueSize),
		cancel:    cancel,
	}

	for i := 0; i < cfg.Workers; i++ {
		a.wg.Add(1)
		go a.worker(ctx)
	}
	return a
}

// Submit enqueues an extraction job. Non-blocking; drops if queue is full.
// Returns true if enqueued, false if dropped.
func (a *AsyncMemoryExtractor) Submit(conversations []map[string]string, currentMemory map[string]interface{}, lt *LongTermMemory, namespace string) bool {
	job := extractJob{
		Conversations: conversations,
		CurrentMemory: currentMemory,
		LongTerm:      lt,
		Namespace:     namespace,
	}
	select {
	case a.queue <- job:
		return true
	default:
		log.Printf("[AsyncMemoryExtractor] Queue full, dropping extraction job for ns=%s", namespace)
		return false
	}
}

// Pending returns the number of jobs waiting in the queue.
func (a *AsyncMemoryExtractor) Pending() int {
	return len(a.queue)
}

// Stop signals workers to drain remaining jobs and exit. Blocks until done.
func (a *AsyncMemoryExtractor) Stop() {
	a.cancel()
	close(a.queue)
	a.wg.Wait()
}

func (a *AsyncMemoryExtractor) worker(ctx context.Context) {
	defer a.wg.Done()
	for {
		select {
		case job, ok := <-a.queue:
			if !ok {
				return
			}
			a.processJob(job)
		case <-ctx.Done():
			// Drain remaining
			for job := range a.queue {
				a.processJob(job)
			}
			return
		}
	}
}

func (a *AsyncMemoryExtractor) processJob(job extractJob) {
	extracted, err := a.extractor.Extract(job.Conversations, job.CurrentMemory)

	result := ExtractionResult{
		Namespace: job.Namespace,
		Err:       err,
	}

	if err != nil {
		log.Printf("[AsyncMemoryExtractor] Extraction failed for ns=%s: %v", job.Namespace, err)
	} else if len(extracted) > 0 {
		if _, updateErr := job.LongTerm.Update(extracted); updateErr != nil {
			log.Printf("[AsyncMemoryExtractor] LongTerm update failed for ns=%s: %v", job.Namespace, updateErr)
			result.Err = updateErr
		} else {
			result.Delta = extracted
			log.Printf("[AsyncMemoryExtractor] Extraction applied | ns=%s", job.Namespace)
		}
	}

	if a.OnResult != nil {
		a.OnResult(result)
	}
}

// ──────────────────────────────────────────────
// MemorySession integration — ExtractAsync
// ──────────────────────────────────────────────

// ExtractAsync submits a background extraction job instead of blocking.
// The conversation buffer is consumed immediately; extraction runs in background.
// Returns true if the job was enqueued.
func (s *MemorySession) ExtractAsync(async *AsyncMemoryExtractor) bool {
	if s.extractor == nil && async == nil {
		return false
	}

	should, err := s.Buffer.ShouldExtract()
	if err != nil || !should {
		return false
	}

	conversations, err := s.Buffer.GetAndClear()
	if err != nil || len(conversations) == 0 {
		return false
	}

	current, err := s.LongTerm.Get()
	if err != nil {
		return false
	}

	return async.Submit(conversations, current, s.LongTerm, s.Namespace)
}
