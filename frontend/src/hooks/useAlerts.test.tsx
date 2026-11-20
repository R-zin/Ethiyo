import { describe, it, expect, vi, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useAlerts, showSystemNotification } from './useAlerts'
import { AlertInputs } from '../utils/alerts'
import { BusLiveInfo } from '../types/transit'

const live = (over: Partial<BusLiveInfo> = {}): BusLiveInfo => ({
  bus_code: 'KS602',
  latitude: 10.03,
  longitude: 76,
  speed: 36,
  recorded_at: new Date().toISOString(),
  live_tracking: true,
  tracking_url: '',
  age_seconds: 5,
  stale: false,
  ...over,
})
const user = { latitude: 10, longitude: 76 }

function setup(initial: AlertInputs) {
  const notify = vi.fn()
  const hook = renderHook(({ input }) => useAlerts(input, { notify }), { initialProps: { input: initial } })
  return { ...hook, notify }
}

afterEach(() => vi.unstubAllGlobals())

describe('useAlerts', () => {
  it('fires an armed approach alert once, as a banner and a notification', () => {
    const { result, rerender, notify } = setup({ busCode: 'KS602', live: live(), user })
    expect(result.current.alerts).toEqual([])

    act(() => result.current.setApproachMinutes(10))
    expect(result.current.config.approachMinutes).toBe(10)
    expect(result.current.alerts).toHaveLength(1)
    expect(result.current.alerts[0]).toMatchObject({ kind: 'approach', busCode: 'KS602' })
    expect(notify).toHaveBeenCalledTimes(1)

    rerender({ input: { busCode: 'KS602', live: live({ latitude: 10.02 }), user } })
    expect(notify).toHaveBeenCalledTimes(1) // one-shot

    act(() => result.current.dismiss(result.current.alerts[0].id))
    expect(result.current.alerts).toEqual([])
  })

  it('re-arming after a fired alert can fire again', () => {
    const { result, notify } = setup({ busCode: 'KS602', live: live(), user })
    act(() => result.current.setApproachMinutes(10))
    act(() => result.current.setApproachMinutes(15))
    expect(notify).toHaveBeenCalledTimes(2)
  })

  it('switching buses disarms (an alert never fires for the wrong bus)', () => {
    const { result, rerender, notify } = setup({ busCode: 'AAA1', live: live({ latitude: 11 }), user })
    act(() => result.current.setApproachMinutes(10)) // far away: armed, not fired
    expect(notify).not.toHaveBeenCalled()

    rerender({ input: { busCode: 'BBB2', live: live(), user } }) // B is close
    expect(result.current.config.approachMinutes).toBeNull()
    expect(notify).not.toHaveBeenCalled()
  })

  it('fires the offline alert when a bus stops reporting', () => {
    const { result, rerender, notify } = setup({ busCode: 'KS602', live: live() })
    act(() => result.current.setOffline(true))
    rerender({ input: { busCode: 'KS602', live: live({ stale: true }) } })
    expect(notify).toHaveBeenCalledTimes(1)
    expect(result.current.alerts[0].kind).toBe('offline')
  })

  it('reports unsupported notifications without crashing', () => {
    vi.stubGlobal('Notification', undefined)
    const { result } = setup({ busCode: 'KS602', live: live(), user })
    expect(result.current.permission).toBe('unsupported')
    act(() => result.current.setOffline(true)) // would request permission if supported
  })

  it('asks for notification permission when an alert is armed', async () => {
    const requestPermission = vi.fn(async () => 'granted' as NotificationPermission)
    vi.stubGlobal('Notification', Object.assign(function () {}, { permission: 'default', requestPermission }))
    const { result } = setup({ busCode: 'KS602', live: live({ latitude: 11 }), user })
    await act(async () => result.current.setApproachMinutes(5))
    expect(requestPermission).toHaveBeenCalledTimes(1)
    expect(result.current.permission).toBe('granted')
  })
})

describe('showSystemNotification', () => {
  const alert = { kind: 'approach' as const, title: 'Bus KS602 is almost here', body: 'About 3 min away.' }

  it('shows a notification when permission is granted', async () => {
    const created: unknown[] = []
    vi.stubGlobal(
      'Notification',
      Object.assign(
        function (this: unknown, title: string, opts: NotificationOptions) {
          created.push({ title, ...opts })
        },
        { permission: 'granted' }
      )
    )
    expect(await showSystemNotification(alert, 'KS602')).toBe(true)
    expect(created).toEqual([expect.objectContaining({ title: alert.title, body: alert.body, tag: 'ethiyo-KS602-approach' })])
  })

  it('stays silent without permission', async () => {
    const ctor = vi.fn()
    vi.stubGlobal('Notification', Object.assign(ctor, { permission: 'denied' }))
    expect(await showSystemNotification(alert, 'KS602')).toBe(false)
    expect(ctor).not.toHaveBeenCalled()
  })
})
