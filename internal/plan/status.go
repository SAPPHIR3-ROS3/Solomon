package plan

const (
	StatusNotBuilt       = "not_built"
	StatusPartiallyBuilt = "partially_built"
	StatusBuilt          = "built"
)

func ComputeStatus(total, checked int) string {
	if total == 0 || checked == 0 {
		return StatusNotBuilt
	}
	if checked >= total {
		return StatusBuilt
	}
	return StatusPartiallyBuilt
}
