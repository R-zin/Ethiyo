import React from 'react'
import { cn } from '../../utils/cn'

export type StatusVariant = 'active' | 'idle' | 'warning' | 'error' | 'neutral'

interface StatusBadgeProps {
  variant?: StatusVariant
  children: React.ReactNode
  pulse?: boolean
  className?: string
}

export function StatusBadge({
  variant = 'neutral',
  children,
  pulse = false,
  className,
}: StatusBadgeProps) {
  const variantStyles = {
    active: 'bg-pine-50 text-pine-700 border-pine-200/80 dark:bg-pine-950/60 dark:text-pine-400 dark:border-pine-800/60',
    idle: 'bg-nordic-100 text-nordic-600 border-nordic-200 dark:bg-nordic-900 dark:text-nordic-400 dark:border-nordic-800',
    warning: 'bg-amber-50 text-amber-700 border-amber-200/80 dark:bg-amber-950/60 dark:text-amber-400 dark:border-amber-800/60',
    error: 'bg-berry-50 text-berry-700 border-berry-200/80 dark:bg-berry-950/60 dark:text-berry-400 dark:border-berry-800/60',
    neutral: 'bg-nordic-100 text-nordic-700 border-nordic-200 dark:bg-nordic-900 dark:text-nordic-300 dark:border-nordic-800',
  }

  const dotStyles = {
    active: 'bg-pine-500',
    idle: 'bg-nordic-400',
    warning: 'bg-amber-500',
    error: 'bg-berry-500',
    neutral: 'bg-nordic-400',
  }

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium border tracking-wide',
        variantStyles[variant],
        className
      )}
    >
      <span className="relative flex h-1.5 w-1.5">
        {pulse && (
          <span
            className={cn(
              'animate-ping absolute inline-flex h-full w-full rounded-full opacity-75',
              dotStyles[variant]
            )}
          />
        )}
        <span className={cn('relative inline-flex rounded-full h-1.5 w-1.5', dotStyles[variant])} />
      </span>
      {children}
    </span>
  )
}
