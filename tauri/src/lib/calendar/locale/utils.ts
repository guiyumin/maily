import { LOCALES, SupportedLang } from './locales';

/**
 * Normalizes a locale string to a supported language code.
 * Handles special cases for regional variants:
 * - Chinese: zh-CN, zh-Hans -> zh (Simplified), zh-TW, zh-Hant, zh-HK -> zhHant (Traditional)
 * - Portuguese: pt-BR -> ptBR (Brazilian Portuguese)
 */
export function normalizeLocale(locale: string): SupportedLang {
  // Normalize: replace underscores with hyphens and convert to lowercase base
  const normalized = locale.replace(/_/g, '-');
  const parts = normalized.split('-');
  const lang = parts[0].toLowerCase();
  const region = parts[1]?.toUpperCase();

  // Handle Chinese variants
  if (lang === 'zh') {
    // Traditional Chinese: zh-TW, zh-Hant, zh-HK
    if (region === 'TW' || region === 'HANT' || region === 'HK') {
      return 'zhHant';
    }
    // Simplified Chinese: zh-CN, zh-Hans, or just zh
    return 'zh';
  }

  // Handle Portuguese variants
  if (lang === 'pt') {
    if (region === 'BR') {
      return 'ptBR';
    }
    // Default Portuguese to Brazilian Portuguese
    return 'ptBR';
  }

  // Check if base language is supported
  if (lang in LOCALES) {
    return lang as SupportedLang;
  }

  return 'en';
}

/**
 * Checks if a string is a valid locale identifier.
 */
export function isValidLocale(locale: string): boolean {
  try {
    // eslint-disable-next-line no-new
    new Intl.DateTimeFormat(locale);
    return true;
  } catch (e) {
    return false;
  }
}
