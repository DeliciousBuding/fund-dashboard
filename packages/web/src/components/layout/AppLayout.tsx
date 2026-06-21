import { type ReactNode } from 'react'
import { Sidebar } from '@cloudflare/kumo'

interface AppLayoutProps {
  sidebar: ReactNode;
  children: ReactNode;
}

export default function AppLayout({ sidebar, children }: AppLayoutProps) {
  return (
    <Sidebar.Provider resizable defaultWidth={260} minWidth={220} maxWidth={340} style={{ height: '100vh', overflow: 'hidden' }}>
      {sidebar}
      <main id="main-content" tabIndex={-1} style={{
        flex: 1, overflow: 'auto', minHeight: 0, height: '100%',
        padding: '24px 32px', background: 'var(--color-kumo-canvas)',
        outline: 'none',
      }}>
        {children}
      </main>
    </Sidebar.Provider>
  );
}
