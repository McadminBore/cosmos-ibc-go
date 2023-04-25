package types

import (
	fmt "fmt"
	"strings"
)

// NewParams creates a new parameter configuration for the host submodule
func NewParams(enableHost bool, allowMsgs []string) Params {
	return Params{
		HostEnabled:   enableHost,
		AllowMessages: allowMsgs,
	}
}

// DefaultParams is the default parameter configuration for the host submodule
func DefaultParams() Params {
	return NewParams(DefaultHostEnabled, []string{AllowAllHostMsgs})
}

// Validate validates all host submodule parameters
func (p Params) Validate() error {
	if err := validateEnabledType(p.HostEnabled); err != nil {
		return err
	}

	return validateAllowedlist(p.AllowMessages)
}

func validateAllowedlist(allowMsgs []string) error {
	for _, typeURL := range allowMsgs {
		if strings.TrimSpace(typeURL) == "" {
			return fmt.Errorf("parameter must not contain empty strings: %s", allowMsgs)
		}
	}

	return nil
}
