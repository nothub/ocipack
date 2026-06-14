package ocipack

import "fmt"

func validatePlatform(p Platform) error {
	if p.OS == "" {
		return fmt.Errorf("platform OS is required")
	}
	if p.Architecture == "" {
		return fmt.Errorf("platform Architecture is required")
	}
	return nil
}
