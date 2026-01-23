import en from './en';
import zh from './zh';
import zhHant from './zhHant';
import ja from './ja';
import ko from './ko';
import fr from './fr';
import de from './de';
import es from './es';
import ptBR from './ptBR';
import pl from './pl';
import nl from './nl';
import it from './it';
import ru from './ru';

export { en, zh, zhHant, ja, ko, fr, de, es, ptBR, pl, nl, it, ru };

export const LOCALES = {
  en,
  zh,
  zhHant,
  ja,
  ko,
  fr,
  de,
  es,
  ptBR,
  pl,
  nl,
  it,
  ru,
};

export type SupportedLang = keyof typeof LOCALES;
