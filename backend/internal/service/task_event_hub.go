package service

import (
	"encoding/json"
	"sync"
	"time"

	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"
)

const (
	taskEventTypeStatus   = "status"
	taskEventTypeStage    = "stage"
	taskEventTypeLog      = "log"
	taskEventTypeSnapshot = "snapshot"
	taskEventTypeError    = "error"
)

type TaskEvent struct {
	ID        int64          `json:"id"`
	TaskID    uint64         `json:"task_id"`
	Type      string         `json:"type"`
	Stage     string         `json:"stage,omitempty"`
	Status    string         `json:"status,omitempty"`
	Message   string         `json:"message,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data,omitempty"`
}

type TaskEventHub struct {
	mu      sync.RWMutex
	streams map[uint64]*taskEventStream
	repo    *repository.TaskEventLogRepo
	seqMap  map[uint64]int
}

type taskEventStream struct {
	nextID      int64
	history     []TaskEvent
	subscribers map[chan TaskEvent]struct{}
}

const maxTaskEventHistory = 200

var DefaultTaskEventHub = NewTaskEventHub()

func NewTaskEventHub() *TaskEventHub {
	return &TaskEventHub{
		streams: make(map[uint64]*taskEventStream),
		seqMap:  make(map[uint64]int),
	}
}

func (h *TaskEventHub) SetRepo(repo *repository.TaskEventLogRepo) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.repo = repo
}

func (h *TaskEventHub) Publish(taskID uint64, event TaskEvent) {
	if taskID == 0 {
		return
	}

	h.mu.Lock()
	stream := h.ensureStreamLocked(taskID)
	stream.nextID++
	event.ID = stream.nextID
	event.TaskID = taskID
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	stream.history = append(stream.history, event)
	if len(stream.history) > maxTaskEventHistory {
		stream.history = append([]TaskEvent(nil), stream.history[len(stream.history)-maxTaskEventHistory:]...)
	}

	h.seqMap[taskID]++
	seq := h.seqMap[taskID]

	var dataJSON model.JSON
	if len(event.Data) > 0 {
		if b, err := json.Marshal(event.Data); err == nil {
			dataJSON = b
		}
	}

	logEntry := &model.TaskEventLog{
		TaskID:  taskID,
		Seq:     seq,
		Type:    event.Type,
		Stage:   event.Stage,
		Status:  event.Status,
		Message: event.Message,
		Data:    dataJSON,
	}

	subs := make([]chan TaskEvent, 0, len(stream.subscribers))
	for ch := range stream.subscribers {
		subs = append(subs, ch)
	}
	repo := h.repo
	h.mu.Unlock()

	if repo != nil {
		go func() {
			_ = repo.Create(logEntry)
		}()
	}

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
		}
	}
}

func (h *TaskEventHub) Subscribe(taskID uint64) ([]TaskEvent, chan TaskEvent, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	stream := h.ensureStreamLocked(taskID)
	ch := make(chan TaskEvent, 32)
	stream.subscribers[ch] = struct{}{}
	history := append([]TaskEvent(nil), stream.history...)

	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if current, ok := h.streams[taskID]; ok {
			delete(current.subscribers, ch)
			close(ch)
			if len(current.subscribers) == 0 && len(current.history) == 0 {
				delete(h.streams, taskID)
			}
		}
	}

	return history, ch, cancel
}

func (h *TaskEventHub) GetHistory(taskID uint64) []TaskEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stream, ok := h.streams[taskID]
	if !ok {
		return nil
	}
	return append([]TaskEvent(nil), stream.history...)
}

func (h *TaskEventHub) GetLogsFromDB(taskID uint64) ([]TaskEvent, error) {
	h.mu.RLock()
	repo := h.repo
	h.mu.RUnlock()

	if repo == nil {
		return nil, nil
	}

	logs, err := repo.ListByTaskID(taskID)
	if err != nil {
		return nil, err
	}

	events := make([]TaskEvent, 0, len(logs))
	for _, log := range logs {
		var data map[string]any
		if len(log.Data) > 0 {
			_ = json.Unmarshal(log.Data, &data)
		}
		events = append(events, TaskEvent{
			ID:        int64(log.Seq),
			TaskID:    log.TaskID,
			Type:      log.Type,
			Stage:     log.Stage,
			Status:    log.Status,
			Message:   log.Message,
			Timestamp: log.CreatedAt,
			Data:      data,
		})
	}
	return events, nil
}

func (h *TaskEventHub) ensureStreamLocked(taskID uint64) *taskEventStream {
	stream, ok := h.streams[taskID]
	if !ok {
		stream = &taskEventStream{
			history:     make([]TaskEvent, 0, 32),
			subscribers: make(map[chan TaskEvent]struct{}),
		}
		h.streams[taskID] = stream
	}
	return stream
}
