package manager

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/reversearrow/orchestrator/task"
)

type Api struct {
	Address string
	Port    int
	Manager *Manager
	Router  *chi.Mux
	Logger  *log.Logger
}

func NewApi(l *log.Logger, m *Manager, address string, port int) (*Api, error) {
	a := &Api{
		Logger:  l,
		Manager: m,
		Address: address,
		Port:    port,
	}
	return a, a.validate()
}

func (a *Api) validate() error {
	if a.Address == "" {
		return fmt.Errorf("invalid manager address")
	}

	if a.Port == 0 {
		return fmt.Errorf("invalid port")
	}

	if a.Logger == nil {
		return fmt.Errorf("logger is nil")
	}

	if a.Manager == nil {
		return fmt.Errorf("manager is nil")
	}

	if a.Logger == nil {
		return fmt.Errorf("logger is nil")
	}

	return nil
}

type ErrorResponse struct {
	HTTPStatusCode int
	Message        string
}

func (a *Api) StartTaskHandler(w http.ResponseWriter, r *http.Request) {
	var te task.TaskEvent
	if err := json.NewDecoder(r.Body).Decode(&te); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		msg := "failed to decode the request body"
		a.Logger.Printf("%s: %v", msg, err)
		json.NewEncoder(w).Encode(
			ErrorResponse{
				HTTPStatusCode: http.StatusBadRequest,
				Message:        msg,
			})
		return
	}

	a.Manager.AddTasks(te)
	a.Logger.Println("task added to the queue")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(te)
}

func (a *Api) GetTasksHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(a.Manager.GetAllTasks())
}

func (a *Api) StopTaskHandler(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if taskID == "" {
		a.Logger.Println("no task id found in the request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tID, _ := uuid.Parse(taskID)
	taskToStop, ok := a.Manager.TaskDb[tID]
	if !ok {
		a.Logger.Printf("task %v not found.\n", tID)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	te := task.TaskEvent{
		ID:        uuid.New(),
		State:     task.Completed,
		Timestamp: time.Now().UTC(),
	}

	taskCopy := *taskToStop
	taskCopy.State = task.Completed
	te.Task = taskCopy

	a.Manager.AddTasks(te)
	a.Logger.Printf("added task event to stop the task")
	w.WriteHeader(http.StatusNoContent)
}

func (a *Api) initRouter() {
	a.Router = chi.NewRouter()
	a.Router.Route("/tasks", func(r chi.Router) {
		r.Post("/", a.StartTaskHandler)
		r.Get("/", a.GetTasksHandler)
		r.Route("/{taskID}", func(r chi.Router) {
			r.Delete("/", a.StopTaskHandler)
		})
	})
}

func (a *Api) Start() {
	a.initRouter()
	a.Logger.Printf("attempting to start the manager: %s:%d\n", a.Address, a.Port)

	go func() {
		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", a.Address, a.Port), a.Router); err != nil {
			a.Logger.Printf("failed to start the worker api: %v", err)
		}
	}()

	a.Logger.Printf("server started on %s:%d\n", a.Address, a.Port)
}
