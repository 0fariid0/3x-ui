package entity

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

// Older settings forms did not include the Xray health tuning fields. They
// must still pass request validation; CheckValid fills their safe defaults.
func TestAllSettingValidationAllowsOmittedXrayHealthTuning(t *testing.T) {
	s := AllSetting{
		WebPort:                    2053,
		SessionMaxAge:              360,
		PageSize:                   25,
		SmtpPort:                   587,
		OutboundDownThreshold:      3,
		SubPort:                    2096,
		LdapPort:                   389,
		XrayHealthEnable:           false,
		XrayHealthFailureThreshold: 0,
		XrayHealthRestartCooldown:  0,
		XrayHealthMaxRestarts:      0,
		XrayHealthWindowMinutes:    0,
	}

	v := validator.New(validator.WithRequiredStructEnabled())
	if err := v.Struct(&s); err != nil {
		t.Fatalf("omitted Xray health tuning fields must be accepted: %v", err)
	}
}
