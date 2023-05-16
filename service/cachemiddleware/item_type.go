package cachemiddleware

// ItemType is the type of a cache item.
type ItemType int

const (
	ItemTypeQuery ItemType = iota + 1
	ItemTypeChain
)

// KeyPrefix is the prefix for a cache key.
type KeyPrefix string

const (
	KeyPrefixUnknown KeyPrefix = "unknown"
	KeyPrefixQuery   KeyPrefix = "query"
	KeyPrefixChain   KeyPrefix = "chain"
)

// IsValid returns true if the key prefix is valid.
func (p KeyPrefix) IsValid() bool {
	return p == KeyPrefixQuery || p == KeyPrefixChain
}

// String returns the string representation of the key prefix.
func (p KeyPrefix) String() string {
	return string(p)
}

// Prefix returns the key prefix for the item type.
func (t ItemType) Prefix() KeyPrefix {
	switch t {
	case ItemTypeQuery:
		return KeyPrefixQuery
	case ItemTypeChain:
		return KeyPrefixChain
	default:
		return KeyPrefixUnknown
	}
}
