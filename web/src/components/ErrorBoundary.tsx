import { Component, type ErrorInfo, type ReactNode } from 'react';

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

/**
 * Without this, one throw during render unmounts the whole tree and the user
 * gets a blank page with no way back. Show what broke and offer a reload.
 */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('snagarr: render failed', error, info.componentStack);
  }

  render() {
    const { error } = this.state;
    if (!error) return this.props.children;

    return (
      <div className="sg-pad py-8">
        <span className="sg-k">SOMETHING BROKE</span>
        <h3 className="mt-2">This screen failed to render.</h3>
        <pre className="text-muted overflow-x-auto whitespace-pre-wrap text-[12px] leading-relaxed">
          {error.message}
        </pre>
        <button
          type="button"
          className="btn btn-secondary mt-3 min-h-[44px]"
          onClick={() => this.setState({ error: null })}
        >
          Try again
        </button>
      </div>
    );
  }
}
