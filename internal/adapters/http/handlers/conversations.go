package handlers

import (
	"mindx/internal/entity"
	sessionMgr "mindx/internal/usecase/session"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type ConversationsHandler struct {
	sessionMgr *sessionMgr.SessionMgr
}

type CurrentSessionResponse struct {
	ID       string                  `json:"id"`
	Messages []ConversationMsgDetail `json:"messages"`
}

type ConversationSummary struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Timestamp    int64  `json:"timestamp"`
	MessageCount int    `json:"messageCount"`
	StartTime    string `json:"start_time,omitempty"`
}

type ConversationDetail struct {
	ID       string                  `json:"id"`
	Messages []ConversationMsgDetail `json:"messages"`
}

type ConversationMsgDetail struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func NewConversationsHandler(sessionMgr *sessionMgr.SessionMgr) *ConversationsHandler {
	return &ConversationsHandler{
		sessionMgr: sessionMgr,
	}
}

func (h *ConversationsHandler) listConversations(c *gin.Context) {
	limit := 100
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	sessions, err := h.sessionMgr.GetAllSessions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取对话列表失败"})
		return
	}

	conversations := h.convertToSummaries(sessions)

	if len(conversations) > limit {
		conversations = conversations[:limit]
	}

	c.JSON(http.StatusOK, conversations)
}

func (h *ConversationsHandler) getConversation(c *gin.Context) {
	id := c.Param("id")

	sessions, err := h.sessionMgr.GetAllSessions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取对话失败"})
		return
	}

	var targetSession *entity.Session
	for _, sess := range sessions {
		if sess.ID == id {
			targetSession = &sess
			break
		}
	}

	if targetSession == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "对话不存在"})
		return
	}

	detail := h.convertToDetail(targetSession)
	c.JSON(http.StatusOK, detail)
}

func (h *ConversationsHandler) getCurrentSession(c *gin.Context) {
	currentSession, exists := h.sessionMgr.GetCurrentSession()
	if !exists || currentSession == nil {
		c.JSON(http.StatusOK, gin.H{
			"id":       "",
			"messages": []ConversationMsgDetail{},
		})
		return
	}

	msgDetails := make([]ConversationMsgDetail, len(currentSession.Messages))
	for i, msg := range currentSession.Messages {
		msgDetails[i] = ConversationMsgDetail{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	c.JSON(http.StatusOK, CurrentSessionResponse{
		ID:       currentSession.ID,
		Messages: msgDetails,
	})
}

func (h *ConversationsHandler) deleteConversation(c *gin.Context) {
	id := c.Param("id")

	err := h.sessionMgr.DeleteSession(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除对话失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "对话已删除"})
}

func (h *ConversationsHandler) switchConversation(c *gin.Context) {
	id := c.Param("id")

	session, err := h.sessionMgr.SwitchSession(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "切换对话失败"})
		return
	}

	msgDetails := make([]ConversationMsgDetail, len(session.Messages))
	for i, msg := range session.Messages {
		msgDetails[i] = ConversationMsgDetail{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	c.JSON(http.StatusOK, CurrentSessionResponse{
		ID:       session.ID,
		Messages: msgDetails,
	})
}

func (h *ConversationsHandler) createNewConversation(c *gin.Context) {
	session, err := h.sessionMgr.CreateNewSession()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建新对话失败"})
		return
	}

	c.JSON(http.StatusOK, CurrentSessionResponse{
		ID:       session.ID,
		Messages: []ConversationMsgDetail{},
	})
}

func (h *ConversationsHandler) convertToSummaries(sessions []entity.Session) []ConversationSummary {
	summaries := make([]ConversationSummary, 0, len(sessions))

	for _, sess := range sessions {
		if len(sess.Messages) == 0 {
			continue
		}

		title := extractTitleFromMessage(sess.Messages[0].Content)
		summaries = append(summaries, ConversationSummary{
			ID:           sess.ID,
			Title:        title,
			Timestamp:    sess.Messages[0].Time.Unix(),
			MessageCount: len(sess.Messages),
			StartTime:    sess.Messages[0].Time.Format(time.RFC3339),
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Timestamp > summaries[j].Timestamp
	})

	return summaries
}

func (h *ConversationsHandler) convertToDetail(session *entity.Session) ConversationDetail {
	msgDetails := make([]ConversationMsgDetail, len(session.Messages))
	for i, msg := range session.Messages {
		msgDetails[i] = ConversationMsgDetail{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	return ConversationDetail{
		ID:       session.ID,
		Messages: msgDetails,
	}
}

func extractTitleFromMessage(content string) string {
	if len(content) > 50 {
		return content[:50] + "..."
	}
	return content
}
