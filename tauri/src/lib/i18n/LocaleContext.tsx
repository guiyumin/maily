import { createContext, useContext } from 'react';
import type { TranslationKey } from './types';

export interface LocaleContextValue {
  locale: string;
  t: (key: TranslationKey) => string;
}

export const LocaleContext = createContext<LocaleContextValue>({
  locale: 'en',
  t: (key) => key,
});

export function useLocale(): LocaleContextValue {
  return useContext(LocaleContext);
}
