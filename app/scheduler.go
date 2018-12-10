package app

import (
	"sync"
	"time"

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
func NewScheduler(queue Queue) Scheduler {
	sch := &scheduler{
		queue:  queue,
		logger: log.WithField("prefix", "scheduler"),
	}

	sch.wg.Add(1)
	return sch
}

func (sch *scheduler) Start() {
	defer sch.wg.Done()

	sch.quit = make(chan struct{})
	sch.stop = false

	// first polling should happen fast(-ish)
	t := time.NewTimer(10 * time.Second)

	for {
		select {
		case <-t.C:
			sch.run()
		case <-sch.quit:
			t.Stop()
			sch.quit = nil
			return
		}
		// successive polls may happen less frequently
		t.Reset(time.Minute)
	}
}

func (sch *scheduler) Stop() {
	sch.stop = true
	close(sch.quit)
	sch.wg.Wait()
}

func (sch *scheduler) run() {
	sch.Lock()
	defer sch.Unlock()

	for host, job := range state.hosts {
		if sch.stop {
			// abort early if Stop() was called
			return
		}
		if job == nil {
			// safety-net
			continue
		}

		now := time.Now()
		if job.ScheduledAt.IsZero() {
			state.reschedule(host, now)
		}

		s := job.Status()
		l := sch.logger.WithFields(logrus.Fields{
			"job":          host,
			"scheduled-at": job.ScheduledAt.Format(time.RFC3339),
			"status":       s.String(),
		})
		if s == StatusRunning || job.ScheduledAt.After(now) {
			l.Debug("ignore active or planned jobs")
			continue
		}

		// this might block if backlog is full
		l.Info("enqueueing job")
		sch.queue.Enqueue(job.job)

		l.Info("rescheduleing job")
		state.reschedule(host, time.Now())
	}
}
