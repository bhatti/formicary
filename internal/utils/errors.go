package utils

// ErrorsAny returns first non-nil error
func ErrorsAny(errors ...error) error {
	for _, err := range errors {
		if err != nil {
			return err
		}
	}
	return nil
}
