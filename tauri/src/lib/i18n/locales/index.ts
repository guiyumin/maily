import yaml from 'js-yaml';
import type { LocaleMessages, SupportedLocale } from '../types';

// Import YAML files as raw strings
import enYml from './en.yml?raw';
import zhYml from './zh.yml?raw';
import zhHantYml from './zhHant.yml?raw';
import koYml from './ko.yml?raw';
import jaYml from './ja.yml?raw';
import esYml from './es.yml?raw';
import deYml from './de.yml?raw';
import frYml from './fr.yml?raw';
import ptBRYml from './ptBR.yml?raw';
import plYml from './pl.yml?raw';
import nlYml from './nl.yml?raw';
import itYml from './it.yml?raw';
import ruYml from './ru.yml?raw';

function parseYaml(content: string): LocaleMessages {
  return yaml.load(content) as LocaleMessages;
}

export const LOCALES: Record<SupportedLocale, LocaleMessages> = {
  en: parseYaml(enYml),
  zh: parseYaml(zhYml),
  zhHant: parseYaml(zhHantYml),
  ko: parseYaml(koYml),
  ja: parseYaml(jaYml),
  es: parseYaml(esYml),
  de: parseYaml(deYml),
  fr: parseYaml(frYml),
  ptBR: parseYaml(ptBRYml),
  pl: parseYaml(plYml),
  nl: parseYaml(nlYml),
  it: parseYaml(itYml),
  ru: parseYaml(ruYml),
};
