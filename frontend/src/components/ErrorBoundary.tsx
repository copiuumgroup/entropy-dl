import { Component, type ReactNode, type ErrorInfo } from 'react';

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

/**
 * ErrorBoundary catches unhandled render errors in the React tree
 * and displays a recovery UI instead of a blank white screen.
 */
export default class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error('[ErrorBoundary]', error, info.componentStack);
  }

  handleReload = (): void => {
    this.setState({ hasError: false, error: null });
    window.location.reload();
  };

  render(): ReactNode {
    if (this.state.hasError) {
      return (
        <div style={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          height: '100vh',
          padding: '2rem',
          fontFamily: 'var(--sys-font, system-ui, sans-serif)',
          color: 'var(--on-surface, #1c1b1f)',
          textAlign: 'center',
        }}>
          <span style={{ fontSize: 48, marginBottom: 16 }} aria-hidden="true">error_outline</span>
          <h2 style={{ margin: '0 0 8px', fontSize: 20, fontWeight: 500 }}>Something went wrong</h2>
          <p style={{ margin: '0 0 16px', color: 'var(--on-surface-variant, #49454f)', maxWidth: 400 }}>
            An unexpected error occurred. This has been logged to the console.
          </p>
          {this.state.error && (
            <pre style={{
              margin: '0 0 16px',
              padding: 12,
              background: 'var(--surface-container, #f3edf7)',
              borderRadius: 12,
              fontSize: 12,
              maxWidth: 500,
              overflow: 'auto',
              textAlign: 'left',
            }}>
              {this.state.error.message}
            </pre>
          )}
          <button
            className="btn filled"
            onClick={this.handleReload}
            type="button"
          >
            Reload app
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}