package auth

type Keys map[string]struct{}

func NewKeys(keys ...string) Keys {
	k := make(Keys, len(keys))
	for _, key := range keys {
		k[key] = struct{}{}
	}
	return k
}

func (k Keys) valid(key string) bool {
	if key == "" {
		return false
	}
	_, ok := k[key]
	return ok
}
