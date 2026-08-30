package dataengine

import "github.com/MajestaNet/ide/internal/db"

// rejectKernelStorage returns a client-facing validation error when the object
// is a kernel identity table (User). Those rows live in users, not records.
func rejectKernelStorage(apiName, mode string) error {
	if db.IsKernelStorage(mode) {
		return validationErrorf("%s is not a flexible object", apiName)
	}
	return nil
}
