import { space, fontWeight, lineHeight } from '../styles/theme'
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
  const { t, i18n } = useTranslation();

  const toggle = () => {
    const next = i18n.language === 'zh' ? 'en' : 'zh';
    persistLanguage(next);
  };

  // Target-language labels (same in zh+en catalogs) so the control names the language it switches to.
  const label = i18n.language === 'zh' ? t('nav.switchToEn') : t('nav.switchToZh');

  return (
    <Button type="button"
      variant="secondary"
      size={size}
      onClick={toggle}
      title={label}
      aria-label={label}
      style={{ minWidth: 32, padding: `${space[2]-2}px ${space[2]}px`, fontWeight: fontWeight.semibold, lineHeight: lineHeight.none }}
    >
      {i18n.language === 'zh' ? t('nav.langFaceEn') : t('nav.langFaceZh')}
    </Button>
  );
}
