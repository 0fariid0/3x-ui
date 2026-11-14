package service

// RelayLightMode is separate from bulk settings so older clients cannot
// accidentally overwrite the operator's choice.
func (s *SettingService) GetRelayLightMode() (bool, error) {
	return s.getBool("relayLightMode")
}

func (s *SettingService) SetRelayLightMode(enabled bool) error {
	return s.setBool("relayLightMode", enabled)
}
