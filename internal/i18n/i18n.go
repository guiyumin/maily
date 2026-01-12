package i18n

import (
	"os"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

var (
	bundle    *i18n.Bundle
	localizer *i18n.Localizer
	lang      string
)

// SupportedLanguages returns the list of supported language codes
var SupportedLanguages = []string{
	"en", "ko", "ja", "zh-Hans", "zh-Hant",
	"es", "de", "fr", "pt-BR", "pl", "nl", "it", "ru",
}

// LanguageNames maps language codes to their native display names
var LanguageNames = map[string]string{
	"en":      "English",
	"ko":      "한국어",
	"ja":      "日本語",
	"zh-Hans": "简体中文",
	"zh-Hant": "繁體中文",
	"es":      "Español",
	"de":      "Deutsch",
	"fr":      "Français",
	"pt-BR":   "Português (Brasil)",
	"pl":      "Polski",
	"nl":      "Nederlands",
	"it":      "Italiano",
	"ru":      "Русский",
}

// Init initializes the i18n system
// Priority: explicit language > LANG env var > default (en)
func Init(configLang string) error {
	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("yml", yaml.Unmarshal)

	// Load embedded translation files
	if err := loadEmbeddedTranslations(); err != nil {
		return err
	}

	// Determine language
	lang = detectLanguage(configLang)

	// Create localizer with fallback to English
	localizer = i18n.NewLocalizer(bundle, lang, "en")
	return nil
}

// detectLanguage determines which language to use
func detectLanguage(configLang string) string {
	// 1. Explicit config setting takes priority
	if configLang != "" {
		return normalizeLanguage(configLang)
	}

	// 2. Check LANG environment variable
	if envLang := os.Getenv("LANG"); envLang != "" {
		return normalizeLanguage(envLang)
	}

	// 3. Check LC_ALL
	if lcAll := os.Getenv("LC_ALL"); lcAll != "" {
		return normalizeLanguage(lcAll)
	}

	// 4. Default to English
	return "en"
}

// normalizeLanguage converts locale strings to supported language codes
func normalizeLanguage(locale string) string {
	// Remove encoding suffix (e.g., ".UTF-8")
	locale = strings.Split(locale, ".")[0]
	// Replace underscore with hyphen
	locale = strings.Replace(locale, "_", "-", 1)

	// Handle Chinese variants
	lower := strings.ToLower(locale)
	switch {
	case strings.HasPrefix(lower, "zh-cn"), strings.HasPrefix(lower, "zh-hans"):
		return "zh-Hans"
	case strings.HasPrefix(lower, "zh-tw"), strings.HasPrefix(lower, "zh-hant"), strings.HasPrefix(lower, "zh-hk"):
		return "zh-Hant"
	case strings.HasPrefix(lower, "pt-br"):
		return "pt-BR"
	}

	// Extract just the language code for other locales
	parts := strings.Split(locale, "-")
	langCode := strings.ToLower(parts[0])

	// Check if it's a supported language
	for _, supported := range SupportedLanguages {
		if supported == langCode {
			return langCode
		}
	}

	// Default to English for unsupported languages
	return "en"
}

// T translates a message ID with optional template data
func T(id string, data ...map[string]interface{}) string {
	if localizer == nil {
		return id // Not initialized, return the ID as fallback
	}

	cfg := &i18n.LocalizeConfig{MessageID: id}
	if len(data) > 0 && data[0] != nil {
		cfg.TemplateData = data[0]
	}

	msg, err := localizer.Localize(cfg)
	if err != nil {
		return id // Fallback to message ID
	}
	return msg
}

// TPlural translates a message with pluralization support
func TPlural(id string, count int, data map[string]interface{}) string {
	if localizer == nil {
		return id
	}

	if data == nil {
		data = make(map[string]interface{})
	}
	data["Count"] = count

	cfg := &i18n.LocalizeConfig{
		MessageID:    id,
		PluralCount:  count,
		TemplateData: data,
	}

	msg, err := localizer.Localize(cfg)
	if err != nil {
		return id
	}
	return msg
}

// CurrentLanguage returns the current language code
func CurrentLanguage() string {
	return lang
}

// DisplayName returns the native display name for a language code
func DisplayName(code string) string {
	if name, ok := LanguageNames[code]; ok {
		return name
	}
	return code
}

// IsSupported checks if a language code is supported
func IsSupported(code string) bool {
	for _, l := range SupportedLanguages {
		if l == code {
			return true
		}
	}
	return false
}
