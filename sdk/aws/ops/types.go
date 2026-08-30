package ops

// RollRequest is the target product version and images for an ECS roll.
type RollRequest struct {
	APIImage       string
	WorkerImage    string
	ProductVersion string
}

// AWSRoller method set (CaptureCurrent / Roll / Rollback) matches the product
// ops.Roller interface so callers can adapt this type into a product Roller
// without importing github.com/MajestaNet/ide/internal/ops from this module.
