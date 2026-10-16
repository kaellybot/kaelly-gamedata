package databases

const (
	// BatchSize is the number of rows per INSERT statement.
	BatchSize = 500

	// chunkSize is the number of values per IN (...) clause.
	chunkSize = 1000
)

// InChunks calls apply on consecutive chunks of values, to keep IN clauses bounded.
func InChunks[T any](values []T, apply func(chunk []T) error) error {
	for start := 0; start < len(values); start += chunkSize {
		end := min(start+chunkSize, len(values))
		if err := apply(values[start:end]); err != nil {
			return err
		}
	}

	return nil
}
