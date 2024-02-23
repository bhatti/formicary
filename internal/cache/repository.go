package cache

// Repository provides access to cache
type Repository interface {
	// Get Finds Cache by group and id
	Get(group string, ids ...string) (map[string][]byte, error)
	// GetAll Finds Cache by group
	GetAll(group string) (map[string][]byte, error)
	// Save updates cache with given value
	Save(group string, id string, value []byte) error
	// Delete deletes Cache by group and id
	Delete(group string, id string) error
}
