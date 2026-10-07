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

const emptySummary = {
  userId: 'user_a',
  availableIdr: 0,
  redeemedIdr: 0,
  earnedTodayIdr: 0,
  dailyCapIdr: 50_000,
  dailyLeftIdr: 50_000,
};

describe('Flash Cashback app', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedApi.campaign.mockResolvedValue({
      id: 'flash_v1',
      name: 'Flash Cashback',
      rateBps: 500,
      minPaymentIdr: 20_000,
      dailyCapIdr: 50_000,
      budgetTotalIdr: 10_000_000,
      budgetSpentIdr: 0,
      budgetLeftIdr: 10_000_000,
      status: 'active',
      timezone: 'Asia/Jakarta',
    });
    mockedApi.cashback.mockResolvedValue({ ...emptySummary });
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

  it('pays Spotify gift card then redeems cashback (happy path)', async () => {
    let availableIdr = 0;
    mockedApi.cashback.mockImplementation(async () => ({
      userId: 'user_a',
      availableIdr,
      redeemedIdr: availableIdr === 0 && mockedApi.redeem.mock.calls.length > 0 ? 5_000 : 0,
      earnedTodayIdr: mockedApi.pay.mock.calls.length > 0 ? 5_000 : 0,
      dailyCapIdr: 50_000,
      dailyLeftIdr: 45_000,
    }));
    mockedApi.pay.mockImplementation(async () => {
      availableIdr = 5_000;
      return {
        paymentId: 'pay-1',
        userId: 'user_a',
        amountIdr: 100_000,
        cashbackIdr: 5_000,
        awardReason: 'awarded',
        availableIdr: 5_000,
        earnedTodayIdr: 5_000,
        budgetLeftIdr: 9_995_000,
        idempotentReplay: false,
      };
    });
    mockedApi.redeem.mockImplementation(async () => {
      availableIdr = 0;
      return {
        redemptionId: 'red-1',
        userId: 'user_a',
        amountIdr: 5_000,
        availableIdr: 0,
        redeemedIdr: 5_000,
        idempotentReplay: false,
      };
    });

    const { getByLabelText, findByText, findByLabelText } = render(<App />);
    // Wait until initial refresh finishes — Pay/Redeem are disabled while loading.
    await findByText('Your cashback wallet');
    const payBtn = await findByLabelText('Pay gift card');
    expect(payBtn.props.accessibilityState?.disabled).not.toBe(true);

    fireEvent.press(payBtn);

    await waitFor(() => {
      expect(mockedApi.pay).toHaveBeenCalledTimes(1);
      expect(mockedApi.pay).toHaveBeenCalledWith('user_a', 100_000);
    });
    expect(await findByText(/5\.000 cashback reward/)).toBeTruthy();
    expect(await findByText('Cashback awarded')).toBeTruthy();

    // After pay, refresh auto-fills redeem with available balance.
    await waitFor(() => {
      expect(availableIdr).toBe(5_000);
    });
    await findByText('Your cashback wallet');

    fireEvent.press(getByLabelText('Redeem to payout'));

    await waitFor(() => {
      expect(mockedApi.redeem).toHaveBeenCalledTimes(1);
      expect(mockedApi.redeem).toHaveBeenCalledWith('user_a', 5_000);
    });
    await waitFor(() => {
      expect(availableIdr).toBe(0);
    });
  });
});
