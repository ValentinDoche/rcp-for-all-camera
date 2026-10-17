package i18n

// Messages exposes the raw messages per language to the external tests
// that check every locale has every key.
func (b *Bundle) Messages() map[string]map[string]string { return b.msgs }
