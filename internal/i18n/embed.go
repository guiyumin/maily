package i18n

import (
	"embed"
)

//go:embed locales/*.yml
var localesFS embed.FS

func loadEmbeddedTranslations() error {
	entries, err := localesFS.ReadDir("locales")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, err := bundle.LoadMessageFileFS(localesFS, "locales/"+entry.Name()); err != nil {
			return err
		}
	}
	return nil
}
