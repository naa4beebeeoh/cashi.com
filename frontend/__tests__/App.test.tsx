import { fireEvent, render, screen, waitFor } from '@testing-library/react-native';
import App from '../App';

const fetchMock = jest.fn();

beforeEach(() => {
  fetchMock.mockReset();
  global.fetch = fetchMock;
});

describe('App', () => {
  it('renders the main cashi heading and check connection action', () => {
    render(<App />);

    expect(screen.getByText('cashi')).toBeTruthy();
    expect(screen.getByText('Check connection')).toBeTruthy();
  });

  it('loads the card balance and recent transactions', async () => {
    fetchMock
      .mockResolvedValueOnce({ ok: true, json: async () => [{ id: 'primary' }] })
      .mockResolvedValueOnce({ ok: true, json: async () => ({ id: 'primary', lastFour: '4821', currentBalance: 1248.6, availableCredit: 3751.4, paymentDueDate: '2026-10-18' }) })
      .mockResolvedValueOnce({ ok: true, json: async () => [{ id: 'txn-1', merchant: 'Metro Coffee', category: 'Food & drink', amount: 5.8, date: '2026-09-28' }] });

    render(<App />);
    fireEvent.press(screen.getByText('Load card overview'));

    await waitFor(() => expect(screen.getByText('Metro Coffee')).toBeTruthy());
    expect(screen.getByText('$1,248.60')).toBeTruthy();
  });

  it('switches to trading and loads portfolio positions', async () => {
    fetchMock.mockResolvedValueOnce({ ok: true, json: async () => ({ name: 'Long-term portfolio', marketValue: 24860.32, dailyChange: 184.76, dailyChangePercent: 0.75, positions: [{ symbol: 'VTI', name: 'Total Stock Market ETF', shares: 42, marketValue: 11382, dailyChange: 92.4 }] }) });

    render(<App />);
    fireEvent.press(screen.getByText('Trading'));
    fireEvent.press(screen.getByText('Load portfolio'));

    await waitFor(() => expect(screen.getByText('VTI')).toBeTruthy());
    expect(screen.getByText('$24,860.32')).toBeTruthy();
  });
});
