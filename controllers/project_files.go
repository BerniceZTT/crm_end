package controllers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/BerniceZTT/crm_end/models"
	"github.com/BerniceZTT/crm_end/repository"
	"github.com/BerniceZTT/crm_end/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// 文件上传请求结构体
type FileUploadRequest struct {
	FileData string `json:"fileData"`
	FileName string `json:"fileName"`
	FileSize int64  `json:"fileSize"`
	FileType string `json:"fileType"`
}

// 文件上传响应结构体
type FileUploadResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	File    models.FileInfo `json:"file"`
}

// uploadFile 处理文件上传
func UploadFile(c *gin.Context) {
	currentUser, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[文件上传] 用户: %s 开始上传文件", currentUser.Username)

	var req FileUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	// 检查文件大小限制（20MB）
	const maxFileSize = 20 * 1024 * 1024 // 20MB
	if req.FileSize > maxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("文件大小超出限制，最大支持 %dMB", maxFileSize/1024/1024),
		})
		return
	}

	// 生成文件ID和存储路径
	fileID := fmt.Sprintf("file_%d_%s", time.Now().Unix(), utils.RandomString(9))
	storagePath := fmt.Sprintf("uploads/%d_%s", time.Now().Unix(), req.FileName)

	var objID primitive.ObjectID
	if v, err := primitive.ObjectIDFromHex(currentUser.ID); err != nil {
		objID = v
	}

	// 创建文件记录
	fileRecord := models.FileInfo{
		ID:           fileID,
		FileName:     storagePath,
		OriginalName: req.FileName,
		FileSize:     req.FileSize,
		FileType:     req.FileType,
		UploadTime:   time.Now(),
		UploadedBy:   currentUser.Username,
		URL:          req.FileData, // 暂时存储base64，后续可改为云存储URL
		UploaderID:   objID,
		CreatedAt:    time.Now(),
	}

	// 存储到专用文件集合
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filesCollection := repository.Collection(repository.ProjectFilesCollection)
	result, err := filesCollection.InsertOne(ctx, fileRecord)
	if err != nil {
		log.Printf("[文件上传] 上传失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文件保存失败"})
		return
	}

	if result.InsertedID == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文件保存失败"})
		return
	}

	log.Printf("[文件上传] 文件上传成功: %s, ID: %s", req.FileName, fileID)

	// 返回文件引用信息（不包含实际文件数据）
	fileReference := models.FileInfo{
		ID:           fileID,
		FileName:     storagePath,
		OriginalName: req.FileName,
		FileSize:     req.FileSize,
		FileType:     req.FileType,
		UploadTime:   fileRecord.UploadTime,
		UploadedBy:   currentUser.Username,
		URL:          fileID, // 使用fileId作为引用
	}

	c.JSON(http.StatusOK, FileUploadResponse{
		Success: true,
		Message: "文件上传成功",
		File:    fileReference,
	})
}

// 文件下载响应结构体
type FileDownloadResponse struct {
	Success bool            `json:"success"`
	File    models.FileInfo `json:"file"`
}

// downloadFile 处理文件下载
func DownloadFile(c *gin.Context) {
	currentUser, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	fileID := c.Param("fileId")
	log.Printf("[文件下载] 用户: %s 下载文件: %s", currentUser.Username, fileID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filesCollection := repository.Collection("repository.ProjectFilesCollection")
	var fileRecord models.FileInfo
	err = filesCollection.FindOne(ctx, bson.M{"id": fileID}).Decode(&fileRecord)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询文件失败"})
		}
		return
	}

	log.Printf("[文件下载] 文件下载成功: %s", fileRecord.OriginalName)

	c.JSON(http.StatusOK, FileDownloadResponse{
		Success: true,
		File: models.FileInfo{
			ID:           fileRecord.ID,
			FileName:     fileRecord.FileName,
			OriginalName: fileRecord.OriginalName,
			FileSize:     fileRecord.FileSize,
			FileType:     fileRecord.FileType,
			UploadTime:   fileRecord.UploadTime,
			UploadedBy:   fileRecord.UploadedBy,
			URL:          fileRecord.URL,
		},
	})
}

// 文件删除响应结构体
type FileDeleteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// deleteFile 处理文件删除
func DeleteFile(c *gin.Context) {
	currentUser, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	fileID := c.Param("fileId")
	log.Printf("[文件删除] 用户: %s 删除文件: %s", currentUser.Username, fileID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filesCollection := repository.Collection("repository.ProjectFilesCollection")
	result, err := filesCollection.DeleteOne(ctx, bson.M{"id": fileID})
	if err != nil {
		log.Printf("[文件删除] 删除失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文件删除失败"})
		return
	}

	if result.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}

	log.Printf("[文件删除] 文件删除成功: %s", fileID)

	c.JSON(http.StatusOK, FileDeleteResponse{
		Success: true,
		Message: "文件删除成功",
	})
}
