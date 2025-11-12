package handler

import (
	"net/http"
	"strconv"
	"time"

	"pagrin/internal/metrics"
	"pagrin/internal/models"
	"pagrin/internal/service"

	"github.com/gin-gonic/gin"
)

type TransferHandler struct {
	service *service.TransferService
}

func NewTransferHandler(service *service.TransferService) *TransferHandler {
	return &TransferHandler{service: service}
}

func (h *TransferHandler) GetTransfers(c *gin.Context) {
	start := time.Now()

	params := models.TransferQueryParams{
		Limit:  100,
		Offset: 0,
	}

	if token := c.Query("token"); token != "" {
		params.Token = token
	}
	if from := c.Query("from"); from != "" {
		params.From = from
	}
	if to := c.Query("to"); to != "" {
		params.To = to
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			params.Limit = limit
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			params.Offset = offset
		}
	}
	if startBlockStr := c.Query("start_block"); startBlockStr != "" {
		if startBlock, err := strconv.ParseUint(startBlockStr, 10, 64); err == nil {
			params.StartBlock = &startBlock
		}
	}
	if endBlockStr := c.Query("end_block"); endBlockStr != "" {
		if endBlock, err := strconv.ParseUint(endBlockStr, 10, 64); err == nil {
			params.EndBlock = &endBlock
		}
	}
	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		if startTime, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			params.StartTime = &startTime
		}
	}
	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		if endTime, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			params.EndTime = &endTime
		}
	}

	transfers, total, err := h.service.QueryTransfers(c.Request.Context(), params)
	if err != nil {
		metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), "500").Inc()
		metrics.HTTPRequestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(time.Since(start).Seconds())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), "200").Inc()
	metrics.HTTPRequestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(time.Since(start).Seconds())

	c.JSON(http.StatusOK, gin.H{
		"data":   transfers,
		"total":  total,
		"limit":  params.Limit,
		"offset": params.Offset,
	})
}

func (h *TransferHandler) GetAggregates(c *gin.Context) {
	start := time.Now()

	params := models.TransferQueryParams{}

	if token := c.Query("token"); token != "" {
		params.Token = token
	}
	if from := c.Query("from"); from != "" {
		params.From = from
	}
	if to := c.Query("to"); to != "" {
		params.To = to
	}
	if startBlockStr := c.Query("start_block"); startBlockStr != "" {
		if startBlock, err := strconv.ParseUint(startBlockStr, 10, 64); err == nil {
			params.StartBlock = &startBlock
		}
	}
	if endBlockStr := c.Query("end_block"); endBlockStr != "" {
		if endBlock, err := strconv.ParseUint(endBlockStr, 10, 64); err == nil {
			params.EndBlock = &endBlock
		}
	}
	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		if startTime, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			params.StartTime = &startTime
		}
	}
	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		if endTime, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			params.EndTime = &endTime
		}
	}

	aggregates, err := h.service.GetAggregates(c.Request.Context(), params)
	if err != nil {
		metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), "500").Inc()
		metrics.HTTPRequestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(time.Since(start).Seconds())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), "200").Inc()
	metrics.HTTPRequestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(time.Since(start).Seconds())

	c.JSON(http.StatusOK, aggregates)
}
