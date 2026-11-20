import { EventSourceLike } from '../hooks/useBusTracker'

/** Scriptable stand-in for EventSource, shared by hook tests. */
export class FakeEventSource implements EventSourceLike {
  static instances: FakeEventSource[] = []
  readyState = 0
  onerror: ((ev: Event) => void) | null = null
  onopen: ((ev: Event) => void) | null = null
  closed = false
  private listeners: Record<string, ((ev: MessageEvent) => void)[]> = {}

  constructor(public url: string) {
    FakeEventSource.instances.push(this)
  }
  static latest(): FakeEventSource {
    return FakeEventSource.instances[FakeEventSource.instances.length - 1]
  }
  static reset() {
    FakeEventSource.instances = []
  }
  addEventListener(type: string, l: (ev: MessageEvent) => void) {
    ;(this.listeners[type] ||= []).push(l)
  }
  close() {
    this.closed = true
    this.readyState = 2
  }
  open() {
    this.readyState = 1
    this.onopen?.(new Event('open'))
  }
  emit(type: string, data: unknown) {
    this.listeners[type]?.forEach(l => l(new MessageEvent(type, { data: JSON.stringify(data) })))
  }
  /** permanently=true mimics the browser giving up (readyState CLOSED). */
  fail(permanently: boolean) {
    this.readyState = permanently ? 2 : 0
    this.onerror?.(new Event('error'))
  }
}

export const fakeFactory = (url: string) => new FakeEventSource(url)
