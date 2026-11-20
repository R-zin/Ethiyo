import { useCallback, useEffect, useRef, useState } from 'react'
import {
  AlertConfig,
  AlertInputs,
  AlertState,
  FiredAlert,
  INITIAL_ALERT_STATE,
  evaluateAlerts,
} from '../utils/alerts'

export type NotifyPermission = NotificationPermission | 'unsupported'

export interface ActiveAlert extends FiredAlert {
  id: number
  busCode: string
}

const OFF: AlertConfig = { approachMinutes: null, offline: false }

function currentPermission(): NotifyPermission {
  return typeof Notification === 'undefined' ? 'unsupported' : Notification.permission
}

/** Shows a system notification when allowed. Never throws. */
export async function showSystemNotification(alert: FiredAlert, busCode: string): Promise<boolean> {
  try {
    navigator.vibrate?.([200, 100, 200])
  } catch {
    // vibration is best-effort
  }
  if (typeof Notification === 'undefined' || Notification.permission !== 'granted') return false
  const opts: NotificationOptions = { body: alert.body, tag: `ethiyo-${busCode}-${alert.kind}`, icon: '/icons/icon-192.png' }
  try {
    // Mobile browsers only allow notifications via a service worker.
    const reg = await navigator.serviceWorker?.getRegistration?.()
    if (reg) {
      await reg.showNotification(alert.title, opts)
      return true
    }
    new Notification(alert.title, opts)
    return true
  } catch {
    return false
  }
}

export interface AlertOptions {
  notify?: (alert: FiredAlert, busCode: string) => void
}

/**
 * Arrival and disruption alerts for the bus on screen. Rules live in the
 * pure evaluateAlerts(); this hook wires them to state, browser
 * notifications and in-page banners. Alerts only run while the page is open.
 */
export function useAlerts(input: AlertInputs, options: AlertOptions = {}) {
  const notify = options.notify ?? showSystemNotification
  // Armed settings belong to one bus; switching buses disarms by derivation
  // rather than by a reset effect, so no stale config can ever fire.
  const [armed, setArmed] = useState<{ busCode: string; config: AlertConfig }>({ busCode: '', config: OFF })
  const config = armed.busCode === input.busCode ? armed.config : OFF
  const [alerts, setAlerts] = useState<ActiveAlert[]>([])
  const [permission, setPermission] = useState<NotifyPermission>(currentPermission)
  const stateRef = useRef<{ busCode: string; state: AlertState }>({ busCode: '', state: INITIAL_ALERT_STATE })
  const nextId = useRef(0)

  const { busCode, live, degraded, user } = input
  useEffect(() => {
    if (!busCode) return
    const prev = stateRef.current.busCode === busCode ? stateRef.current.state : INITIAL_ALERT_STATE
    const { state, fired } = evaluateAlerts(config, { busCode, live, degraded, user }, prev)
    stateRef.current = { busCode, state }
    if (fired.length === 0) return
    setAlerts(list => [...list, ...fired.map(f => ({ ...f, id: ++nextId.current, busCode }))])
    for (const f of fired) notify(f, busCode)
  }, [config, busCode, live, degraded, user, notify])

  const requestPermission = useCallback(async () => {
    if (typeof Notification === 'undefined') return
    if (Notification.permission === 'default') {
      try {
        setPermission(await Notification.requestPermission())
      } catch {
        // some browsers only support the callback form; ignore
      }
    } else {
      setPermission(Notification.permission)
    }
  }, [])

  const setApproachMinutes = useCallback(
    (minutes: number | null) => {
      if (stateRef.current.busCode === input.busCode) {
        // Re-arming after a fired alert should be able to fire again.
        stateRef.current = { ...stateRef.current, state: { ...stateRef.current.state, approachFired: false } }
      }
      setArmed({ busCode: input.busCode, config: { ...config, approachMinutes: minutes } })
      if (minutes !== null) void requestPermission()
    },
    [config, input.busCode, requestPermission]
  )

  const setOffline = useCallback(
    (enabled: boolean) => {
      setArmed({ busCode: input.busCode, config: { ...config, offline: enabled } })
      if (enabled) void requestPermission()
    },
    [config, input.busCode, requestPermission]
  )

  const dismiss = useCallback((id: number) => setAlerts(list => list.filter(a => a.id !== id)), [])

  return { config, alerts, dismiss, permission, setApproachMinutes, setOffline }
}
