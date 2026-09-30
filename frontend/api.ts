function resolveApiUrl() {
  const configuredUrl = process.env.EXPO_PUBLIC_API_URL;
  if (configuredUrl) return configuredUrl;

  if (typeof window !== 'undefined' && window.location?.origin) {
    return window.location.origin.replace('-8081.', '-8080.');
  }

  return 'http://localhost:8080';
}

const apiUrl = resolveApiUrl();

export type CardSummary = {
  id: string;
  name: string;
  lastFour: string;
  status: string;
  currentBalance: number;
  availableCredit: number;
  creditLimit: number;
  paymentDueDate: string;
};

export type Transaction = {
  id: string;
  merchant: string;
  category: string;
  amount: number;
  date: string;
};

export type Position = {
  symbol: string;
  name: string;
  shares: number;
  marketValue: number;
  dailyChange: number;
};

export type Portfolio = {
  name: string;
  marketValue: number;
  dailyChange: number;
  dailyChangePercent: number;
  positions: Position[];
};

export async function requestJSON<T>(path: string): Promise<T> {
  const response = await fetch(`${apiUrl}${path}`);
  if (!response.ok) {
    throw new Error(`API returned ${response.status}`);
  }
  return response.json() as Promise<T>;
}

export const api = {
  health: () => requestJSON<{ status?: string }>('/healthz'),
  cards: () => requestJSON<CardSummary[]>('/api/v1/cards'),
  cardSummary: (cardID: string) => requestJSON<CardSummary>(`/api/v1/cards/${cardID}/summary`),
  transactions: (cardID: string) => requestJSON<Transaction[]>(`/api/v1/cards/${cardID}/transactions`),
  portfolio: () => requestJSON<Portfolio>('/api/v1/portfolio'),
};

export { apiUrl };
