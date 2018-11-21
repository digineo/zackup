package app

import (
	"sync"

	"git.digineo.de/digineo/zackup/config"
)

// Queue manages the parallel execution of jobs.
type Queue interface {
	// Enqueue adds a job to the queue. It is run immediately after all
	// previous queued items are consumed.
	Enqueue(host string, job *config.JobConfig)

	// Resize changes the size of the queue. When sizing down, surplus
	// running jobs will finish. Values for newSize are capped; for values
	// less then 1, 1 is assumed, and for values larger than an arbitrary
	// threshold, that threshold value is assumed.
	Resize(newSize int)
}

type queueItem struct {
	hostname string
	job      *config.JobConfig
}

type quitCh chan struct{}

type queue struct {
	workers []quitCh
	jobs    chan *queueItem
	wg      *sync.WaitGroup

	sync.RWMutex
}

// NewQueue constructs an empty queue with the given size and starts
// the same amount of workers.
func NewQueue(size int) Queue {
	if size < 1 {
		size = 1
	} else if size > qResizeThreshold {
		size = qResizeThreshold
	}

	q := queue{
		workers: make([]quitCh, 0, qResizeThreshold),
		jobs:    make(chan *queueItem),
		wg:      &sync.WaitGroup{},
	}

	q.wg.Add(int(size))
	for i := 0; i < size; i++ {
		q.newWorker()
	}

	return &q
}

func (q *queue) newWorker() {
	q.Lock()
	quit := make(quitCh)
	q.workers = append(q.workers, quit)
	q.Unlock()

	go func() {
	Loop:
		for {
			select {
			case j := <-q.jobs:
				PerformBackup(j.hostname, j.job)
			case <-quit:
				break Loop
			}
		}
		q.wg.Done()
	}()
}

const qResizeThreshold = 255

func (q *queue) Resize(newSize int) {
	if newSize < 0 {
		newSize = 1
	} else if newSize > qResizeThreshold {
		newSize = qResizeThreshold
	}

	q.Lock()
	defer q.Unlock()

	diff := len(q.workers) - newSize

	if diff > 0 {
		// kill surplus of workers, see notes below for details
		for _, quit := range q.workers[:diff] {
			close(quit)
		}
		copy(q.workers, q.workers[diff:])
		for i := range q.workers[newSize:] {
			q.workers[newSize+i] = nil
		}
		q.workers = q.workers[:newSize]
	} else if diff < 0 {
		// create missing workers
		q.wg.Add(-diff)
		for i := 0; i < -diff; i++ {
			go q.newWorker()
		}
	}
}

func (q *queue) Enqueue(host string, job *config.JobConfig) {
	q.jobs <- &queueItem{hostname: host, job: job}
}

// Notes on the "kill surplus of workers" algorithm
//
// The algorithm removes the first n elements from a slice and adjusts
// its size (length) afterwards. It also preserves the order of its
// elements, but that is not really important. More important is that
// it avoids memory leaks.
//
// For the implementation, let's consider this illustration:
//
// Let x be a slice of len(x)=5 and cap(x)=6:
//
//    x := [a b c d e | _]
//
// where a..e denote some elements (x[0]=a, x[1]=b, ...), "|" marks the
// len/cap position and "_" is the zero value for any given element.
//
// Our goal is to resize x to have a length of s=3. This means we need
// to remove the first n = len(x)-s = 2 elements.
//
// We first copy the remaining elements (c, d, e) to the front,
// overwriting (a, b, c):
//
//    copy(x, x[n:])
//    => x = [c d e d e | _]
//
// Note that x[s:] (x[1]==x[3] and x[2]==x[4]) now contain duplicates,
// because that's how copy() works. If we resize the slice with
//
//    x = x[:s]
//    => x = [c d e | d e _]
//
// the duplicates are still stored in the underlying array an cannot be
// garbage collected. In this iteration it would not be problematic, but
// it might lead to this situation:
//
//    // resize down to 1 element, note how d is still referenced
//    // and cannot be GC'ed:
//    => x = [e | d e d e _]
//
//    // to allow for d to be GC'ed, we need to append (at least)
//    // 3 new elements to overwrite all existing references:
//    => x = [e f g h | e _]
//
// Now, after downsizing to len=3, we'd leave e in the underlying array.
// These numbers are worse for slices with larger metrics.
//
// To allow GC, we need to remove those pesky references prior to the
// resizing operation (setting to _):
//
//    x[s] = x[s+1] = ... x[len(x)-1] = _
//    => x = [c d e _ _ | _]
//
//    x = x[:s]
//    => x = [c d e | _ _ _]
