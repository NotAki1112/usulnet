// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024-2026 usulnet contributors
// https://github.com/fr4nsys/usulnet

package web

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fr4nsys/usulnet/internal/models"
)

// calendarJSON writes a JSON response with {data: ...} envelope.
func calendarJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"data": data})
}

// calendarError writes a JSON error response.
func calendarError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"error": message})
}

// calendarUserID extracts and parses the user ID from the web session context.
func calendarUserID(r *http.Request) (uuid.UUID, bool) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		return uuid.Nil, false
	}
	uid, err := uuid.Parse(user.ID)
	if err != nil {
		return uuid.Nil, false
	}
	return uid, true
}

// ============================================================================
// Event Handlers
// ============================================================================

// CalendarListEvents returns events for a given year/month.
func (h *Handler) CalendarListEvents(w http.ResponseWriter, r *http.Request) {
	userID, ok := calendarUserID(r)
	if !ok {
		calendarError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	month, _ := strconv.Atoi(r.URL.Query().Get("month"))
	if year == 0 || month == 0 {
		calendarError(w, http.StatusBadRequest, "year and month are required")
		return
	}

	events, err := h.calendarSvc.ListEventsByMonth(r.Context(), userID, year, month)
	if err != nil {
		slog.Error("calendar: list events", "error", err)
		calendarError(w, http.StatusInternalServerError, "failed to load events")
		return
	}
	if events == nil {
		events = []*models.CalendarEvent{}
	}
	calendarJSON(w, http.StatusOK, events)
}

// CalendarCreateEvent creates a new calendar event.
func (h *Handler) CalendarCreateEvent(w http.ResponseWriter, r *http.Request) {
	userID, ok := calendarUserID(r)
	if !ok {
		calendarError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		EventDate   string `json:"event_date"`
		EventTime   string `json:"event_time"`
		Color       string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		calendarError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Title == "" || req.EventDate == "" {
		calendarError(w, http.StatusBadRequest, "title and event_date are required")
		return
	}

	ev := &models.CalendarEvent{
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
		EventDate:   req.EventDate,
		EventTime:   req.EventTime,
		Color:       req.Color,
	}
	if err := h.calendarSvc.CreateEvent(r.Context(), ev); err != nil {
		slog.Error("calendar: create event", "error", err)
		calendarError(w, http.StatusInternalServerError, "failed to create event")
		return
	}
	calendarJSON(w, http.StatusCreated, ev)
}

// CalendarUpdateEvent updates a calendar event.
func (h *Handler) CalendarUpdateEvent(w http.ResponseWriter, r *http.Request) {
	userID, ok := calendarUserID(r)
	if !ok {
		calendarError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		calendarError(w, http.StatusBadRequest, "invalid event ID")
		return
	}

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		EventDate   string `json:"event_date"`
		EventTime   string `json:"event_time"`
		Color       string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		calendarError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	ev := &models.CalendarEvent{
		ID:          id,
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
		EventDate:   req.EventDate,
		EventTime:   req.EventTime,
		Color:       req.Color,
	}
	if err := h.calendarSvc.UpdateEvent(r.Context(), ev); err != nil {
		slog.Error("calendar: update event", "error", err)
		calendarError(w, http.StatusInternalServerError, "failed to update event")
		return
	}
	calendarJSON(w, http.StatusOK, ev)
}

// CalendarDeleteEvent deletes a calendar event.
func (h *Handler) CalendarDeleteEvent(w http.ResponseWriter, r *http.Request) {
	userID, ok := calendarUserID(r)
	if !ok {
		calendarError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		calendarError(w, http.StatusBadRequest, "invalid event ID")
		return
	}

	if err := h.calendarSvc.DeleteEvent(r.Context(), id, userID); err != nil {
		slog.Error("calendar: delete event", "error", err)
		calendarError(w, http.StatusInternalServerError, "failed to delete event")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ============================================================================
// Task Handlers
// ============================================================================

// CalendarListTasks returns tasks with optional filter.
func (h *Handler) CalendarListTasks(w http.ResponseWriter, r *http.Request) {
	userID, ok := calendarUserID(r)
	if !ok {
		calendarError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "all"
	}

	tasks, err := h.calendarSvc.ListTasks(r.Context(), userID, filter)
	if err != nil {
		slog.Error("calendar: list tasks", "error", err)
		calendarError(w, http.StatusInternalServerError, "failed to load tasks")
		return
	}
	if tasks == nil {
		tasks = []*models.CalendarTask{}
	}
	calendarJSON(w, http.StatusOK, tasks)
}

// CalendarCreateTask creates a new calendar task.
func (h *Handler) CalendarCreateTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := calendarUserID(r)
	if !ok {
		calendarError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Text     string  `json:"text"`
		Priority string  `json:"priority"`
		DueDate  *string `json:"due_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		calendarError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Text == "" {
		calendarError(w, http.StatusBadRequest, "text is required")
		return
	}
	if req.Priority == "" {
		req.Priority = "normal"
	}

	t := &models.CalendarTask{
		UserID:   userID,
		Text:     req.Text,
		Priority: req.Priority,
		DueDate:  req.DueDate,
	}
	if err := h.calendarSvc.CreateTask(r.Context(), t); err != nil {
		slog.Error("calendar: create task", "error", err)
		calendarError(w, http.StatusInternalServerError, "failed to create task")
		return
	}
	calendarJSON(w, http.StatusCreated, t)
}

// CalendarToggleTask toggles the done status of a task.
func (h *Handler) CalendarToggleTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := calendarUserID(r)
	if !ok {
		calendarError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		calendarError(w, http.StatusBadRequest, "invalid task ID")
		return
	}

	if err := h.calendarSvc.ToggleTask(r.Context(), id, userID); err != nil {
		slog.Error("calendar: toggle task", "error", err)
		calendarError(w, http.StatusInternalServerError, "failed to toggle task")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CalendarDeleteTask deletes a calendar task.
func (h *Handler) CalendarDeleteTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := calendarUserID(r)
	if !ok {
		calendarError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		calendarError(w, http.StatusBadRequest, "invalid task ID")
		return
	}

	if err := h.calendarSvc.DeleteTask(r.Context(), id, userID); err != nil {
		slog.Error("calendar: delete task", "error", err)
		calendarError(w, http.StatusInternalServerError, "failed to delete task")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ============================================================================
// Note Handlers
// ============================================================================

// CalendarListNotes returns notes for the current user.
func (h *Handler) CalendarListNotes(w http.ResponseWriter, r *http.Request) {
	userID, ok := calendarUserID(r)
	if !ok {
		calendarError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	notes, err := h.calendarSvc.ListNotes(r.Context(), userID)
	if err != nil {
		slog.Error("calendar: list notes", "error", err)
		calendarError(w, http.StatusInternalServerError, "failed to load notes")
		return
	}
	if notes == nil {
		notes = []*models.CalendarNote{}
	}
	calendarJSON(w, http.StatusOK, notes)
}

// CalendarCreateNote creates a new calendar note.
func (h *Handler) CalendarCreateNote(w http.ResponseWriter, r *http.Request) {
	userID, ok := calendarUserID(r)
	if !ok {
		calendarError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		calendarError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Title == "" {
		calendarError(w, http.StatusBadRequest, "title is required")
		return
	}

	n := &models.CalendarNote{
		UserID:  userID,
		Title:   req.Title,
		Content: req.Content,
	}
	if err := h.calendarSvc.CreateNote(r.Context(), n); err != nil {
		slog.Error("calendar: create note", "error", err)
		calendarError(w, http.StatusInternalServerError, "failed to create note")
		return
	}
	calendarJSON(w, http.StatusCreated, n)
}

// CalendarUpdateNote updates a calendar note.
func (h *Handler) CalendarUpdateNote(w http.ResponseWriter, r *http.Request) {
	userID, ok := calendarUserID(r)
	if !ok {
		calendarError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		calendarError(w, http.StatusBadRequest, "invalid note ID")
		return
	}

	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		calendarError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	n := &models.CalendarNote{
		ID:      id,
		UserID:  userID,
		Title:   req.Title,
		Content: req.Content,
	}
	if err := h.calendarSvc.UpdateNote(r.Context(), n); err != nil {
		slog.Error("calendar: update note", "error", err)
		calendarError(w, http.StatusInternalServerError, "failed to update note")
		return
	}
	calendarJSON(w, http.StatusOK, n)
}

// CalendarDeleteNote deletes a calendar note.
func (h *Handler) CalendarDeleteNote(w http.ResponseWriter, r *http.Request) {
	userID, ok := calendarUserID(r)
	if !ok {
		calendarError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		calendarError(w, http.StatusBadRequest, "invalid note ID")
		return
	}

	if err := h.calendarSvc.DeleteNote(r.Context(), id, userID); err != nil {
		slog.Error("calendar: delete note", "error", err)
		calendarError(w, http.StatusInternalServerError, "failed to delete note")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ============================================================================
// Checklist Handlers
// ============================================================================

// CalendarListChecklists returns checklists for the current user.
func (h *Handler) CalendarListChecklists(w http.ResponseWriter, r *http.Request) {
	userID, ok := calendarUserID(r)
	if !ok {
		calendarError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	checklists, err := h.calendarSvc.ListChecklists(r.Context(), userID)
	if err != nil {
		slog.Error("calendar: list checklists", "error", err)
		calendarError(w, http.StatusInternalServerError, "failed to load checklists")
		return
	}
	if checklists == nil {
		checklists = []*models.CalendarChecklist{}
	}
	calendarJSON(w, http.StatusOK, checklists)
}

// CalendarCreateChecklist creates a new calendar checklist.
func (h *Handler) CalendarCreateChecklist(w http.ResponseWriter, r *http.Request) {
	userID, ok := calendarUserID(r)
	if !ok {
		calendarError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Title string          `json:"title"`
		Items json.RawMessage `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		calendarError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Title == "" {
		calendarError(w, http.StatusBadRequest, "title is required")
		return
	}
	if req.Items == nil {
		req.Items = json.RawMessage(`[]`)
	}

	cl := &models.CalendarChecklist{
		UserID: userID,
		Title:  req.Title,
		Items:  req.Items,
	}
	if err := h.calendarSvc.CreateChecklist(r.Context(), cl); err != nil {
		slog.Error("calendar: create checklist", "error", err)
		calendarError(w, http.StatusInternalServerError, "failed to create checklist")
		return
	}
	calendarJSON(w, http.StatusCreated, cl)
}

// CalendarUpdateChecklist updates a calendar checklist.
func (h *Handler) CalendarUpdateChecklist(w http.ResponseWriter, r *http.Request) {
	userID, ok := calendarUserID(r)
	if !ok {
		calendarError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		calendarError(w, http.StatusBadRequest, "invalid checklist ID")
		return
	}

	var req struct {
		Title string          `json:"title"`
		Items json.RawMessage `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		calendarError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	cl := &models.CalendarChecklist{
		ID:     id,
		UserID: userID,
		Title:  req.Title,
		Items:  req.Items,
	}
	if err := h.calendarSvc.UpdateChecklist(r.Context(), cl); err != nil {
		slog.Error("calendar: update checklist", "error", err)
		calendarError(w, http.StatusInternalServerError, "failed to update checklist")
		return
	}
	calendarJSON(w, http.StatusOK, cl)
}

// CalendarDeleteChecklist deletes a calendar checklist.
func (h *Handler) CalendarDeleteChecklist(w http.ResponseWriter, r *http.Request) {
	userID, ok := calendarUserID(r)
	if !ok {
		calendarError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		calendarError(w, http.StatusBadRequest, "invalid checklist ID")
		return
	}

	if err := h.calendarSvc.DeleteChecklist(r.Context(), id, userID); err != nil {
		slog.Error("calendar: delete checklist", "error", err)
		calendarError(w, http.StatusInternalServerError, "failed to delete checklist")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
