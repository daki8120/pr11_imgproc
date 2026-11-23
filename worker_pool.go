package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type AdaptivePool struct {
	maxWorkers int32
	active     int32
	queue      chan Job
	results    chan JobResult
	wg         sync.WaitGroup
}

func NewAdaptivePool(initWorkers, maxWorkers, queueCap int) *AdaptivePool {
	p := &AdaptivePool{
		maxWorkers: int32(maxWorkers),
		queue:      make(chan Job, queueCap),
		results:    make(chan JobResult, queueCap),
	}
	for i := 0; i < initWorkers; i++ {
		p.spawnWorker()
	}
	return p
}

func (p *AdaptivePool) spawnWorker() {
	if atomic.LoadInt32(&p.active) >= p.maxWorkers {
		return
	}
	atomic.AddInt32(&p.active, 1)
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for job := range p.queue {
			p.results <- handleJob(job)
		}
	}()
}

// Submit добавляет задачу и масштабирует пул при накоплении очереди
func (p *AdaptivePool) Submit(j Job) {
	if len(p.queue) > cap(p.queue)/2 && atomic.LoadInt32(&p.active) < p.maxWorkers {
		p.spawnWorker()
	}
	p.queue <- j
}

func (p *AdaptivePool) CloseAndWait() {
	close(p.queue)
	p.wg.Wait()
	close(p.results)
}

func runBenchmark(jobs []Job, counts []int) {
	fmt.Println("\n=== Сравнительный тест (Benchmark) ===")
	t0 := time.Now()
	for _, j := range jobs {
		handleJob(j)
	}
	seqTime := time.Since(t0)
	fmt.Printf("Последовательно (1 поток): %v\n", seqTime.Round(time.Millisecond))

	for _, w := range counts {
		pool := NewAdaptivePool(w, w, len(jobs))
		start := time.Now()
		go func() {
			for _, j := range jobs {
				pool.Submit(j)
			}
			pool.CloseAndWait()
		}()
		for range pool.results {
		}
		parTime := time.Since(start)
		speedup := float64(seqTime) / float64(parTime)
		fmt.Printf("Параллельно (%d воркеров): %v | Ускорение (Speedup): %.2fx\n",
			w, parTime.Round(time.Millisecond), speedup)
	}
}
