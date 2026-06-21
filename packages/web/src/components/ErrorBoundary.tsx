import { Component, type ReactNode } from 'react'
import { Button, Text } from '@cloudflare/kumo'
import i18n from '../i18n'

interface Props { children: ReactNode; fallback?: ReactNode }
interface State { hasError: boolean; error: string }

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: '' }

  static getDerivedStateFromError(e: Error) {
    return { hasError: true, error: e.message }
  }

  render() {
    if (this.state.hasError) {
      return this.props.fallback || (
        <div style={{ padding: 60, textAlign: 'center' }}>
          <Text variant="error" as="span" style={{ display: 'block', fontSize: 14, marginBottom: 16 }}>
            {i18n.t('error.boundaryTitle')}: {this.state.error}
          </Text>
          <Button variant="primary" onClick={() => this.setState({ hasError: false, error: '' })}>
            {i18n.t('error.boundaryRetry')}
          </Button>
        </div>
      )
    }
    return <ErrorResetKey key={this.state.hasError ? 'err' : 'ok'}>{this.props.children}</ErrorResetKey>
  }
}

function ErrorResetKey({ children }: { children: ReactNode }) { return <>{children}</> }
