package entity

type IPBlockEntry struct {
	IP        string `json:"ip" example:"203.0.113.25"`
	Source    string `json:"source" example:"manual"`
	Email     string `json:"email" example:"customer-a"`
	BannedAt  int64  `json:"bannedAt" example:"1793102400000"`
	ExpiresAt int64  `json:"expiresAt" example:"1793104200000"`
	Permanent bool   `json:"permanent" example:"false"`
}

type IPBlockStatus struct {
	Available    bool           `json:"available" example:"true"`
	Reason       string         `json:"reason" example:""`
	Jail         string         `json:"jail" example:"3x-ipl"`
	BanSeconds   int64          `json:"banSeconds" example:"1800"`
	Blocks       []IPBlockEntry `json:"blocks"`
	AuditWarning bool           `json:"auditWarning" example:"false"`
}

type IPBlockChange struct {
	NodeID int    `json:"nodeId" form:"nodeId" example:"0"`
	IP     string `json:"ip" form:"ip" example:"203.0.113.25"`
	Action string `json:"action" form:"action" example:"ban"`
	Email  string `json:"email" form:"email" example:"customer-a"`
}
