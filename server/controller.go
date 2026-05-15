package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Controller struct {
	service *Service
}

func NewController(service *Service) *Controller {
	return &Controller{service: service}
}

func NewRouter(ctrl *Controller) *gin.Engine {
	r := gin.Default()
	r.Use(corsMiddleware())

	api := r.Group("/api")
	{
		api.POST("/conversations", ctrl.createConversation)
		api.GET("/conversations", ctrl.listConversations)
		api.GET("/conversations/:conversation_id", ctrl.getConversation)
		api.GET("/conversations/:conversation_id/messages", ctrl.listMessages)
		api.PATCH("/conversations/:conversation_id", ctrl.updateConversation)
		api.DELETE("/conversations/:conversation_id", ctrl.deleteConversation)
		api.POST("/conversations/:conversation_id/messages", ctrl.sendMessage)
	}

	r.Static("/assets", "./frontend/dist/assets")
	r.NoRoute(func(c *gin.Context) {
		c.File("./frontend/dist/index.html")
	})

	return r
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

type APIResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

func successResponse(data interface{}) APIResponse {
	return APIResponse{Code: 0, Data: data}
}

func errorResponse(msg string) APIResponse {
	return APIResponse{Code: 1, Msg: msg}
}

type CreateConversationRequest struct {
	Title string `json:"title"`
}

type SendMessageRequest struct {
	ParentMessageID string `json:"parent_message_id"`
	Content         string `json:"content"`
}

func (ctrl *Controller) createConversation(c *gin.Context) {
	var req CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
		return
	}

	if req.Title == "" {
		req.Title = "New Conversation"
	}

	conv, err := ctrl.service.CreateConversation(c.Request.Context(), req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, successResponse(conv))
}

func (ctrl *Controller) listConversations(c *gin.Context) {
	convs, err := ctrl.service.ListConversations(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, successResponse(convs))
}

func (ctrl *Controller) getConversation(c *gin.Context) {
	conversationID := c.Param("conversation_id")

	conv, err := ctrl.service.GetConversation(c.Request.Context(), conversationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, successResponse(conv))
}

type UpdateConversationRequest struct {
	Title    *string `json:"title"`
	Archived *bool   `json:"archived"`
}

func (ctrl *Controller) updateConversation(c *gin.Context) {
	conversationID := c.Param("conversation_id")

	var req UpdateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
		return
	}

	conv, err := ctrl.service.UpdateConversation(c.Request.Context(), conversationID, req.Title, req.Archived)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, successResponse(conv))
}

func (ctrl *Controller) deleteConversation(c *gin.Context) {
	conversationID := c.Param("conversation_id")

	if err := ctrl.service.DeleteConversation(c.Request.Context(), conversationID); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, successResponse(gin.H{"message": "deleted"}))
}

func (ctrl *Controller) listMessages(c *gin.Context) {
	conversationID := c.Param("conversation_id")

	messages, err := ctrl.service.ListMessages(c.Request.Context(), conversationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, successResponse(messages))
}

func (ctrl *Controller) sendMessage(c *gin.Context) {
	conversationID := c.Param("conversation_id")

	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	eventCh := make(chan StreamEvent, 100)

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	go func() {
		defer close(eventCh)
		_, err := ctrl.service.SendMessage(ctx, conversationID, req.ParentMessageID, req.Content, eventCh)
		if err != nil {
			eventCh <- StreamEvent{Event: "error", Content: err.Error()}
		}
	}()

	c.Stream(func(w io.Writer) bool {
		select {
		case <-c.Request.Context().Done():
			return false
		case event, ok := <-eventCh:
			if !ok {
				return false
			}
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", data)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return true
		}
	})
}
