package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BerniceZTT/crm_end/repository"
	"github.com/BerniceZTT/crm_end/service"
	"github.com/BerniceZTT/crm_end/utils"
)

// AssignCustomer 处理客户分配请求
// 分配客户到指定销售和(可选)代理商
func AssignCustomer(c *gin.Context) {
	// 获取客户ID
	customerId := c.Param("id")
	if customerId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "客户ID不能为空"})
		return
	}

	// 解析请求数据
	var assignRequest service.AssignRequest
	if err := c.ShouldBindJSON(&assignRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	_, code, ginH := service.AssignCustomer(repository.GetContext(), c, customerId, assignRequest, user)
	c.JSON(code, ginH)
}
