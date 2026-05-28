package core

func pageCountFor(total, perPage int) int {
	if total == 0 || perPage <= 0 {
		return 0
	}
	return (total + perPage - 1) / perPage
}
