package imports

// changes compares the records of a table to the database, by content hash.
type changes[K comparable] struct {
	created      []K
	updated      []K
	deleted      []K
	currentTotal int
	nextTotal    int
}

func computeChanges[K comparable](current, next map[K]string) changes[K] {
	result := changes[K]{currentTotal: len(current), nextTotal: len(next)}
	for key, hash := range next {
		currentHash, found := current[key]
		switch {
		case !found:
			result.created = append(result.created, key)
		case currentHash != hash:
			result.updated = append(result.updated, key)
		}
	}

	for key := range current {
		if _, found := next[key]; !found {
			result.deleted = append(result.deleted, key)
		}
	}

	return result
}

func (c changes[K]) report() tableReport {
	return tableReport{
		Total:   c.nextTotal,
		Created: len(c.created),
		Updated: len(c.updated),
		Deleted: len(c.deleted),
	}
}

func (c changes[K]) isEmpty() bool {
	return len(c.created) == 0 && len(c.updated) == 0 && len(c.deleted) == 0
}

// upserts returns the records created or updated.
func upserts[T any, K comparable](values []T, key func(T) K, c changes[K]) []T {
	keys := make(map[K]struct{}, len(c.created)+len(c.updated))
	for _, k := range c.created {
		keys[k] = struct{}{}
	}
	for _, k := range c.updated {
		keys[k] = struct{}{}
	}

	result := make([]T, 0, len(keys))
	for _, value := range values {
		if _, found := keys[key(value)]; found {
			result = append(result, value)
		}
	}

	return result
}

func hashes[T any, K comparable](values []T, key func(T) K, hash func(T) string) map[K]string {
	result := make(map[K]string, len(values))
	for _, value := range values {
		result[key(value)] = hash(value)
	}

	return result
}
