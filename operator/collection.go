package infraoperator

// collection is a slice of an arbitrary objects without duplicates
type collection []interface{}

func (c collection) add(obj interface{}) collection {
	for i := range c {
		if obj == c[i] {
			return c
		}
	}

	return append(c, obj)
}

func (c collection) remove(obj interface{}) collection {
	if len(c) == 0 {
		return c
	}

	for i := range c {
		if obj == c[i] {
			c[i] = c[len(c)-1]
			return c[0 : len(c)-1]
		}
	}

	return c
}
