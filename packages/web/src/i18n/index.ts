import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import en from './en.json';
import zh from './zh.json';

const LANG_KEY = 'fund-dashboard-lang';

function detectLanguage(): string {
  const stored = localStorage.getItem(LANG_KEY);
  if (stored === 'en' || stored === 'zh') return stored;
  const nav = navigator.language || '';
  if (nav.startsWith('zh')) return 'zh';
  return 'en';
}

i18n.use(initReactI18next).init({
  resources: { en: { translation: en }, zh: { translation: zh } },
  lng: detectLanguage(),
  fallbackLng: 'en',
  interpolation: { escapeValue: false },
});

export function persistLanguage(lang: string) {
  localStorage.setItem(LANG_KEY, lang);
  i18n.changeLanguage(lang);
}

export default i18n;
