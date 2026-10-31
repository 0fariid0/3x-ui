package entity

type IPBlockSettings struct {
	ExemptIPs      string `json:"exemptIPs"`
	Enabled        bool   `json:"enabled"`
	BanSeconds     int    `json:"banSeconds"`
	FindSeconds    int    `json:"findSeconds"`
	MaxRetry       int    `json:"maxRetry"`
	ScanSeconds    int    `json:"scanSeconds"`
	LiveSeconds    int    `json:"liveSeconds"`
	HistorySeconds int    `json:"historySeconds"`
	Disconnect     bool   `json:"disconnect"`
}

type IPBlockSettingsStatus struct {
	Settings  IPBlockSettings `json:"settings"`
	Available bool            `json:"available"`
	Reason    string          `json:"reason"`
}
