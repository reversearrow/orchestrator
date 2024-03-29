package worker

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/reversearrow/orchestrator/task"
)

type Api struct {
	Address string
	Port    int
	Worker  *Worker
	Router  *chi.Mux
	Logger  *log.Logger
}

func NewAPI(address string, port int, worker *Worker, logger *log.Logger) (*Api, error) {
	a := &Api{
		Address: address,
		Port:    port,
		Worker:  worker,
		Logger:  logger,
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

	if a.Worker == nil {
		return fmt.Errorf("worker is nil")
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

func (a *Api) StartTask(w http.ResponseWriter, r *http.Request) {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()

	te := task.TaskEvent{}
	if err := d.Decode(&te); err != nil {
		msg := fmt.Sprintf("error marshalling body: %v\n", err)
		log.Println(msg)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			HTTPStatusCode: http.StatusBadRequest,
			Message:        msg,
		})
		return
	}

	a.Worker.AddTask(r.Context(), te.Task)
	a.Logger.Printf("added task: %v\n", te.Task.ID)
	if err := json.NewEncoder(w).Encode(te.Task); err != nil {
		log.Printf("error encoding task: %v\n", err)
		return
	}
}

func (a *Api) GetTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(a.Worker.GetTasks())
}

func (a *Api) StopTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if taskID == "" {
		a.Logger.Println("no task id passed in request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tID, err := uuid.Parse(taskID)
	if err != nil {
		a.Logger.Printf("failed to parse task id from the request: %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	_, ok := a.Worker.Db[tID]
	if !ok {
		a.Logger.Printf("task with id: %v not found", tID)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	taskToStop := a.Worker.Db[tID]
	taskCopy := *taskToStop
	taskCopy.State = task.Completed
	a.Worker.AddTask(r.Context(), taskCopy)
	a.Logger.Printf("added task :%v to stop container: %v\n", taskToStop.ID, taskToStop.ContainerID)
	w.WriteHeader(http.StatusNoContent)
}

func (a *Api) GetStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(a.Worker.Stats)
}

func (a *Api) initRouter() {
	a.Router = chi.NewRouter()
	a.Router.Route("/tasks", func(r chi.Router) {
		r.Post("/", a.StartTask)
		r.Get("/", a.GetTasks)
		r.Route("/{taskID}", func(r chi.Router) {
			r.Delete("/", a.StopTask)
		})
	})

	a.Router.Route("/stats", func(r chi.Router) {
		r.Get("/", a.GetStatsHandler)
	})
}

func (a *Api) Start() {
	a.initRouter()
	a.Logger.Printf("attempting to start the worker: %s:%d\n", a.Address, a.Port)

	go func() {
		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", a.Address, a.Port), a.Router); err != nil {
			a.Logger.Printf("failed to start the worker api: %v", err)
		}
	}()

	a.Logger.Printf("server started on %s:%d\n", a.Address, a.Port)
}
