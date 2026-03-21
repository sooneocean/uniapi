package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sooneocean/uniapi/internal/provider"
	"github.com/sooneocean/uniapi/internal/router"
)

type RoomsHandler struct {
	db     *sql.DB
	router *router.Router
	hub    *RoomHub
}

func NewRoomsHandler(db *sql.DB, rtr *router.Router) *RoomsHandler {
	return &RoomsHandler{db: db, router: rtr, hub: NewRoomHub()}
}

type ChatRoom struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedBy string `json:"created_by"`
	CreatedAt string `json:"created_at"`
}

type RoomMessage struct {
	ID        string `json:"id"`
	RoomID    string `json:"room_id"`
	UserID    string `json:"user_id,omitempty"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Model     string `json:"model,omitempty"`
	CreatedAt string `json:"created_at"`
}

func (h *RoomsHandler) Create(c *gin.Context) {
	userID := mustUserID(c)
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	id := uuid.New().String()
	_, err := h.db.Exec("INSERT INTO chat_rooms (id, name, created_by) VALUES (?,?,?)", id, req.Name, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Auto-join creator
	h.db.Exec("INSERT OR IGNORE INTO chat_room_members (room_id, user_id) VALUES (?,?)", id, userID)
	c.JSON(http.StatusCreated, gin.H{"id": id, "name": req.Name, "created_by": userID})
}

func (h *RoomsHandler) List(c *gin.Context) {
	userID := mustUserID(c)
	rows, err := h.db.Query(`
		SELECT cr.id, cr.name, cr.created_by, cr.created_at
		FROM chat_rooms cr
		JOIN chat_room_members crm ON crm.room_id = cr.id
		WHERE crm.user_id = ?
		ORDER BY cr.created_at DESC`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var rooms []ChatRoom
	for rows.Next() {
		var r ChatRoom
		rows.Scan(&r.ID, &r.Name, &r.CreatedBy, &r.CreatedAt)
		rooms = append(rooms, r)
	}
	if rooms == nil {
		rooms = []ChatRoom{}
	}
	c.JSON(http.StatusOK, rooms)
}

func (h *RoomsHandler) Join(c *gin.Context) {
	userID := mustUserID(c)
	roomID := c.Param("id")
	// Check room exists
	var count int
	h.db.QueryRow("SELECT COUNT(*) FROM chat_rooms WHERE id = ?", roomID).Scan(&count)
	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}
	h.db.Exec("INSERT OR IGNORE INTO chat_room_members (room_id, user_id) VALUES (?,?)", roomID, userID)
	c.JSON(http.StatusOK, gin.H{"joined": true})
}

func (h *RoomsHandler) GetMessages(c *gin.Context) {
	userID := mustUserID(c)
	roomID := c.Param("id")
	if !h.isMember(roomID, userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member"})
		return
	}
	rows, err := h.db.Query(`
		SELECT id, room_id, COALESCE(user_id,''), username, role, content, COALESCE(model,''), created_at
		FROM chat_room_messages WHERE room_id = ? ORDER BY created_at ASC LIMIT 100`, roomID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var msgs []RoomMessage
	for rows.Next() {
		var m RoomMessage
		rows.Scan(&m.ID, &m.RoomID, &m.UserID, &m.Username, &m.Role, &m.Content, &m.Model, &m.CreatedAt)
		msgs = append(msgs, m)
	}
	if msgs == nil {
		msgs = []RoomMessage{}
	}
	c.JSON(http.StatusOK, msgs)
}

func (h *RoomsHandler) SendMessage(c *gin.Context) {
	userID := mustUserID(c)
	roomID := c.Param("id")
	if !h.isMember(roomID, userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member"})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
		Model   string `json:"model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get username
	username := ""
	h.db.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username)
	if username == "" {
		username = "user"
	}

	// Store user message
	msgID := uuid.New().String()
	now := time.Now().UTC()
	h.db.Exec("INSERT INTO chat_room_messages (id, room_id, user_id, username, role, content) VALUES (?,?,?,?,?,?)",
		msgID, roomID, userID, username, "user", req.Content)

	userMsg := RoomMessage{
		ID: msgID, RoomID: roomID, UserID: userID, Username: username,
		Role: "user", Content: req.Content, CreatedAt: now.Format(time.RFC3339),
	}

	// Broadcast user message via SSE
	if msgJSON, err := json.Marshal(userMsg); err == nil {
		h.hub.Broadcast(roomID, string(msgJSON))
	}

	// Check if AI response is needed
	content := req.Content
	triggerAI := strings.HasPrefix(strings.TrimSpace(content), "@ai") || req.Model != ""
	if !triggerAI {
		c.JSON(http.StatusCreated, gin.H{"message": userMsg})
		return
	}

	// Collect recent room history (last 20 messages)
	histRows, err := h.db.Query(`
		SELECT role, content FROM chat_room_messages
		WHERE room_id = ? AND id != ?
		ORDER BY created_at DESC LIMIT 20`, roomID, msgID)
	if err == nil {
		defer histRows.Close()
	}

	var history []provider.Message
	// Collect in reverse
	var rawHist []struct{ role, content string }
	for histRows != nil && histRows.Next() {
		var role, cont string
		histRows.Scan(&role, &cont)
		rawHist = append(rawHist, struct{ role, content string }{role, cont})
	}
	// Reverse
	for i := len(rawHist) - 1; i >= 0; i-- {
		r := rawHist[i]
		history = append(history, provider.Message{
			Role:    r.role,
			Content: []provider.ContentBlock{{Type: "text", Text: r.content}},
		})
	}
	// Add current user message (strip @ai prefix)
	aiContent := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(content), "@ai"))
	if aiContent == "" {
		aiContent = content
	}
	history = append(history, provider.Message{
		Role:    "user",
		Content: []provider.ContentBlock{{Type: "text", Text: aiContent}},
	})

	model := req.Model
	if model == "" {
		models := h.router.AllModels()
		if len(models) > 0 {
			model = models[0].ID
		}
	}

	chatReq := &provider.ChatRequest{
		Model:    model,
		Messages: history,
	}

	resp, err := h.router.Route(c.Request.Context(), chatReq, userID)
	if err != nil {
		// Still return the user message, just note the AI error
		c.JSON(http.StatusCreated, gin.H{"message": userMsg, "ai_error": err.Error()})
		return
	}

	aiText := ""
	for _, block := range resp.Content {
		if block.Type == "text" {
			aiText += block.Text
		}
	}

	aiMsgID := uuid.New().String()
	aiNow := time.Now().UTC()
	h.db.Exec("INSERT INTO chat_room_messages (id, room_id, username, role, content, model) VALUES (?,?,?,?,?,?)",
		aiMsgID, roomID, resp.Model, "assistant", aiText, resp.Model)

	aiMsg := RoomMessage{
		ID: aiMsgID, RoomID: roomID, Username: resp.Model,
		Role: "assistant", Content: aiText, Model: resp.Model,
		CreatedAt: aiNow.Format(time.RFC3339),
	}

	// Broadcast AI message via SSE
	if aiJSON, err := json.Marshal(aiMsg); err == nil {
		h.hub.Broadcast(roomID, string(aiJSON))
	}

	c.JSON(http.StatusCreated, gin.H{"message": userMsg, "ai_response": aiMsg})
}

func (h *RoomsHandler) Delete(c *gin.Context) {
	userID := mustUserID(c)
	roomID := c.Param("id")
	// Only creator can delete
	var createdBy string
	h.db.QueryRow("SELECT created_by FROM chat_rooms WHERE id = ?", roomID).Scan(&createdBy)
	if createdBy == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}
	if createdBy != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "only creator can delete"})
		return
	}
	h.db.Exec("DELETE FROM chat_rooms WHERE id = ?", roomID)
	c.Status(http.StatusNoContent)
}

func (h *RoomsHandler) GetMembers(c *gin.Context) {
	userID := mustUserID(c)
	roomID := c.Param("id")
	if !h.isMember(roomID, userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member"})
		return
	}
	rows, err := h.db.Query(`
		SELECT u.id, u.username, crm.joined_at
		FROM chat_room_members crm
		JOIN users u ON u.id = crm.user_id
		WHERE crm.room_id = ?`, roomID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var members []gin.H
	for rows.Next() {
		var id, username, joinedAt string
		rows.Scan(&id, &username, &joinedAt)
		members = append(members, gin.H{"id": id, "username": username, "joined_at": joinedAt})
	}
	if members == nil {
		members = []gin.H{}
	}
	c.JSON(http.StatusOK, members)
}

func (h *RoomsHandler) isMember(roomID, userID string) bool {
	var count int
	h.db.QueryRow("SELECT COUNT(*) FROM chat_room_members WHERE room_id = ? AND user_id = ?", roomID, userID).Scan(&count)
	return count > 0
}
