import { useTranslation } from 'react-i18next';
import { Button } from '@cloudflare/kumo';
import { persistLanguage } from '../i18n';

interface LanguageSwitcherProps {
  size?: 'sm' | 'md' | 'lg';
}

/** Compact Kumo Button that toggles the UI language (zh ⇄ en).
 *  v3.0: migrated from a raw <button> to Kumo Button for component consistency
 *  and accessible naming (Button exposes aria-label natively). */
export default function LanguageSwitcher({ size = 'sm' }: LanguageSwitcherProps) {
  const { i18n } = useTranslation();

  const toggle = () => {
    const next = i18n.language === 'zh' ? 'en' : 'zh';
    persistLanguage(next);
  };

  const label = i18n.language === 'zh' ? 'Switch to English' : '切换到中文';

  return (
    <Button
      variant="secondary"
      size={size}
      onClick={toggle}
      title={label}
      aria-label={label}
      style={{ minWidth: 32, padding: '6px 8px', fontWeight: 600, lineHeight: 1 }}
    >
      {i18n.language === 'zh' ? 'EN' : '中'}
    </Button>
  );
}
