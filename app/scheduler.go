package app

import (
	"time"

	"git.digineo.de/digineo/zackup/config"
	"github.com/sirupsen/logrus"
)

// The Scheduler periodically performs backups.
type Scheduler interface {
	// Start begins a new schedule cycle.
	Start()
}

type scheduler struct {
	config  config.Tree
	queue   Queue
	nextRun time.Time
}

// NewScheduler returns a new scheduler instance. It reads the schedule
// interval from the config.Tree and enqueue new backup jobs into queue.
func NewScheduler(tree config.Tree, queue Queue) Scheduler {
	sch := &scheduler{
		config: tree,
		queue:  queue,
	}
	go sch.Start()
	return sch
}

func (sch *scheduler) Start() {
	for {
		now := time.Now().UTC()
		next := sch.config.Service().NextSchedule(now)
		diff := next.Sub(now).Truncate(time.Minute)
		if diff < time.Minute {
			diff = time.Minute
		}
		log.WithFields(logrus.Fields{
			"prefix": "scheduler",
			"sleep":  diff.Truncate(time.Second).String(),
			"date":   now.Add(diff).Truncate(time.Minute).Format(time.RFC3339),
		}).Info("scheduled next backup cycle")

		time.Sleep(diff)
		sch.run()
	}
}

func (sch *scheduler) run() {
	for _, name := range sch.config.Hosts() {
		if job := sch.config.Host(name); job != nil {
			sch.queue.Enqueue(job)
		}
	}
}
