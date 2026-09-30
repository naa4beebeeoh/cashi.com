import { render, screen } from '@testing-library/react-native';
import App from '../App';

describe('App', () => {
  it('renders the main cashi heading and check connection action', () => {
    render(<App />);

    expect(screen.getByText('cashi')).toBeTruthy();
    expect(screen.getByText('Check connection')).toBeTruthy();
  });
});
