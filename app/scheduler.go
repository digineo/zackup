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
	t := time.NewTimer(sch.plan())

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
		t.Reset(sch.plan())
	}
}

func (sch *scheduler) Stop() {
	sch.stop = true
	close(sch.quit)
	sch.wg.Wait()
}

func (sch *scheduler) plan() time.Duration {
	now := time.Now().UTC()
	next := sch.config.Service().NextSchedule(now)
	diff := next.Sub(now).Truncate(time.Second)

	if diff < time.Minute {
		diff = time.Minute
	}
	sch.nextRun = now.Add(diff)

	sch.logger.WithFields(logrus.Fields{
		"sleep": int64(diff / time.Second),
		"date":  sch.nextRun.Format(time.RFC3339),
	}).Info("scheduled next backup cycle")

	return diff
}

func (sch *scheduler) run() {
	for _, name := range sch.config.Hosts() {
		if sch.stop {
			break
		}
		if job := sch.config.Host(name); job != nil {
			sch.queue.Enqueue(job)
		}
	}
}
