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
})
