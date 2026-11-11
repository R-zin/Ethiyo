import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ErrorBoundary } from './ErrorBoundary'

function ProblemChild({ shouldThrow = false }: { shouldThrow?: boolean }) {
  if (shouldThrow) {
    throw new Error('Test simulation crash')
  }
  return <div>Safe child content</div>
}

describe('ErrorBoundary', () => {
  let consoleErrorSpy: any

  beforeEach(() => {
    // Suppress React boundary logging during intentional crash tests
    consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    consoleErrorSpy.mockRestore()
  })

  it('renders children when there are no errors', () => {
    render(
      <ErrorBoundary>
        <ProblemChild shouldThrow={false} />
      </ErrorBoundary>
    )

    expect(screen.getByText('Safe child content')).toBeDefined()
  })

  it('renders default fallback UI when a child throws an error', () => {
    render(
      <ErrorBoundary>
        <ProblemChild shouldThrow={true} />
      </ErrorBoundary>
    )

    expect(screen.getByRole('alert')).toBeDefined()
    expect(screen.getByText('Something went wrong with the tracker')).toBeDefined()
    expect(screen.getByText(/Test simulation crash/)).toBeDefined()
    expect(screen.getByRole('button', { name: /Try Again/i })).toBeDefined()
    expect(screen.getByRole('button', { name: /Reload Tracker/i })).toBeDefined()
  })

  it('invokes onError prop when an error is caught', () => {
    const handleError = vi.fn()
    render(
      <ErrorBoundary onError={handleError}>
        <ProblemChild shouldThrow={true} />
      </ErrorBoundary>
    )

    expect(handleError).toHaveBeenCalledTimes(1)
    expect(handleError.mock.calls[0][0].message).toBe('Test simulation crash')
  })

  it('resets error state when Try Again is clicked', () => {
    const handleReset = vi.fn()
    let hasThrown = true

    function DynamicChild() {
      if (hasThrown) {
        throw new Error('Failing first time')
      }
      return <div>Recovered successfully</div>
    }

    const { rerender } = render(
      <ErrorBoundary onReset={handleReset}>
        <DynamicChild />
      </ErrorBoundary>
    )

    expect(screen.getByText('Something went wrong with the tracker')).toBeDefined()

    // Fix the condition and click Try Again
    hasThrown = false
    const tryAgainBtn = screen.getByRole('button', { name: /Try Again/i })
    fireEvent.click(tryAgainBtn)

    expect(handleReset).toHaveBeenCalledTimes(1)
    rerender(
      <ErrorBoundary onReset={handleReset}>
        <DynamicChild />
      </ErrorBoundary>
    )
    expect(screen.getByText('Recovered successfully')).toBeDefined()
  })

  it('renders custom function fallback when provided', () => {
    render(
      <ErrorBoundary
        fallback={({ error, resetError }) => (
          <div>
            <span>Custom: {error.message}</span>
            <button onClick={resetError}>Custom Reset</button>
          </div>
        )}
      >
        <ProblemChild shouldThrow={true} />
      </ErrorBoundary>
    )

    expect(screen.getByText('Custom: Test simulation crash')).toBeDefined()
    expect(screen.getByRole('button', { name: 'Custom Reset' })).toBeDefined()
  })

  it('invokes window.location.reload when Reload Tracker is clicked', () => {
    const originalLocation = window.location
    const reloadMock = vi.fn()
    Object.defineProperty(window, 'location', {
      value: { ...originalLocation, reload: reloadMock },
      writable: true,
      configurable: true,
    })

    render(
      <ErrorBoundary>
        <ProblemChild shouldThrow={true} />
      </ErrorBoundary>
    )

    const reloadBtn = screen.getByRole('button', { name: /Reload Tracker/i })
    fireEvent.click(reloadBtn)

    expect(reloadMock).toHaveBeenCalled()
    Object.defineProperty(window, 'location', {
      value: originalLocation,
      writable: true,
      configurable: true,
    })
  })
})
