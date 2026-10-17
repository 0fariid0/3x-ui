package model

type NodePendingReset struct {
	Id     int    `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeId int    `json:"nodeId" gorm:"uniqueIndex:idx_node_pending_reset,priority:1;not null"`
	Email  string `json:"email" gorm:"uniqueIndex:idx_node_pending_reset,priority:2;not null"`
	QueuedAt int64 `json:"queuedAt"`
}
