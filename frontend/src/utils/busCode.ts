// Mirrors the backend's ValidateBusCode (internal/models/bus.go).
const BUS_CODE = /^[A-Za-z0-9_-]{1,64}$/

export function normalizeCode(code: string): string {
  return code.trim().toUpperCase()
}

export function isValidBusCode(code: string): boolean {
  return BUS_CODE.test(code.trim())
}
