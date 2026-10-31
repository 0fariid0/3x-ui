package entity

type IPBlockSettings struct {
	NamedExemptions    string `json:"namedExemptions"`
	UnbanGraceSeconds  int    `json:"unbanGraceSeconds"`
	NotifyTelegram     bool   `json:"notifyTelegram"`
	EnforcementScope   string `json:"enforcementScope"`
	SwitchGraceSeconds int    `json:"switchGraceSeconds"`
	ConfirmScans       int    `json:"confirmScans"`
	ExemptIPs          string `json:"exemptIPs"`
	Enabled            bool   `json:"enabled"`
	BanSeconds         int    `json:"banSeconds"`
	FindSeconds        int    `json:"findSeconds"`
	MaxRetry           int    `json:"maxRetry"`
	ScanSeconds        int    `json:"scanSeconds"`
	LiveSeconds        int    `json:"liveSeconds"`
	HistorySeconds     int    `json:"historySeconds"`
	Disconnect         bool   `json:"disconnect"`
}

type IPBlockSettingsStatus struct {
	PolicyRevision int             `json:"policyRevision"`
	Settings       IPBlockSettings `json:"settings"`
	Available      bool            `json:"available"`
	Reason         string          `json:"reason"`
}
