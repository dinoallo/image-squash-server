package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingdie/image-rebase-server/pkg/options"
	"github.com/lingdie/image-rebase-server/pkg/runtime"
)

type Request struct {
	options.Option
}

type SquashStatus string

const (
	SquashStatusPending SquashStatus = "pending"
	SquashStatusSuccess SquashStatus = "success"
	SquashStatusFailed  SquashStatus = "failed"
)

type Response struct {
	Status  SquashStatus `json:"status"`
	Message string       `json:"message"`
	Time    string       `json:"time"`
}

type Server struct {
	router  *gin.Engine
	runtime *runtime.Runtime
}

func NewServer(runtime *runtime.Runtime) *Server {
	return &Server{
		router:  gin.Default(),
		runtime: runtime,
	}
}

func (s *Server) Run() {
	serverAddr := ":8080"
	fmt.Printf("Server starting on %s\n", serverAddr)
	s.router.POST("/squash", s.SquashHandler)
	s.router.Run(serverAddr)
}

func (s *Server) SquashHandler(c *gin.Context) {
	startTime := time.Now()
	request := Request{}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Status:  SquashStatusFailed,
			Message: err.Error(),
			Time:    time.Since(startTime).String(),
		})
		return
	}
	err := s.runtime.Squash(c.Request.Context(), request.Option)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Status:  SquashStatusFailed,
			Message: err.Error(),
			Time:    time.Since(startTime).String(),
		})
		return
	}
	responseData := Response{
		Status:  SquashStatusSuccess,
		Message: fmt.Sprintf("Squash %s to %s success", request.SourceImage, request.TargetImage),
		Time:    time.Since(startTime).String(),
	}
	c.JSON(http.StatusOK, responseData)
}
