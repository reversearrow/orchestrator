package manager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	url2 "net/url"
	"time"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
	"github.com/reversearrow/orchestrator/task"
	"github.com/reversearrow/orchestrator/worker"
)

type Manager struct {
	Pending       queue.Queue
	TaskDb        map[uuid.UUID]*task.Task
	EventDb       map[uuid.UUID]*task.TaskEvent
	Workers       []string
	WorkerTaskMap map[string][]uuid.UUID
	TaskWorkerMap map[uuid.UUID]string
	LastWorker    int
	Logger        *log.Logger
	client        *http.Client
}

func NewManager(l *log.Logger, c *http.Client, workers []string) (*Manager, error) {
	m := &Manager{
		TaskDb:        make(map[uuid.UUID]*task.Task),
		EventDb:       make(map[uuid.UUID]*task.TaskEvent),
		Workers:       workers,
		WorkerTaskMap: make(map[string][]uuid.UUID),
		TaskWorkerMap: make(map[uuid.UUID]string),
		LastWorker:    0,
		Logger:        l,
		client:        c,
	}

	for w := range workers {
		m.WorkerTaskMap[workers[w]] = []uuid.UUID{}
	}

	return m, m.validate()
}

func (m *Manager) validate() error {
	if m.Logger == nil {
		return fmt.Errorf("logger is nil")
	}

	if m.client == nil {
		return fmt.Errorf("http client is nil")
	}

	if len(m.Workers) == 0 {
		return fmt.Errorf("no worker provided cannot start the manager")
	}

	return nil
}

func (m *Manager) AddTasks(te task.TaskEvent) {
	m.Pending.Enqueue(te)
}

func (m *Manager) SelectWorker() string {
	if m.LastWorker == len(m.Workers)-1 {
		m.LastWorker = 0
		return m.Workers[m.LastWorker]
	}

	m.LastWorker++
	return m.Workers[m.LastWorker]
}

func (m *Manager) updateTasks() {
	for _, w := range m.Workers {
		m.Logger.Printf("checking worker: %v for the task updates", w)

		resp, err := m.client.Get(fmt.Sprintf("http://%v/tasks", w))
		if err != nil {
			m.Logger.Printf("error making a request: %v\n", err)
		}
		if resp.StatusCode != http.StatusOK {
			m.Logger.Printf("error fetching tasks, resp code: %v\n", resp.StatusCode)
			continue
		}
		var te []*task.Task
		if err := json.NewDecoder(resp.Body).Decode(&te); err != nil {
			log.Printf("error decoding tasks: %v\n", err)
			continue
		}

		for _, t := range te {
			taskFromDB, ok := m.TaskDb[t.ID]
			if !ok {
				m.Logger.Fatal("task not found in the db: %v", t.ID)
			}

			if taskFromDB.State != t.State {
				taskFromDB.State = t.State
			}

			taskFromDB.StartTime = t.StartTime
			taskFromDB.FinishTime = t.FinishTime
			taskFromDB.ContainerID = t.ContainerID
			m.TaskDb[taskFromDB.ID] = taskFromDB
		}
	}
}

func (m *Manager) SendWork() {
	if m.Pending.Len() == 0 {
		m.Logger.Println("no pending tasks to run in the manager")
		return
	}

	e := m.Pending.Dequeue()
	taskEvent, ok := e.(task.TaskEvent)
	if !ok {
		m.Logger.Printf("invalid type found, expected task event")
		return
	}
	t := taskEvent.Task
	log.Printf("pulled %v off pending queue\n", t)

	m.EventDb[taskEvent.ID] = &taskEvent
	w := m.SelectWorker()
	m.WorkerTaskMap[w] = append(m.WorkerTaskMap[w], t.ID)
	m.TaskWorkerMap[t.ID] = w

	t.State = task.Scheduled
	m.TaskDb[t.ID] = &t

	data, err := json.Marshal(taskEvent)
	if err != nil {
		m.Logger.Printf("unable to send marshal task object: %v.\n", taskEvent)
		return
	}

	u := url2.URL{
		Scheme: "http",
		Host:   w,
		Path:   "tasks",
	}
	resp, err := m.client.Post(u.String(), "application/json", bytes.NewBuffer(data))
	if err != nil {
		m.Logger.Printf("error connecting to url: %q, err: %v\n.", u.String(), err)
		m.AddTasks(taskEvent)
		return
	}

	d := json.NewDecoder(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		e := worker.ErrorResponse{}
		err := d.Decode(&e)
		if err != nil {
			m.Logger.Printf("error decoding: %v", err)
			return
		}
		m.Logger.Printf("response error (%d): %s", e.HTTPStatusCode, e.Message)
		return
	}

	t = task.Task{}
	err = d.Decode(&t)
	if err != nil {
		m.Logger.Printf("error decoding response: %s\n", err.Error())
		return
	}
	m.Logger.Printf("%#v\n", t)
}

func (m *Manager) GetAllTasks() []*task.Task {
	tasks := make([]*task.Task, 0, len(m.TaskDb))
	for _, t := range m.TaskDb {
		tasks = append(tasks, t)
	}

	return tasks
}

func (m *Manager) UpdateTasks() {
	for {
		m.Logger.Println("checking for task updates from workers")
		m.updateTasks()
		m.Logger.Printf("task updates completed")
		m.Logger.Printf("sleeping for 10 seconds")
		time.Sleep(10 * time.Second)
	}
}

func (m *Manager) ProcessTasks() {
	for {
		m.Logger.Println("processing tasks in the queue")
		m.SendWork()
		m.Logger.Println("sleeping for 10 seconds")
		time.Sleep(10 * time.Second)
	}
}
