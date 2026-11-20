import { describe, it, expect, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useFavorites, sanitizeFavorites, FAVORITES_KEY, MAX_FAVORITES } from './useFavorites'

beforeEach(() => localStorage.clear())

describe('useFavorites', () => {
  it('adds, normalizes, persists and removes buses', () => {
    const { result } = renderHook(() => useFavorites())
    expect(result.current.favorites).toEqual([])

    act(() => {
      result.current.toggle(' ks602 ')
    })
    expect(result.current.favorites).toEqual(['KS602'])
    expect(result.current.isFavorite('ks602')).toBe(true)
    expect(JSON.parse(localStorage.getItem(FAVORITES_KEY)!)).toEqual(['KS602'])

    act(() => {
      result.current.toggle('DL1PC0001')
    })
    act(() => result.current.remove('KS602'))
    expect(result.current.favorites).toEqual(['DL1PC0001'])

    act(() => {
      result.current.toggle('DL1PC0001')
    })
    expect(result.current.favorites).toEqual([])
  })

  it('restores sanitized favorites from storage', () => {
    localStorage.setItem(FAVORITES_KEY, JSON.stringify(['ks602', 'KS602', 'bad code!', 42, 'route_500']))
    const { result } = renderHook(() => useFavorites())
    expect(result.current.favorites).toEqual(['KS602', 'ROUTE_500'])
  })

  it('survives corrupt storage', () => {
    localStorage.setItem(FAVORITES_KEY, '{not json')
    const { result } = renderHook(() => useFavorites())
    expect(result.current.favorites).toEqual([])
  })

  it('refuses to grow past the limit', () => {
    localStorage.setItem(FAVORITES_KEY, JSON.stringify(Array.from({ length: MAX_FAVORITES }, (_, i) => `B${i}`)))
    const { result } = renderHook(() => useFavorites())
    expect(result.current.isFull).toBe(true)
    let added = true
    act(() => {
      added = result.current.toggle('EXTRA1')
    })
    expect(added).toBe(false)
    expect(result.current.favorites).toHaveLength(MAX_FAVORITES)
    expect(result.current.isFavorite('EXTRA1')).toBe(false)
  })

  it('syncs when another tab changes the list', () => {
    const { result } = renderHook(() => useFavorites())
    localStorage.setItem(FAVORITES_KEY, JSON.stringify(['ZZ1']))
    act(() => {
      window.dispatchEvent(new StorageEvent('storage', { key: FAVORITES_KEY }))
    })
    expect(result.current.favorites).toEqual(['ZZ1'])
  })
})

describe('sanitizeFavorites', () => {
  it('rejects non-arrays and caps the length', () => {
    expect(sanitizeFavorites({ a: 1 })).toEqual([])
    expect(sanitizeFavorites(null)).toEqual([])
    expect(sanitizeFavorites(Array.from({ length: 40 }, (_, i) => `C${i}`))).toHaveLength(MAX_FAVORITES)
  })
})
