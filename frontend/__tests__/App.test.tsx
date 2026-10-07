import React from 'react';
import { fireEvent, render, waitFor } from '@testing-library/react-native';

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

  it('shows Spotify purchase flow and hides campaign debug until Dev opens', async () => {
    const { getByText, getByLabelText, queryByText, queryAllByText } = render(<App />);
    await waitFor(() => {
      expect(getByText('Pay with Cashi. Earn cashback.')).toBeTruthy();
      expect(getByText('Spotify Gift Card')).toBeTruthy();
      expect(getByText(/spend from/)).toBeTruthy();
    });
    expect(queryAllByText(/Shopee/).length).toBe(0);
    expect(queryByText('Developer tools')).toBeNull();
    expect(queryByText(/Spent/)).toBeNull();

    fireEvent.press(getByLabelText('Open developer tools'));
    await waitFor(() => {
      expect(getByText('Developer tools')).toBeTruthy();
      expect(getByText(/Active/)).toBeTruthy();
      expect(getByText(/Interview \/ ops helpers/)).toBeTruthy();
    });
  });
});
