package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 项目进展状态枚举
type ProjectProgress string

const (
	ProgressSampleEvaluation ProjectProgress = "样板评估"
	ProgressTesting          ProjectProgress = "打样测试"
	ProgressSmallBatch       ProjectProgress = "小批量导入"
	ProgressMassProduction   ProjectProgress = "批量出货"
	ProgressAbandoned        ProjectProgress = "废弃"
)

// 项目结构体
type Project struct {
	ID                        primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	ProjectName               string             `json:"projectName" bson:"projectName" binding:"required"`
	CustomerID                primitive.ObjectID `json:"customerId" bson:"customerId" binding:"required"`
	CustomerName              string             `json:"customerName" bson:"customerName"`
	CreatorID                 primitive.ObjectID `json:"creatorId" bson:"creatorId"`
	CreatorName               string             `json:"creatorName" bson:"creatorName"`
	UpdaterID                 primitive.ObjectID `json:"updaterId,omitempty" bson:"updaterId,omitempty"`
	UpdaterName               string             `json:"updaterName,omitempty" bson:"updaterName,omitempty"`
	ProductID                 primitive.ObjectID `json:"productId" bson:"productId" binding:"required"`
	ProductName               string             `json:"productName" bson:"productName"`
	BatchNumber               string             `json:"batchNumber" bson:"batchNumber" binding:"required"`
	ProjectProgress           ProjectProgress    `json:"projectProgress" bson:"projectProgress" binding:"required"`
	SmallBatchPrice           float64            `json:"smallBatchPrice,omitempty" bson:"smallBatchPrice,omitempty"`
	SmallBatchQuantity        int                `json:"smallBatchQuantity,omitempty" bson:"smallBatchQuantity,omitempty"`
	SmallBatchTotal           float64            `json:"smallBatchTotal,omitempty" bson:"smallBatchTotal,omitempty"`
	SmallBatchAttachments     []FileAttachment   `json:"smallBatchAttachments,omitempty" bson:"smallBatchAttachments,omitempty"`
	MassProductionPrice       float64            `json:"massProductionPrice,omitempty" bson:"massProductionPrice,omitempty"`
	MassProductionQuantity    int                `json:"massProductionQuantity,omitempty" bson:"massProductionQuantity,omitempty"`
	MassProductionTotal       float64            `json:"massProductionTotal,omitempty" bson:"massProductionTotal,omitempty"`
	PaymentTerm               string             `json:"paymentTerm,omitempty" bson:"paymentTerm,omitempty"`
	MassProductionAttachments []FileAttachment   `json:"massProductionAttachments,omitempty" bson:"massProductionAttachments,omitempty"`
	Remark                    string             `json:"remark,omitempty" bson:"remark,omitempty"`
	CreatedAt                 time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt                 time.Time          `json:"updatedAt" bson:"updatedAt"`
	StartDate                 time.Time          `json:"startDate" bson:"startDate"`
	WebHidden                 bool               `json:"webHidden" bson:"webHidden"`
}

func ConvertProjectToResponse(project Project) ProjectResponse {
	return ProjectResponse{
		ID:                        project.ID.Hex(),
		ProjectName:               project.ProjectName,
		CustomerID:                project.CustomerID.Hex(),
		CustomerName:              project.CustomerName,
		CreatorID:                 project.CreatorID.Hex(),
		CreatorName:               project.CreatorName,
		UpdaterID:                 project.UpdaterID.Hex(),
		UpdaterName:               project.UpdaterName,
		ProductID:                 project.ProductID.Hex(),
		ProductName:               project.ProductName,
		BatchNumber:               project.BatchNumber,
		ProjectProgress:           project.ProjectProgress,
		SmallBatchPrice:           project.SmallBatchPrice,
		SmallBatchQuantity:        project.SmallBatchQuantity,
		SmallBatchTotal:           project.SmallBatchTotal,
		SmallBatchAttachments:     project.SmallBatchAttachments,
		MassProductionPrice:       project.MassProductionPrice,
		MassProductionQuantity:    project.MassProductionQuantity,
		MassProductionTotal:       project.MassProductionTotal,
		PaymentTerm:               project.PaymentTerm,
		MassProductionAttachments: project.MassProductionAttachments,
		Remark:                    project.Remark,
		CreatedAt:                 project.CreatedAt,
		UpdatedAt:                 project.UpdatedAt,
		StartDate:                 project.StartDate,
	}
}

// 文件附件结构体
type FileAttachment struct {
	ID           string    `json:"id" bson:"id"`
	FileName     string    `json:"fileName" bson:"fileName"`
	OriginalName string    `json:"originalName" bson:"originalName"`
	FileSize     int64     `json:"fileSize" bson:"fileSize"`
	FileType     string    `json:"fileType" bson:"fileType"`
	UploadTime   time.Time `json:"uploadTime" bson:"uploadTime"`
	UploadedBy   string    `json:"uploadedBy" bson:"uploadedBy"`
	URL          string    `json:"url" bson:"url"`
}

// 项目响应结构体
type ProjectResponse struct {
	ID                        string           `json:"_id"`
	ProjectName               string           `json:"projectName"`
	CustomerID                string           `json:"customerId"`
	CustomerName              string           `json:"customerName"`
	CreatorID                 string           `json:"creatorId"`
	CreatorName               string           `json:"creatorName"`
	UpdaterID                 string           `json:"updaterId,omitempty"`
	UpdaterName               string           `json:"updaterName,omitempty"`
	ProductID                 string           `json:"productId"`
	ProductName               string           `json:"productName"`
	BatchNumber               string           `json:"batchNumber"`
	ProjectProgress           ProjectProgress  `json:"projectProgress"`
	SmallBatchPrice           float64          `json:"smallBatchPrice,omitempty"`
	SmallBatchQuantity        int              `json:"smallBatchQuantity,omitempty"`
	SmallBatchTotal           float64          `json:"smallBatchTotal,omitempty"`
	SmallBatchAttachments     []FileAttachment `json:"smallBatchAttachments,omitempty"`
	MassProductionPrice       float64          `json:"massProductionPrice,omitempty"`
	MassProductionQuantity    int              `json:"massProductionQuantity,omitempty"`
	MassProductionTotal       float64          `json:"massProductionTotal,omitempty"`
	PaymentTerm               string           `json:"paymentTerm,omitempty"`
	MassProductionAttachments []FileAttachment `json:"massProductionAttachments,omitempty"`
	Remark                    string           `json:"remark,omitempty"`
	CreatedAt                 time.Time        `json:"createdAt"`
	UpdatedAt                 time.Time        `json:"updatedAt"`
	StartDate                 time.Time        `json:"startDate"`
}

type ProjectListResponse struct {
	Projects []ProjectResponse `json:"projects"`
}

type ProjectDetailResponse struct {
	Success bool    `json:"success"`
	Project Project `json:"project"`
}

type FileDownloadResponse struct {
	Success bool           `json:"success"`
	File    FileAttachment `json:"file"`
	Message string         `json:"message"`
}
