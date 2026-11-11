import { Component, ErrorInfo, ReactNode } from 'react'
import { AlertCircle, RefreshCw, RotateCcw } from 'lucide-react'

export interface ErrorBoundaryProps {
  children: ReactNode
  fallback?: ReactNode | ((props: { error: Error; resetError: () => void }) => ReactNode)
  onError?: (error: Error, errorInfo: ErrorInfo) => void
  onReset?: () => void
}

export interface ErrorBoundaryState {
  hasError: boolean
  error: Error | null
  errorInfo: ErrorInfo | null
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props)
    this.state = {
      hasError: false,
      error: null,
      errorInfo: null,
    }
  }

  static getDerivedStateFromError(error: Error): Partial<ErrorBoundaryState> {
    return {
      hasError: true,
      error,
    }
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    this.setState({ errorInfo })
    this.props.onError?.(error, errorInfo)
  }

  resetError = () => {
    this.setState({
      hasError: false,
      error: null,
      errorInfo: null,
    })
    this.props.onReset?.()
  }

  handleReload = () => {
    if (typeof window !== 'undefined') {
      window.location.reload()
    }
  }

  render() {
    if (this.state.hasError) {
      const error = this.state.error || new Error('Unknown application error')

      if (typeof this.props.fallback === 'function') {
        return this.props.fallback({ error, resetError: this.resetError })
      }

      if (this.props.fallback) {
        return this.props.fallback
      }

      return (
        <div
          role="alert"
          aria-live="assertive"
          className="min-h-[400px] flex items-center justify-center p-6 bg-nordic-50 dark:bg-nordic-950 text-nordic-900 dark:text-nordic-100"
        >
          <div className="w-full max-w-lg p-6 sm:p-8 rounded-lg border border-berry-200/80 dark:border-berry-900/60 bg-white dark:bg-nordic-900 shadow-lg text-center space-y-4">
            <div className="w-12 h-12 rounded-full bg-berry-100 dark:bg-berry-900/40 text-berry-600 dark:text-berry-400 flex items-center justify-center mx-auto">
              <AlertCircle className="w-6 h-6" />
            </div>

            <div className="space-y-1.5">
              <span className="inline-flex items-center px-2.5 py-0.5 rounded-full bg-berry-100 dark:bg-berry-900/60 text-berry-700 dark:text-berry-300 text-xs font-mono font-medium">
                Application Error
              </span>
              <h2 className="text-lg font-bold tracking-tight text-nordic-900 dark:text-white">
                Something went wrong with the tracker
              </h2>
              <p className="text-xs sm:text-sm text-nordic-600 dark:text-nordic-400 max-w-md mx-auto">
                An unexpected runtime error prevented the transit view from rendering properly. You can try recovering or reloading the interface.
              </p>
            </div>

            {/* Error Details Accordion */}
            <details className="text-left rounded-md bg-nordic-50 dark:bg-nordic-950/80 border border-nordic-200 dark:border-nordic-800 p-3 text-xs">
              <summary className="cursor-pointer font-medium text-nordic-700 dark:text-nordic-300 hover:text-nordic-900 dark:hover:text-white select-none">
                Error Details Summary
              </summary>
              <div className="mt-2.5 space-y-2">
                <p className="font-mono text-berry-700 dark:text-berry-400 break-all font-semibold text-[11px]">
                  {error.name}: {error.message}
                </p>
                {this.state.errorInfo?.componentStack && (
                  <pre className="max-h-36 overflow-auto p-2 bg-nordic-100 dark:bg-nordic-900/90 rounded text-[10px] font-mono text-nordic-600 dark:text-nordic-400 whitespace-pre-wrap">
                    {this.state.errorInfo.componentStack.trim()}
                  </pre>
                )}
              </div>
            </details>

            {/* Recovery Action Buttons */}
            <div className="pt-2 flex flex-col sm:flex-row items-center justify-center gap-2.5">
              <button
                type="button"
                onClick={this.resetError}
                className="w-full sm:w-auto px-4 py-2 rounded-md bg-nordic-900 text-white dark:bg-frost-600 dark:hover:bg-frost-500 hover:bg-nordic-800 text-xs font-medium transition-colors flex items-center justify-center gap-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-frost-500 focus-visible:ring-offset-2"
              >
                <RotateCcw className="w-3.5 h-3.5" />
                <span>Try Again</span>
              </button>

              <button
                type="button"
                onClick={this.handleReload}
                className="w-full sm:w-auto px-4 py-2 rounded-md border border-nordic-200 dark:border-nordic-700 bg-white dark:bg-nordic-800 text-nordic-700 dark:text-nordic-200 hover:bg-nordic-50 dark:hover:bg-nordic-700 text-xs font-medium transition-colors flex items-center justify-center gap-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-frost-500 focus-visible:ring-offset-2"
              >
                <RefreshCw className="w-3.5 h-3.5" />
                <span>Reload Tracker</span>
              </button>
            </div>
          </div>
        </div>
      )
    }

    return this.props.children
  }
}
