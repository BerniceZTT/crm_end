package models

// 图表数据项
type ChartDataItem struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// 项目批量统计
type ProjectBatchStats struct {
	TotalAmount   float64 `json:"totalAmount"`   // 总金额
	TotalProjects int     `json:"totalProjects"` // 项目数量
	MaxAmount     float64 `json:"maxAmount"`     // 最大金额
	MinAmount     float64 `json:"minAmount"`     // 最小金额
}

// 项目月度统计
type ProjectMonthlyStats struct {
	Month            string  `json:"month"`            // 月份 (格式: YYYY.MM)
	ProjectCount     int     `json:"projectCount"`     // 项目数量
	TotalAmount      float64 `json:"totalAmount"`      // 总金额
	BatchAmount      float64 `json:"batchAmount"`      // 批量金额
	SmallBatchAmount float64 `json:"smallBatchAmount"` // 小批量金额
}

// 项目价值项
type ProjectValueItem struct {
	ProjectID           string  `json:"projectId"`           // 项目ID
	ProjectName         string  `json:"projectName"`         // 项目名称
	CustomerName        string  `json:"customerName"`        // 客户名称
	ProductID           string  `json:"productId"`           // 产品ID
	ProductName         string  `json:"productName"`         // 产品名称
	SmallBatchTotal     float64 `json:"smallBatchTotal"`     // 小批量金额
	MassProductionTotal float64 `json:"massProductionTotal"` // 批量金额
	TotalValue          float64 `json:"totalValue"`          // 总价值
	Progress            string  `json:"progress"`            // 项目进展
}

// 数据看板响应结构
type DashboardDataResponse struct {
	CustomerCount int `json:"customerCount"` // 客户总数
	ProductCount  int `json:"productCount"`  // 产品总数
	AgentCount    int `json:"agentCount"`    // 代理商总数
	ProjectCount  int `json:"projectCount"`  // 项目总数

	CustomerImportance []ChartDataItem `json:"customerImportance"` // 客户重要性分布
	CustomerNature     []ChartDataItem `json:"customerNature"`     // 客户性质分布

	ProductPackageType      []ChartDataItem `json:"productPackageType"`      // 产品包装类型分布
	ProductStockLevel       []ChartDataItem `json:"productStockLevel"`       // 产品库存等级分布
	ProductCustomerRelation []ChartDataItem `json:"productCustomerRelation"` // 产品客户关联分布
	ProductProjectRelation  []ChartDataItem `json:"productProjectRelation"`  // 产品项目关联分布

	ProjectProgressDistribution []ChartDataItem       `json:"projectProgressDistribution"` // 项目进展分布
	ProjectBatchTotalStats      ProjectBatchStats     `json:"projectBatchTotalStats"`      // 批量总额统计
	ProjectSmallBatchTotalStats ProjectBatchStats     `json:"projectSmallBatchTotalStats"` // 小批量总额统计
	ProjectMonthlyStats         []ProjectMonthlyStats `json:"projectMonthlyStats"`         // 项目月度统计
	TopProjectsByValue          []ProjectValueItem    `json:"topProjectsByValue"`          // 项目价值排行Top10
}
