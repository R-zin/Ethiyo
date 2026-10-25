import React, { useState, useEffect, useRef } from 'react'
import { Search, X, History, Sparkles } from 'lucide-react'

interface SearchBarProps {
  onSearch: (busCode: string) => void
  isLoading?: boolean
  initialCode?: string
}

const PRESET_ROUTES = [
  { code: 'KS602', label: 'Bangalore Express' },
  { code: 'DL1PC0001', label: 'Delhi Transit' },
  { code: 'route_500', label: 'Ring Road 500' },
  { code: 'KA01F1234', label: 'City Shuttle' },
]

export function SearchBar({ onSearch, isLoading = false, initialCode = '' }: SearchBarProps) {
  const [query, setQuery] = useState(initialCode)
  const [recentSearches, setRecentSearches] = useState<string[]>(() => {
    try {
      const saved = localStorage.getItem('ethiyo_recent_searches')
      return saved ? JSON.parse(saved).slice(0, 4) : []
    } catch {
      return []
    }
  })
  const inputRef = useRef<HTMLInputElement>(null)

  // Update query when initialCode changes externally
  useEffect(() => {
    if (initialCode) {
      setQuery(initialCode)
    }
  }, [initialCode])

  // Global '/' keyboard shortcut to focus search input
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === '/' && document.activeElement !== inputRef.current) {
        e.preventDefault()
        inputRef.current?.focus()
        inputRef.current?.select()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const clean = query.trim().toUpperCase()
    if (!clean) return

    // Save to recents
    const updated = [clean, ...recentSearches.filter(s => s !== clean)].slice(0, 5)
    setRecentSearches(updated)
    try {
      localStorage.setItem('ethiyo_recent_searches', JSON.stringify(updated))
    } catch {
      // ignore storage errors
    }

    onSearch(clean)
  }

  const handleSelectPreset = (code: string) => {
    setQuery(code)
    onSearch(code)
  }

  return (
    <div className="w-full space-y-3">
      <form onSubmit={handleSubmit} className="relative">
        <label htmlFor="bus-search-input" className="sr-only">
          Search bus code or route identifier
        </label>

        <div className="relative flex items-center">
          <div className="absolute left-3.5 text-nordic-400 dark:text-nordic-500 pointer-events-none">
            <Search className="w-4 h-4" />
          </div>

          <input
            ref={inputRef}
            id="bus-search-input"
            aria-label="Search bus code or route identifier"
            type="text"
            value={query}
            onChange={e => setQuery(e.target.value)}
            placeholder="Search bus code (e.g. KS602, DL1PC0001, route_500)..."
            disabled={isLoading}
            autoCapitalize="characters"
            spellCheck={false}
            className="w-full pl-10 pr-24 py-3 rounded-lg border border-nordic-200 dark:border-nordic-800 bg-white dark:bg-nordic-900 text-nordic-900 dark:text-nordic-100 placeholder:text-nordic-400 focus:outline-none focus:ring-1 focus:ring-frost-500 focus:border-frost-500 text-sm font-mono tracking-wide transition-all shadow-sm"
          />

          <div className="absolute right-2 flex items-center gap-1.5">
            {query && (
              <button
                type="button"
                onClick={() => {
                  setQuery('')
                  inputRef.current?.focus()
                }}
                className="p-1 rounded text-nordic-400 hover:text-nordic-600 dark:hover:text-nordic-200"
                aria-label="Clear input"
              >
                <X className="w-3.5 h-3.5" />
              </button>
            )}

            <button
              type="submit"
              disabled={isLoading || !query.trim()}
              className="px-3 py-1.5 rounded-md bg-nordic-900 text-white dark:bg-frost-600 dark:hover:bg-frost-500 hover:bg-nordic-800 text-xs font-medium tracking-wide transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1.5"
            >
              <span>Track</span>
              <kbd className="hidden sm:inline text-[10px] bg-nordic-800/80 dark:bg-frost-700/80 px-1 py-0.5 rounded font-mono text-nordic-300">
                ↵
              </kbd>
            </button>
          </div>
        </div>
      </form>

      {/* Suggested Routes & Recent Search Chips */}
      <div className="flex flex-wrap items-center gap-2 pt-1 text-xs">
        <span className="text-nordic-400 dark:text-nordic-500 font-medium flex items-center gap-1">
          <Sparkles className="w-3 h-3 text-frost-500" /> Suggestions:
        </span>
        {PRESET_ROUTES.map(item => (
          <button
            key={item.code}
            type="button"
            onClick={() => handleSelectPreset(item.code)}
            className="px-2.5 py-1 rounded border border-nordic-200/90 dark:border-nordic-800 bg-white dark:bg-nordic-900 hover:border-frost-400 dark:hover:border-frost-600 text-nordic-700 dark:text-nordic-300 font-mono text-xs transition-colors"
          >
            {item.code}
          </button>
        ))}

        {recentSearches.length > 0 && (
          <>
            <span className="text-nordic-400 dark:text-nordic-600 ml-1">·</span>
            <span className="text-nordic-400 dark:text-nordic-500 font-medium flex items-center gap-1">
              <History className="w-3 h-3" /> Recent:
            </span>
            {recentSearches.map(code => (
              <button
                key={code}
                type="button"
                onClick={() => handleSelectPreset(code)}
                className="px-2 py-0.5 rounded bg-nordic-100 dark:bg-nordic-800/70 hover:bg-nordic-200 dark:hover:bg-nordic-800 text-nordic-600 dark:text-nordic-400 font-mono text-xs transition-colors"
              >
                {code}
              </button>
            ))}
          </>
        )}
      </div>
    </div>
  )
}
