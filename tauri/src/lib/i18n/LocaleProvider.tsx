import React, { useMemo } from 'react';
import { LocaleContext } from './LocaleContext';
import { LOCALES } from './locales';
import type { TranslationKey, SupportedLocale } from './types';

export interface LocaleProviderProps {
  locale: string;
  children: React.ReactNode;
}

/**
 * Normalize config language to our SupportedLocale keys
 * Config uses: en, ko, ja, zh-Hans, zh-Hant, zh, es, de, fr, pt-BR, pl, nl, it, ru
 * Our keys:    en, ko, ja, zh,      zhHant, zh, es, de, fr, ptBR,  pl, nl, it, ru
 */
function normalizeLocale(locale: string): SupportedLocale {
  // Map config values to our locale keys
  const mapping: Record<string, SupportedLocale> = {
    'en': 'en',
    'ko': 'ko',
    'ja': 'ja',
    'zh': 'zh',
    'zh-Hans': 'zh',
    'zh-Hant': 'zhHant',
    'es': 'es',
    'de': 'de',
    'fr': 'fr',
    'pt-BR': 'ptBR',
    'pl': 'pl',
    'nl': 'nl',
    'it': 'it',
    'ru': 'ru',
  };

  return mapping[locale] || 'en';
}

export const LocaleProvider: React.FC<LocaleProviderProps> = ({
  locale,
  children,
}) => {
  const value = useMemo(() => {
    const normalizedLocale = normalizeLocale(locale);
    const messages = LOCALES[normalizedLocale] || LOCALES.en;

    return {
      locale: normalizedLocale,
      t: (key: TranslationKey): string => {
        return messages[key] || LOCALES.en[key] || key;
      },
    };
  }, [locale]);

  return (
    <LocaleContext.Provider value={value}>
      {children}
    </LocaleContext.Provider>
  );
};
