import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { TransitApi } from './api'

describe('TransitApi', () => {
  const originalFetch = window.fetch

  beforeEach(() => {
    vi.restoreAllMocks()
  })

  afterEach(() => {
    window.fetch = originalFetch
  })

  describe('trackBus', () => {
    it('requests bus tracking with trimmed uppercase code and returns data', async () => {
      const mockTrackingData = {
        bus_code: 'KS602',
        tracking_url: 'https://chalo.com/track/ks602',
        position: {
          latitude: 12.9716,
          longitude: 77.5946,
          speed_kmh: 32,
        },
      }

      window.fetch = vi.fn().mockResolvedValue({
        ok: true,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ success: true, data: mockTrackingData }),
      } as Response)

      const res = await TransitApi.trackBus('  ks602  ')

      expect(window.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/api/v1/bus/KS602/track'),
        expect.objectContaining({
          headers: expect.objectContaining({ Accept: 'application/json' }),
        })
      )
      expect(res).toEqual(mockTrackingData)
    })

    it('throws ApiError with server error details on failure', async () => {
      window.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 404,
        statusText: 'Not Found',
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({
          error: {
            code: 'BUS_NOT_FOUND',
            message: 'Vehicle not registered or out of service',
          },
        }),
      } as Response)

      await expect(TransitApi.trackBus('INVALID99')).rejects.toMatchObject({
        name: 'ApiError',
        code: 'BUS_NOT_FOUND',
        status: 404,
        message: 'Vehicle not registered or out of service',
      })
    })
  })

  describe('getRoute', () => {
    it('fetches route details and returns data correctly', async () => {
      const mockRouteData = {
        bus_code: 'DL1PC0001',
        tracking_url: 'https://chalo.com/route-live',
        route_url: 'https://chalo.com/routedetailslive',
      }

      window.fetch = vi.fn().mockResolvedValue({
        ok: true,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ data: mockRouteData }),
      } as Response)

      const res = await TransitApi.getRoute('dl1pc0001')

      expect(window.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/api/v1/bus/DL1PC0001/route'),
        expect.anything()
      )
      expect(res).toEqual(mockRouteData)
    })
  })

  describe('checkHealth', () => {
    it('returns health payload with calculated latencyMs', async () => {
      window.fetch = vi.fn().mockResolvedValue({
        ok: true,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({
          status: 'healthy',
          message: 'Transit gateway operating',
          browser: true,
        }),
      } as Response)

      const res = await TransitApi.checkHealth()

      expect(window.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/health'),
        expect.anything()
      )
      expect(res.status).toBe('healthy')
      expect(typeof res.latencyMs).toBe('number')
      expect(res.latencyMs).toBeGreaterThanOrEqual(0)
    })
  })

  describe('error handling', () => {
    it('maps network failures to NETWORK_ERROR ApiError', async () => {
      window.fetch = vi.fn().mockRejectedValue(new Error('Failed to fetch'))

      await expect(TransitApi.trackBus('KS602')).rejects.toMatchObject({
        name: 'ApiError',
        code: 'NETWORK_ERROR',
        status: 0,
        message: 'Failed to fetch',
      })
    })

    it('maps abort signals to TIMEOUT ApiError', async () => {
      const abortError = new DOMException('The operation was aborted', 'AbortError')
      window.fetch = vi.fn().mockRejectedValue(abortError)

      await expect(TransitApi.trackBus('KS602')).rejects.toMatchObject({
        name: 'ApiError',
        code: 'TIMEOUT',
        status: 504,
        message: 'Request timed out while connecting to transit service',
      })
    })
  })
})
