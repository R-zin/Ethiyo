import { useCallback, useEffect, useState } from 'react'
import { isValidBusCode, normalizeCode } from '../utils/busCode'

export const FAVORITES_KEY = 'ethiyo_favorites'
export const MAX_FAVORITES = 12

/** Keeps only valid, de-duplicated, normalized codes, capped at MAX_FAVORITES. */
export function sanitizeFavorites(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  const out: string[] = []
  for (const v of value) {
    if (typeof v !== 'string') continue
    const code = normalizeCode(v)
    if (isValidBusCode(code) && !out.includes(code)) out.push(code)
    if (out.length >= MAX_FAVORITES) break
  }
  return out
}

function readFavorites(): string[] {
  try {
    return sanitizeFavorites(JSON.parse(localStorage.getItem(FAVORITES_KEY) || '[]'))
  } catch {
    return [] // blocked storage or corrupt JSON: start empty
  }
}

/**
 * "My buses": saved bus codes persisted in localStorage and kept in sync
 * across open tabs. Storage failures (private mode, blocked site data) only
 * cost persistence; the list still works for the current page.
 */
export function useFavorites() {
  const [favorites, setFavorites] = useState<string[]>(readFavorites)

  useEffect(() => {
    try {
      localStorage.setItem(FAVORITES_KEY, JSON.stringify(favorites))
    } catch {
      // ignore storage errors
    }
  }, [favorites])

  useEffect(() => {
    const onStorage = (e: StorageEvent) => {
      if (e.key === FAVORITES_KEY) setFavorites(readFavorites())
    }
    window.addEventListener('storage', onStorage)
    return () => window.removeEventListener('storage', onStorage)
  }, [])

  const isFavorite = useCallback((code: string) => favorites.includes(normalizeCode(code)), [favorites])

  /** Adds or removes a bus. Adding to a full list is refused (returns false). */
  const toggle = useCallback(
    (code: string): boolean => {
      const clean = normalizeCode(code)
      if (!isValidBusCode(clean)) return false
      if (favorites.includes(clean)) {
        setFavorites(favorites.filter(c => c !== clean))
        return false
      }
      if (favorites.length >= MAX_FAVORITES) return false
      setFavorites([...favorites, clean])
      return true
    },
    [favorites]
  )

  const remove = useCallback((code: string) => {
    const clean = normalizeCode(code)
    setFavorites(prev => prev.filter(c => c !== clean))
  }, [])

  return { favorites, isFavorite, toggle, remove, isFull: favorites.length >= MAX_FAVORITES }
}
