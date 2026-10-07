import React from 'react';
import { render, waitFor } from '@testing-library/react-native';

import App from '../App';
import { api } from '../api';

jest.mock('../api', () => {
  const actual = jest.requireActual('../api');
  return {
    ...actual,
    api: {
      health: jest.fn(),
      ready: jest.fn(),
      campaign: jest.fn(),
      cashback: jest.fn(),
      ledger: jest.fn(),
      pay: jest.fn(),
      redeem: jest.fn(),
    },
  };
});

const mockedApi = api as jest.Mocked<typeof api>;

describe('Flash Cashback app', () => {
  beforeEach(() => {
    mockedApi.campaign.mockResolvedValue({
      id: 'flash_v1',
      name: 'Flash Cashback',
      rateBps: 500,
      minPaymentIdr: 20000,
      dailyCapIdr: 50000,
      budgetTotalIdr: 10000000,
      budgetSpentIdr: 0,
      budgetLeftIdr: 10000000,
      status: 'active',
      timezone: 'Asia/Jakarta',
    });
    mockedApi.cashback.mockResolvedValue({
      userId: 'user_a',
      availableIdr: 0,
      redeemedIdr: 0,
      earnedTodayIdr: 0,
      dailyCapIdr: 50000,
      dailyLeftIdr: 50000,
    });
    mockedApi.ledger.mockResolvedValue([]);
  });

  it('loads campaign and cashback balance', async () => {
    const { getByText, getAllByText } = render(<App />);
    await waitFor(() => {
      expect(getAllByText('Flash Cashback').length).toBeGreaterThan(0);
      expect(getByText(/Active/)).toBeTruthy();
      expect(getAllByText('cashi').length).toBeGreaterThan(0);
    });
  });
});
