// github.com/BerniceZTT/crm_end/models/customer_assignment.go
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CustomerAssignmentHistory 客户分配历史记录
type CustomerAssignmentHistory struct {
	ID                   primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	CustomerID           string             `json:"customerId" bson:"customerid"`
	CustomerName         string             `json:"customerName" bson:"customername"`
	FromRelatedSalesID   string             `json:"fromRelatedSalesId,omitempty" bson:"fromrelatedsalesid,omitempty"`
	FromRelatedSalesName string             `json:"fromRelatedSalesName,omitempty" bson:"fromrelatedsalesname,omitempty"`
	ToRelatedSalesID     string             `json:"toRelatedSalesId,omitempty" bson:"torelatedsalesid,omitempty"`
	ToRelatedSalesName   string             `json:"toRelatedSalesName,omitempty" bson:"torelatedsalesname,omitempty"`
	FromRelatedAgentID   string             `json:"fromRelatedAgentId,omitempty" bson:"fromrelatedagentid,omitempty"`
	FromRelatedAgentName string             `json:"fromRelatedAgentName,omitempty" bson:"fromrelatedagentname,omitempty"`
	ToRelatedAgentID     string             `json:"toRelatedAgentId,omitempty" bson:"torelatedagentid,omitempty"`
	ToRelatedAgentName   string             `json:"toRelatedAgentName,omitempty" bson:"torelatedagentname,omitempty"`
	OperatorID           string             `json:"operatorId" bson:"operatorid"`
	OperatorName         string             `json:"operatorName" bson:"operatorname"`
	OperationType        string             `json:"operationType" bson:"operationtype"`
	CreatedAt            time.Time          `json:"createdAt" bson:"createdat"`
	UpdatedAt            time.Time          `json:"updatedAt" bson:"updatedat"`
}
