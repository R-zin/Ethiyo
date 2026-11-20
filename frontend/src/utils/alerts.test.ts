import { describe, it, expect } from 'vitest'
import { evaluateAlerts, INITIAL_ALERT_STATE, AlertConfig, AlertState } from './alerts'
import { BusLiveInfo } from '../types/transit'

const live = (over: Partial<BusLiveInfo> = {}): BusLiveInfo => ({
  bus_code: 'KS602',
  latitude: 10.03, // ~3.34 km north of `user`
  longitude: 76,
  speed: 36, // → ETA ≈ 6 min
  recorded_at: new Date().toISOString(),
  live_tracking: true,
  tracking_url: '',
  age_seconds: 5,
  stale: false,
  ...over,
})
const user = { latitude: 10, longitude: 76 }
const approach = (m: number): AlertConfig => ({ approachMinutes: m, offline: false })
const OFFLINE: AlertConfig = { approachMinutes: null, offline: true }

describe('approach alert', () => {
  it('fires when the naive ETA reaches the threshold', () => {
    expect(evaluateAlerts(approach(5), { busCode: 'KS602', live: live(), user }, INITIAL_ALERT_STATE).fired).toEqual([])
    const { fired } = evaluateAlerts(approach(10), { busCode: 'KS602', live: live(), user }, INITIAL_ALERT_STATE)
    expect(fired).toHaveLength(1)
    expect(fired[0].kind).toBe('approach')
    expect(fired[0].title).toBe('Bus KS602 is almost here')
    expect(fired[0].body).toContain('About 6 min away')
    expect(fired[0].body).toContain('straight line')
  })

  it('is one-shot', () => {
    const first = evaluateAlerts(approach(10), { busCode: 'KS602', live: live(), user }, INITIAL_ALERT_STATE)
    const second = evaluateAlerts(approach(10), { busCode: 'KS602', live: live({ latitude: 10.02 }), user }, first.state)
    expect(second.fired).toEqual([])
  })

  it('fires within the arrival radius even when the bus is stopped', () => {
    const { fired } = evaluateAlerts(
      approach(2),
      { busCode: 'KS602', live: live({ latitude: 10.0015, speed: 0 }), user },
      INITIAL_ALERT_STATE
    )
    expect(fired).toHaveLength(1)
    expect(fired[0].body).toMatch(/^It is \d+ m away\.$/)
  })

  it('never fires on untrustworthy data or without a location', () => {
    const cases = [
      { busCode: 'KS602', live: live({ stale: true }), user },
      { busCode: 'KS602', live: live(), degraded: true, user },
      { busCode: 'KS602', live: live(), user: null },
      { busCode: 'KS602', live: undefined, user },
    ]
    for (const inputs of cases) {
      expect(evaluateAlerts(approach(15), inputs, INITIAL_ALERT_STATE).fired).toEqual([])
    }
    expect(evaluateAlerts({ approachMinutes: null, offline: false }, { busCode: 'KS602', live: live(), user }, INITIAL_ALERT_STATE).fired).toEqual([])
  })
})

describe('offline alert', () => {
  const step = (state: AlertState, inputs: Partial<Parameters<typeof evaluateAlerts>[1]>) =>
    evaluateAlerts(OFFLINE, { busCode: 'KS602', ...inputs }, state)

  it('fires once per outage, only after the bus was seen reporting', () => {
    // Never seen live: going quiet is not news.
    let r = step(INITIAL_ALERT_STATE, { live: live({ stale: true }) })
    expect(r.fired).toEqual([])

    r = step(r.state, { live: live() }) // reporting
    r = step(r.state, { live: live({ stale: true }) })
    expect(r.fired.map(f => f.kind)).toEqual(['offline'])
    expect(r.fired[0].body).toContain('GPS has gone quiet')

    r = step(r.state, { live: live({ stale: true }) })
    expect(r.fired).toEqual([]) // same outage

    r = step(r.state, { live: live() }) // recovered: re-armed
    r = step(r.state, { live: live(), degraded: true })
    expect(r.fired).toHaveLength(1)
    expect(r.fired[0].body).toContain('Chalo is not responding')
  })

  it('tracks reporting state even while disabled', () => {
    const r = evaluateAlerts({ approachMinutes: null, offline: false }, { busCode: 'KS602', live: live() }, INITIAL_ALERT_STATE)
    expect(r.state.wasReporting).toBe(true)
    expect(r.fired).toEqual([])
  })
})
