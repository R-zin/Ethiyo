export function SkeletonLoader() {
  return (
    <div className="space-y-6" aria-busy="true" aria-label="Loading transit data">
      {/* Top Banner Skeleton */}
      <div className="bg-white dark:bg-nordic-900 border border-nordic-200/80 dark:border-nordic-800 rounded-lg p-6">
        <div className="flex flex-col sm:flex-row justify-between gap-4">
          <div className="space-y-2.5">
            <div className="h-7 w-44 bg-nordic-200 dark:bg-nordic-800 animate-pulse rounded" />
            <div className="h-4 w-72 bg-nordic-100 dark:bg-nordic-800/60 animate-pulse rounded" />
          </div>
          <div className="flex gap-2">
            <div className="h-8 w-24 bg-nordic-200 dark:bg-nordic-800 animate-pulse rounded-full" />
            <div className="h-8 w-8 bg-nordic-200 dark:bg-nordic-800 animate-pulse rounded" />
          </div>
        </div>

        {/* 4 Metrics grid */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4 mt-6">
          {Array.from({ length: 4 }).map((_, i) => (
            <div
              key={i}
              className="p-4 rounded-lg border border-nordic-100 dark:border-nordic-800/60 bg-nordic-50/50 dark:bg-nordic-950/40 space-y-2"
            >
              <div className="h-3 w-16 bg-nordic-200 dark:bg-nordic-800 animate-pulse rounded" />
              <div className="h-6 w-24 bg-nordic-200 dark:bg-nordic-800 animate-pulse rounded" />
            </div>
          ))}
        </div>
      </div>

      {/* Map Skeleton */}
      <div className="h-96 rounded-lg border border-nordic-200/80 dark:border-nordic-800 bg-nordic-100/70 dark:bg-nordic-900/60 animate-pulse flex items-center justify-center">
        <span className="text-xs font-mono text-nordic-400">Loading satellite coordinates...</span>
      </div>
    </div>
  )
}
