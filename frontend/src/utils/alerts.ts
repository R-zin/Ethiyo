import { BusLiveInfo } from '../types/transit'
import { estimateEtaMinutes, formatDistance, haversineMeters } from './geo'

/** Within this distance the bus counts as "here", even if it is stopped. */
export const ARRIVAL_RADIUS_M = 300

export interface AlertConfig {
  /** Fire when the bus is this many minutes away (null = off). */
  approachMinutes: number | null
  /** Fire when the bus stops reporting live positions. */
  offline: boolean
}

export interface AlertInputs {
  busCode: string
  live?: BusLiveInfo
  degraded?: boolean
  user?: { latitude: number; longitude: number } | null
}

export interface AlertState {
  approachFired: boolean
  offlineFired: boolean
  wasReporting: boolean
}

export interface FiredAlert {
  kind: 'approach' | 'offline'
  title: string
  body: string
}

export const INITIAL_ALERT_STATE: AlertState = { approachFired: false, offlineFired: false, wasReporting: false }

function isReporting(inputs: AlertInputs): boolean {
  return !!inputs.live && !inputs.live.stale && !inputs.degraded
}

/**
 * Pure alert rules, evaluated on every position update.
 *
 * - approach: one-shot; fires when the naive ETA (straight line ÷ current
 *   speed) drops to the threshold, or when the bus is within
 *   ARRIVAL_RADIUS_M regardless of speed. Never fires on stale/degraded data.
 * - offline: fires once per outage, on the transition from reporting live to
 *   not reporting (stale, degraded or gone); re-arms when reporting resumes.
 */
export function evaluateAlerts(
  config: AlertConfig,
  inputs: AlertInputs,
  state: AlertState
): { state: AlertState; fired: FiredAlert[] } {
  const fired: FiredAlert[] = []
  const next = { ...state }
  const reporting = isReporting(inputs)

  if (config.approachMinutes !== null && !state.approachFired && reporting && inputs.user && inputs.live) {
    const distance = haversineMeters(
      inputs.user.latitude,
      inputs.user.longitude,
      inputs.live.latitude,
      inputs.live.longitude
    )
    const eta = estimateEtaMinutes(distance, inputs.live.speed)
    if (distance <= ARRIVAL_RADIUS_M || (eta !== null && eta <= config.approachMinutes)) {
      next.approachFired = true
      fired.push({
        kind: 'approach',
        title: `Bus ${inputs.busCode} is almost here`,
        body:
          distance <= ARRIVAL_RADIUS_M
            ? `It is ${formatDistance(distance)} away.`
            : `About ${eta} min away (${formatDistance(distance)}, straight line).`,
      })
    }
  }

  if (config.offline) {
    if (reporting) {
      next.offlineFired = false
    } else if (state.wasReporting && !state.offlineFired) {
      next.offlineFired = true
      fired.push({
        kind: 'offline',
        title: `Bus ${inputs.busCode} stopped reporting`,
        body: inputs.degraded
          ? 'Chalo is not responding. Showing the last known position.'
          : 'Its GPS has gone quiet. The position shown may be out of date.',
      })
    }
  }

  next.wasReporting = reporting
  return { state: next, fired }
}
