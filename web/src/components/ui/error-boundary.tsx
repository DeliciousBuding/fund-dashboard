import { Component, type ErrorInfo, type ReactNode } from "react";
import { sanitizeUserError } from "../../services/userError";
import { Button } from "./button";

// ErrorBoundary — 面板级错误兜底：崩一面板不崩全页（四态纪律 error 态）。
interface Props {
  children: ReactNode;
  fallbackTitle?: string;
}

interface State {
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("[ErrorBoundary]", error, info.componentStack);
  }

  render() {
    if (this.state.error) {
      return (
        <div className="flex flex-col items-center justify-center gap-2 rounded-xl border border-border bg-surface-1 px-6 py-10 text-center">
          <p className="text-sm font-medium text-fg-2">
            {this.props.fallbackTitle ?? "这一块出错了"}
          </p>
          <p className="max-w-md text-xs break-all text-fg-3">
            {sanitizeUserError(this.state.error, "未知错误")}
          </p>
          <Button
            variant="secondary"
            size="sm"
            className="mt-2"
            onClick={() => this.setState({ error: null })}
          >
            重试
          </Button>
        </div>
      );
    }
    return this.props.children;
  }
}
