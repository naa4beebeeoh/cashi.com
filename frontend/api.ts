import { Platform } from 'react-native';

function resolveApiUrl() {
  const configuredUrl = process.env.EXPO_PUBLIC_API_URL;
  let url = configuredUrl;

  if (!url && typeof window !== 'undefined' && window.location?.origin) {
    url = window.location.origin
      .replace('-8081.', '-8080.')
      .replace(':8081', ':8080');
  }

  if (!url) {
    url = 'http://localhost:8080';
  }

  // Android emulator: localhost is the emulator itself; host machine is 10.0.2.2
  if (
    Platform.OS === 'android' &&
    (url.includes('localhost') || url.includes('127.0.0.1'))
  ) {
    return url.replace('localhost', '10.0.2.2').replace('127.0.0.1', '10.0.2.2');
  }

  return url;
}

const apiUrl = resolveApiUrl();

export type Campaign = {
  id: string;
  name: string;
  rateBps: number;
  minPaymentIdr: number;
  dailyCapIdr: number;
  budgetTotalIdr: number;
  budgetSpentIdr: number;
  budgetLeftIdr: number;
  status: string;
  timezone: string;
};

export type CashbackSummary = {
  userId: string;
  availableIdr: number;
  redeemedIdr: number;
  earnedTodayIdr: number;
  dailyCapIdr: number;
  dailyLeftIdr: number;
};

export type PaymentResult = {
  paymentId: string;
  userId: string;
  amountIdr: number;
  cashbackIdr: number;
  awardReason: string;
  availableIdr: number;
  earnedTodayIdr: number;
  budgetLeftIdr: number;
  idempotentReplay: boolean;
};

export type RedeemResult = {
  redemptionId: string;
  userId: string;
  amountIdr: number;
  availableIdr: number;
  redeemedIdr: number;
  idempotentReplay: boolean;
};

export type LedgerEntry = {
  id: string;
  entryType: string;
  amountIdr: number;
  paymentId?: string;
  redemptionId?: string;
  createdAt: string;
};

async function requestJSON<T>(
  path: string,
  init?: RequestInit & { userId?: string; idempotencyKey?: string },
): Promise<T> {
  const headers = new Headers(init?.headers);
  headers.set('Content-Type', 'application/json');
  if (init?.userId) headers.set('X-User-ID', init.userId);
  if (init?.idempotencyKey) headers.set('Idempotency-Key', init.idempotencyKey);

  const response = await fetch(`${apiUrl}${path}`, {
    ...init,
    headers,
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    const message = typeof body?.error === 'string' ? body.error : `API returned ${response.status}`;
    throw new Error(message);
  }
  return body as T;
}

function newIdempotencyKey() {
  return `rn-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

export const api = {
  health: () => requestJSON<{ status?: string }>('/healthz'),
  ready: () => requestJSON<{ status?: string }>('/readyz'),
  campaign: () => requestJSON<Campaign>('/api/v1/campaign'),
  cashback: (userId: string) =>
    requestJSON<CashbackSummary>('/api/v1/me/cashback', { userId }),
  ledger: (userId: string) =>
    requestJSON<LedgerEntry[]>('/api/v1/me/ledger', { userId }),
  pay: (userId: string, amountIdr: number, idempotencyKey = newIdempotencyKey()) =>
    requestJSON<PaymentResult>('/api/v1/payments', {
      method: 'POST',
      userId,
      idempotencyKey,
      body: JSON.stringify({ amountIdr }),
    }),
  redeem: (userId: string, amountIdr: number, idempotencyKey = newIdempotencyKey()) =>
    requestJSON<RedeemResult>('/api/v1/redeem', {
      method: 'POST',
      userId,
      idempotencyKey,
      body: JSON.stringify({ amountIdr }),
    }),
};

export { apiUrl, newIdempotencyKey };
