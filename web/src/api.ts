export interface Counterparty {
  id: string
  name: string
  phone: string
  note: string
  tags: string[]
  created_at: string
  receivable_cents: number
  payable_cents: number
}

export interface Trade {
  id: string
  counterparty_id: string
  type: string
  direction: string
  total_cents: number
  paid_cents: number
  remaining_cents: number
  note: string
  created_at: string
  items: TradeItem[]
}

export interface TradeItem {
  name: string
  qty: number
  price_cents: number
}

export interface TimelineEntry {
  kind: string
  id: string
  trade_id?: string
  offset_id?: string
  amount_cents: number
  note: string
  at: string
}

export interface Offset {
  id: string
  counterparty_id: string
  receivable_trade_id: string
  payable_trade_id: string
  amount_cents: number
  note: string
  occurred_at: string
  receivable_remaining_cents: number
  payable_remaining_cents: number
}

export interface Overview {
  receivable_cents: number
  payable_cents: number
  recent: Trade[]
}

export const yuan = (cents: number) => (cents / 100).toFixed(2)

const AUTH_STORAGE_KEY = 'trade-ledger-password'
let pagePassword = ''
let passwordRequired = false

function storedPassword(): string {
	try {
		return window.sessionStorage.getItem(AUTH_STORAGE_KEY) ?? pagePassword
	} catch {
		return pagePassword
  }
}

function notifyAuthRequired() {
	passwordRequired = true
  if (typeof window !== 'undefined') window.dispatchEvent(new Event('trade-ledger-auth-required'))
}

export const auth = {
  get: storedPassword,
  required: () => passwordRequired,
  set: (password: string) => {
    passwordRequired = false
    pagePassword = password
    try {
      window.sessionStorage.setItem(AUTH_STORAGE_KEY, password)
    } catch {
      // Session storage may be unavailable; the request still uses this page session.
    }
  },
  clear: () => {
    pagePassword = ''
    try {
      window.sessionStorage.removeItem(AUTH_STORAGE_KEY)
    } catch {
      // Nothing else is needed when storage is unavailable.
    }
  },
}

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers)
  headers.set('Content-Type', 'application/json')
  const password = storedPassword()
  if (password) headers.set('Authorization', `Bearer ${password}`)
  const res = await fetch(path, {
    ...init,
    headers,
  })
  if (res.status === 401) {
    auth.clear()
    notifyAuthRequired()
  }
  if (!res.ok) throw new Error(`${res.status} ${await res.text()}`)
  return res.json() as Promise<T>
}

export const api = {
  overview: () => req<Overview>('/api/overview'),
  counterparties: () => req<Counterparty[]>('/api/counterparties'),
  createCounterparty: (counterparty: { name: string; phone?: string; note?: string; tags?: string[] }) =>
    req<Counterparty>('/api/counterparties', {
      method: 'POST',
      body: JSON.stringify(counterparty),
    }),
  trades: (counterparty_id: string) => req<Trade[]>(`/api/trades?counterparty_id=${counterparty_id}`),
  createTrade: (counterparty_id: string, type: string, total_cents: number, note: string, direction?: string, items: TradeItem[] = []) =>
    req<Trade>('/api/trades', {
      method: 'POST',
      body: JSON.stringify({ counterparty_id, type, total_cents, note, direction, items }),
    }),
  pay: (trade_id: string, amount_cents: number) =>
    req<{ paid_cents: number }>(`/api/trades/${trade_id}/payments`, {
      method: 'POST',
      body: JSON.stringify({ amount_cents }),
    }),
  timeline: (id: string) => req<TimelineEntry[]>(`/api/counterparties/${id}/timeline`),
  offset: (receivable_trade_id: string, payable_trade_id: string, amount_cents: number, note = '') =>
    req<Offset>('/api/offsets', {
      method: 'POST',
      body: JSON.stringify({ receivable_trade_id, payable_trade_id, amount_cents, note }),
    }),
}
