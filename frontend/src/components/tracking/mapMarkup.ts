// Markup helpers for the Leaflet map, kept free of Leaflet so they can be
// unit-tested in isolation.

export interface PopupData {
  busCode: string
  routeName?: string
  speedKmh?: number
  latitude: number
  longitude: number
  stale?: boolean
}

function el<K extends keyof HTMLElementTagNameMap>(tag: K, style: string, text?: string): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag)
  node.setAttribute('style', style)
  // textContent, never innerHTML: route names come from the upstream API and
  // must not be interpreted as markup.
  if (text !== undefined) node.textContent = text
  return node
}

/**
 * Builds the bus popup as DOM nodes. Every dynamic value is assigned via
 * textContent, so a hostile route name such as `<img src=x onerror=…>` is
 * rendered as literal text instead of executing.
 */
export function buildPopupContent(d: PopupData): HTMLElement {
  const root = el('div', 'font-family: inherit;')
  root.append(
    el('div', 'font-weight: 600; font-size: 13px; margin-bottom: 2px;', `Bus ${d.busCode}`),
    el('div', 'font-size: 11px; opacity: 0.7; margin-bottom: 6px;', d.routeName || 'Live Transit Route')
  )
  const row = el('div', 'display: flex; gap: 8px; font-size: 11px; font-family: monospace;')
  row.append(
    el('span', 'color: #0284c7; font-weight: 600;', `${Math.round(d.speedKmh ?? 0)} km/h`),
    el('span', 'opacity: 0.5;', '·'),
    el('span', '', `${d.latitude.toFixed(4)}, ${d.longitude.toFixed(4)}`)
  )
  root.append(row)
  if (d.stale) {
    root.append(el('div', 'margin-top: 6px; font-size: 11px; color: #d97706;', 'Position may be outdated'))
  }
  return root
}

/**
 * HTML for the bus marker. Only numbers derived via Number() are interpolated,
 * so no untrusted string ever reaches this markup.
 */
export function busIconHtml(heading?: number, stale = false): string {
  const hasHeading = typeof heading === 'number' && Number.isFinite(heading)
  const deg = hasHeading ? ((Math.round(Number(heading)) % 360) + 360) % 360 : 0
  const arrow = hasHeading
    ? `<div class="bus-heading" data-heading="${deg}" style="position:absolute;inset:0;transform:rotate(${deg}deg);pointer-events:none">
         <svg width="40" height="40" viewBox="0 0 40 40" aria-hidden="true"><path d="M20 1 L25 9 L15 9 Z" fill="#0284c7"/></svg>
       </div>`
    : ''
  const ping = stale
    ? ''
    : '<span class="animate-ping absolute inline-flex h-8 w-8 rounded-full bg-frost-400 opacity-60"></span>'
  const core = stale ? 'bg-nordic-400' : 'bg-nordic-900'
  return `
    <div class="relative flex items-center justify-center w-10 h-10">
      ${ping}
      ${arrow}
      <div class="relative flex items-center justify-center w-8 h-8 rounded-full ${core} border-2 border-white shadow-md text-white text-xs">🚌</div>
    </div>`
}
