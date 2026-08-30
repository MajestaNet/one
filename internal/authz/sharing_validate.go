package authz

import "fmt"

// ValidateSharingAccessLevel checks rule access_level values.
func ValidateSharingAccessLevel(level string) error {
	switch level {
	case "read", "read_write":
		return nil
	default:
		return fmt.Errorf("accessLevel must be read or read_write")
	}
}

// ValidateOWDDefaultAccess checks object default_access values.
// Empty string is allowed for PATCH (means leave unchanged at the store layer).
func ValidateOWDDefaultAccess(access string) error {
	switch access {
	case DefaultAccessPrivate, DefaultAccessPublicRead, DefaultAccessPublicReadWrite, "":
		return nil
	default:
		return fmt.Errorf("defaultAccess must be private, public_read, or public_read_write")
	}
}
