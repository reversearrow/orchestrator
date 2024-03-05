package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
	"github.com/reversearrow/orchestrator/container/docker"
	"github.com/reversearrow/orchestrator/task"
)

type Worker struct {
	Name      string
	Queue     queue.Queue
	Db        map[uuid.UUID]*task.Task
	TaskCount int
	Logger    *log.Logger
	Stats     *Stats
}

func NewWorker(logger *log.Logger, queue *queue.Queue, db map[uuid.UUID]*task.Task) (*Worker, error) {
	w := &Worker{
		Queue:  *queue,
		Db:     db,
		Logger: logger,
	}

	return w, w.validate()
}

func (w *Worker) validate() error {
	if w.Db == nil {
		return fmt.Errorf("worker: db is nil")
	}

	if w.Logger == nil {
		return fmt.Errorf("worker: logger is nil")
	}

	return nil
}

func (w *Worker) CollectStats() {
	for {
		w.Logger.Println("collecting system stats")
		w.Stats = GetStats(w.Logger)
		w.Stats.TaskCount = w.TaskCount
		time.Sleep(time.Second * 15)
	}
}

func (w *Worker) AddTask(ctx context.Context, t task.Task) {
	w.Queue.Enqueue(t)
}

func (w *Worker) runTask(ctx context.Context) task.Result {
	t := w.Queue.Dequeue()
	if t == nil {
		w.Logger.Printf("no tasks in queue")
		return task.Result{Error: nil}
	}

	taskQueued := t.(task.Task)
	taskPersisted := w.Db[taskQueued.ID]
	if taskPersisted == nil {
		taskPersisted = &taskQueued
		w.Db[taskQueued.ID] = &taskQueued
	}

	var result task.Result
	if task.ValidStateTransition(taskPersisted.State, taskQueued.State) {
		switch taskQueued.State {
		case task.Scheduled:
			result = w.StartTask(ctx, taskQueued)
		case task.Completed:
			result = w.StopTask(ctx, taskQueued)
		default:
			result.Error = errors.New("we should not get here")
		}
	} else {
		result.Error = fmt.Errorf("invalid transition from %v to %v", taskPersisted.State, taskQueued.State)
	}

	return result
}

func (w *Worker) StartTask(ctx context.Context, t task.Task) task.Result {
	t.StartTime = time.Now().UTC()
	cfg := docker.NewConfig(&t)
	d, err := docker.NewDocker(cfg)
	if err != nil {
		return task.Result{
			Error: fmt.Errorf("error creating a new docker instance: %w", err),
		}
	}
	result := d.Run(ctx)
	if result.Error != nil {
		w.Logger.Printf("error starting the task: %v", result.Error)
		t.State = task.Failed
		w.Db[t.ID] = &t
		return result
	}
	t.ContainerID = result.ContainerId
	t.State = task.Running
	w.Db[t.ID] = &t
	return result
}

func (w *Worker) StopTask(ctx context.Context, t task.Task) task.Result {
	cfg := docker.NewConfig(&t)
	d, err := docker.NewDocker(cfg)
	if err != nil {
		return task.Result{
			Error: fmt.Errorf("error creating a new docker instance: %w", err),
		}
	}

	result := d.Stop(ctx, t.ContainerID)
	if result.Error != nil {
		return result
	}
	t.FinishTime = time.Now().UTC()
	t.State = task.Completed
	w.Db[t.ID] = &t
	w.Logger.Printf("stopped and removed container %v for task %v\n", t.ContainerID, t.ID)
	return result
}

func (w *Worker) GetTasks() []*task.Task {
	tasks := make([]*task.Task, 0, len(w.Db))
	for _, t := range tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

func (w *Worker) RunTasks(ctx context.Context, logger *log.Logger) {
	const sleep = time.Second * 10

	for {
		if w.Queue.Len() == 0 {
			logger.Println("no available tasks to run, sleeping for 10 seconds")
			time.Sleep(sleep)
			continue
		}

		result := w.runTask(ctx)
		if result.Error != nil {
			logger.Printf("error running tasks: %v\n", result.Error)
		}
	}
}

func (w *Worker) InspectTask(ctx context.Context, t task.Task) docker.InspectResponse {
	cfg := docker.NewConfig(&t)
	d, err := docker.NewDocker(cfg)
	if err != nil {
		return docker.InspectResponse{
			Error: fmt.Errorf("error creating a new docker instance: %w", err),
		}
	}
	return d.Inspect(ctx, t.ContainerID)
}

func (w *Worker) updateTasks() {
	for
}

func (w *Worker) UpdateTasks() {
	for {
		w.Logger.Println("checking status of tasks")
		w.Logger.Println("task updates completed")
		w.updateTasks()
		w.Logger.Println("sleeping for 15 seconds")
		time.Sleep(15 * time.Second)
	}
}
