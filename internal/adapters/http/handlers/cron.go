package handlers

import (
	"mindx/internal/usecase/cron"
	"net/http"

	"github.com/gin-gonic/gin"
)

type CronHandler struct {
	scheduler cron.Scheduler
}

func NewCronHandler(scheduler cron.Scheduler) *CronHandler {
	return &CronHandler{scheduler: scheduler}
}

func (h *CronHandler) RegisterRoutes(api *gin.RouterGroup) {
	cronGroup := api.Group("/cron")
	{
		cronGroup.GET("/jobs", h.listJobs)
		cronGroup.GET("/jobs/:id", h.getJob)
		cronGroup.POST("/jobs", h.addJob)
		cronGroup.PUT("/jobs/:id", h.updateJob)
		cronGroup.DELETE("/jobs/:id", h.deleteJob)
		cronGroup.POST("/jobs/:id/pause", h.pauseJob)
		cronGroup.POST("/jobs/:id/resume", h.resumeJob)
	}
}

func (h *CronHandler) listJobs(c *gin.Context) {
	jobs, err := h.scheduler.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}

func (h *CronHandler) getJob(c *gin.Context) {
	id := c.Param("id")
	job, err := h.scheduler.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, job)
}

func (h *CronHandler) addJob(c *gin.Context) {
	var job cron.Job
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id, err := h.scheduler.Add(&job)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h *CronHandler) updateJob(c *gin.Context) {
	id := c.Param("id")
	var job cron.Job
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.scheduler.Update(id, &job); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Job updated"})
}

func (h *CronHandler) deleteJob(c *gin.Context) {
	id := c.Param("id")
	if err := h.scheduler.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Job deleted"})
}

func (h *CronHandler) pauseJob(c *gin.Context) {
	id := c.Param("id")
	if err := h.scheduler.Pause(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Job paused"})
}

func (h *CronHandler) resumeJob(c *gin.Context) {
	id := c.Param("id")
	if err := h.scheduler.Resume(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Job resumed"})
}
