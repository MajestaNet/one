import { Component, type ErrorInfo, type ReactNode } from "react";
import { Button } from "../ui";

type Props = {
  children: ReactNode;
  /** Optional label for the fallback (e.g. Tool apiName). */
  label?: string;
};

type State = {
  error: Error | null;
};

/** Isolates Run Tool render failures so one bad node cannot freeze the IDE shell. */
export class RunToolErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error("Run Tool render failed", error, info.componentStack);
  }

  render() {
    if (this.state.error) {
      return (
        <div className="run-tool-error-boundary" data-testid="run-tool-error-boundary" role="alert">
          <p className="error">
            {this.props.label ? `${this.props.label}: ` : ""}
            This Tool failed to render. The rest of the IDE stays usable.
          </p>
          <p className="muted mono">{this.state.error.message}</p>
          <Button
            type="button"
            variant="secondary"
            onClick={() => this.setState({ error: null })}
          >
            Try again
          </Button>
        </div>
      );
    }
    return this.props.children;
  }
}
