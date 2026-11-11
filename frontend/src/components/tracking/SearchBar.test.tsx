import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { SearchBar } from './SearchBar'

describe('SearchBar', () => {
  it('calls onSearch when submitted', () => {
    const handleSearch = vi.fn()
    render(<SearchBar onSearch={handleSearch} />)

    const input = screen.getByPlaceholderText(/Search bus code/i)
    fireEvent.change(input, { target: { value: 'KS602' } })

    const form = input.closest('form')
    if (form) {
      fireEvent.submit(form)
    }

    expect(handleSearch).toHaveBeenCalledWith('KS602')
  })

  it('selects preset chip when clicked', () => {
    const handleSearch = vi.fn()
    render(<SearchBar onSearch={handleSearch} />)

    const preset = screen.getByText('DL1PC0001')
    fireEvent.click(preset)

    expect(handleSearch).toHaveBeenCalledWith('DL1PC0001')
  })

  it('focuses search input when "/" is pressed and user is not in an editable element', () => {
    const handleSearch = vi.fn()
    render(<SearchBar onSearch={handleSearch} />)

    const searchInput = screen.getByPlaceholderText(/Search bus code/i)
    expect(document.activeElement).not.toBe(searchInput)

    fireEvent.keyDown(window, { key: '/' })
    expect(document.activeElement).toBe(searchInput)
  })

  it('does not hijack focus when "/" is pressed while focused in another input or textarea', () => {
    const handleSearch = vi.fn()
    render(
      <div>
        <input data-testid="other-input" type="text" />
        <textarea data-testid="other-textarea" />
        <div data-testid="other-editable" contentEditable="true" />
        <SearchBar onSearch={handleSearch} />
      </div>
    )

    const searchInput = screen.getByPlaceholderText(/Search bus code/i)
    const otherInput = screen.getByTestId('other-input')
    const otherTextarea = screen.getByTestId('other-textarea')
    const otherEditable = screen.getByTestId('other-editable')

    // Focused in external input
    otherInput.focus()
    fireEvent.keyDown(otherInput, { key: '/' })
    expect(document.activeElement).toBe(otherInput)
    expect(document.activeElement).not.toBe(searchInput)

    // Focused in external textarea
    otherTextarea.focus()
    fireEvent.keyDown(otherTextarea, { key: '/' })
    expect(document.activeElement).toBe(otherTextarea)
    expect(document.activeElement).not.toBe(searchInput)

    // Focused in contenteditable div
    otherEditable.focus()
    fireEvent.keyDown(otherEditable, { key: '/' })
    expect(document.activeElement).toBe(otherEditable)
    expect(document.activeElement).not.toBe(searchInput)
  })
})
