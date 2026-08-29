package snapshot

var zeroCanonical = []byte("{\"schema_version\":1,\"families\":[]}\n")

// Zero returns the valid immutable application state used before any publication.
func Zero() ValidatedSnapshot {
	return ValidatedSnapshot{families: []Family{}, canonical: append([]byte(nil), zeroCanonical...)}
}

// NewInitialHolder creates the production last-valid state at generation zero.
func NewInitialHolder() *Holder {
	return NewHolder(Zero())
}
