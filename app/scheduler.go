package app

import (
	"sync"
	"time"

	"git.digineo.de/digineo/zackup/config"
	"github.com/sirupsen/logrus"
)

// The Scheduler periodically performs backups.
type Scheduler interface {
	// Start begins a new schedule cycle. This method will block until
	// you call Stop().
	Start()

	// Stop halts the scheduler. Running jobs are still finished, though.
	Stop()
}

type scheduler struct {
	config config.Tree
	queue  Queue
	logger *logrus.Entry

	quit chan struct{} // interrupts loop in Start()
	stop bool          // interrupts loop in run()
	wg   sync.WaitGroup

	sync.RWMutex
}

// NewScheduler returns a new scheduler instance. It reads the schedule
// interval from the config.Tree and enqueue new backup jobs into queue.
// The instance is not started yet, you need to call Start().
func NewScheduler(tree config.Tree, queue Queue) Scheduler {
	sch := &scheduler{
		config: tree,
		queue:  queue,
		logger: log.WithField("prefix", "scheduler"),
	}

	sch.wg.Add(1)
	return sch
}

func (sch *scheduler) Start() {
	sch.quit = make(chan struct{})
	sch.stop = false

	const interval = time.Minute
	t := time.NewTimer(interval)

	defer sch.wg.Done()

	for {
		select {
		case <-t.C:
			sch.run()
		case <-sch.quit:
			t.Stop()
			sch.quit = nil
			return
		}
		t.Reset(interval)
	}
}

func (sch *scheduler) Stop() {
	sch.stop = true
	close(sch.quit)
	sch.wg.Wait()
}

func (sch *scheduler) run() {
	next := sch.config.Service().NextSchedule

	sch.Lock()
	defer sch.Unlock()

	for _, host := range sch.config.Hosts() {
		if sch.stop {
			// abort early if Stop() was called
			return
		}

		job := sch.config.Host(host)
		if job == nil {
			continue
		}

		now := time.Now()
		if job.NextRun().IsZero() {
			job.Schedule(next(now))
		}

		if job.IsActive() || job.NextRun().After(now) {
			// do not touch active or planned jobs
			continue
		}

		// this might block if backlog is full
		job.Start()
		sch.queue.Enqueue(job)
		job.Schedule(next(time.Now()))
	}
}
