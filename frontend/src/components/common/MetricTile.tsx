import React from 'react'
import { cn } from '../../utils/cn'

interface MetricTileProps {
  label: string
  value: string | number
  unit?: string
  subtext?: string
  icon?: React.ReactNode
  variant?: 'default' | 'highlight'
  className?: string
}

export function MetricTile({
  label,
  value,
  unit,
  subtext,
  icon,
  variant = 'default',
  className,
}: MetricTileProps) {
  return (
    <div
      className={cn(
        'p-4 rounded-lg border transition-all duration-150',
        variant === 'highlight'
          ? 'bg-frost-50/50 border-frost-200 dark:bg-frost-950/20 dark:border-frost-900/60'
          : 'bg-white border-nordic-200/80 dark:bg-nordic-900/70 dark:border-nordic-800/80',
        className
      )}
    >
      <div className="flex items-center justify-between text-nordic-500 dark:text-nordic-400 mb-1.5">
        <span className="text-xs font-medium uppercase tracking-wider">{label}</span>
        {icon && <span className="text-nordic-400 dark:text-nordic-500">{icon}</span>}
      </div>

      <div className="flex items-baseline gap-1.5">
        <span className="text-xl sm:text-2xl font-semibold tracking-tight text-nordic-900 dark:text-nordic-100 tabular-nums">
          {value}
        </span>
        {unit && (
          <span className="text-xs font-medium text-nordic-500 dark:text-nordic-400">
            {unit}
          </span>
        )}
      </div>

      {subtext && (
        <p className="mt-1 text-xs text-nordic-500 dark:text-nordic-400 truncate">
          {subtext}
        </p>
      )}
    </div>
  )
}
