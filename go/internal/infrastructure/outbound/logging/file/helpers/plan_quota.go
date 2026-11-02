package helpers

// =============================================================================
// PLAN QUOTA
// =============================================================================

// PlanQuota returns the names of the rotated files to delete, in order, so that the total size fits the quota: the
// oldest first (the highest index), never the current file, and only while the total is over the quota. The sizes hold
// the files that exist among base and base.1 ... base.(max_files - 1); any other name is ignored. When every rotated file
// is planned and the total is still over the quota, the current file alone is over it and nothing more is planned.
func PlanQuota(base string, max_files int, sizes map[string]int64, quota_bytes int64) []string {
	var total int64
	for _, size := range sizes {
		total += size
	}
	to_delete := []string{}
	for index := max_files - 1; index >= 1 && total > quota_bytes; index-- {
		name := RotatedName(base, index)
		size, exists := sizes[name]
		if !exists {
			continue
		}
		to_delete = append(to_delete, name)
		total -= size
	}
	return to_delete
}
