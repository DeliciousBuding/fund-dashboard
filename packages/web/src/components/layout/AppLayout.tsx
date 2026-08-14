import { type ReactNode, type CSSProperties } from 'react'
import { Sidebar } from '@cloudflare/kumo'
import { useAppStore } from '../../stores/appStore'
import { ambientCanvasStyle, getTheme, space } from '../../styles/theme'

interface AppLayoutProps {
  sidebar: ReactNode;
  children: ReactNode;
  contentPaddingBottom?: string | number;
}

export default function AppLayout({ sidebar, children, contentPaddingBottom }: AppLayoutProps) {
  const dark = useAppStore((s) => s.dark);
  const theme = getTheme(dark);
  const ambient = ambientCanvasStyle(theme);

  const mainStyle: CSSProperties = {
    flex: 1,
    overflow: 'auto',
    minHeight: 0,
    height: '100%',
    padding: 'var(--fd-space-5, 24px) var(--fd-space-6, 32px)',
    outline: 'none',
    position: 'relative',
    // Ambient mesh under content so frosted cards have depth to sample.
    backgroundColor: ambient.backgroundColor as string,
    backgroundImage: ambient.backgroundImage as string,
    backgroundAttachment: 'fixed',
  };
  if (contentPaddingBottom != null) {
    mainStyle.paddingBottom = contentPaddingBottom;
  }
  return (
    <Sidebar.Provider
      resizable
      defaultWidth={260}
      minWidth={220}
      maxWidth={340}
      style={{ height: '100vh', overflow: 'hidden', background: theme.canvas }}
    >
      {sidebar}
      <main id="main-content" tabIndex={-1} className="fd-main-ambient" style={mainStyle}>
        {children}
      </main>
    </Sidebar.Provider>
  );
}
