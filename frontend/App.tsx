import { useState } from 'react';
import {
  ActivityIndicator,
  SafeAreaView,
  StatusBar,
  StyleSheet,
  Text,
  Pressable,
  View,
} from 'react-native';

import { api, apiUrl, CardSummary, Portfolio, Transaction } from './api';

type ConnectionState = 'idle' | 'checking' | 'connected' | 'error';
type Feature = 'card' | 'trading';
type LoadState = 'idle' | 'loading' | 'loaded' | 'error';

const currency = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' });

function money(value: number) {
  return currency.format(value);
}

export default function App() {
  const [feature, setFeature] = useState<Feature>('card');
  const [connection, setConnection] = useState<ConnectionState>('idle');
  const [cardState, setCardState] = useState<LoadState>('idle');
  const [card, setCard] = useState<CardSummary | null>(null);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [portfolioState, setPortfolioState] = useState<LoadState>('idle');
  const [portfolio, setPortfolio] = useState<Portfolio | null>(null);

  async function checkConnection() {
    setConnection('checking');
    try {
      const body = await api.health();
      setConnection(body.status === 'ok' ? 'connected' : 'error');
    } catch {
      setConnection('error');
    }
  }

  async function loadCard() {
    setCardState('loading');
    try {
      const cards = await api.cards();
      const selectedCard = cards[0];
      if (!selectedCard) throw new Error('No cards returned');
      const [summary, recentTransactions] = await Promise.all([
        api.cardSummary(selectedCard.id),
        api.transactions(selectedCard.id),
      ]);
      setCard(summary);
      setTransactions(recentTransactions);
      setCardState('loaded');
    } catch {
      setCardState('error');
    }
  }

  async function loadPortfolio() {
    setPortfolioState('loading');
    try {
      setPortfolio(await api.portfolio());
      setPortfolioState('loaded');
    } catch {
      setPortfolioState('error');
    }
  }

  const statusText = {
    idle: 'Not checked',
    checking: 'Connecting',
    connected: 'Connected',
    error: 'Unavailable',
  }[connection];

  return (
    <SafeAreaView style={styles.safeArea}>
      <StatusBar barStyle="dark-content" />
      <View style={styles.container}>
        <View style={styles.topLine}>
          <View style={styles.mark}><Text style={styles.markText}>c</Text></View>
          <Text style={styles.wordmark}>cashi</Text>
          <Text style={styles.version}>MOBILE / 01</Text>
        </View>

        <View style={styles.tabs}>
          <Pressable accessibilityRole="tab" accessibilityState={{ selected: feature === 'card' }} onPress={() => setFeature('card')} style={[styles.tab, feature === 'card' && styles.activeTab]}>
            <Text style={[styles.tabText, feature === 'card' && styles.activeTabText]}>Credit Card</Text>
          </Pressable>
          <Pressable accessibilityRole="tab" accessibilityState={{ selected: feature === 'trading' }} onPress={() => setFeature('trading')} style={[styles.tab, feature === 'trading' && styles.activeTab]}>
            <Text style={[styles.tabText, feature === 'trading' && styles.activeTabText]}>Trading</Text>
          </Pressable>
        </View>

        <View style={styles.content}>
          <Text style={styles.eyebrow}>{feature === 'card' ? 'CREDIT / EVERYDAY' : 'TRADING / LONG TERM'}</Text>
          <Text style={styles.title}>{feature === 'card' ? 'Your money, at a glance.' : 'Invest with a little more room.'}</Text>
          <Text style={styles.description}>
            {feature === 'card' ? 'See your balance, payment date, and latest spending in one calm view.' : 'A simple portfolio view for tracking the value and movement of your investments.'}
          </Text>

          {feature === 'card' ? (
            <CardJourney card={card} transactions={transactions} state={cardState} onLoad={loadCard} />
          ) : (
            <TradingJourney portfolio={portfolio} state={portfolioState} onLoad={loadPortfolio} />
          )}

          <View style={styles.connection}>
            <View style={styles.connectionHeader}>
              <Text style={styles.connectionLabel}>BACKEND CONNECTION</Text>
              <View style={styles.statusGroup}>
                {connection === 'checking' ? <ActivityIndicator size="small" color="#257a58" /> : <View style={[styles.dot, connection === 'connected' && styles.dotConnected, connection === 'error' && styles.dotError]} />}
                <Text style={styles.statusText}>{statusText}</Text>
              </View>
            </View>
            <Text style={styles.endpoint}>{apiUrl}/healthz</Text>
            <Pressable
              accessibilityRole="button"
              disabled={connection === 'checking'}
              onPress={checkConnection}
              style={({ pressed }) => [styles.button, pressed && styles.buttonPressed]}
            >
              <Text style={styles.buttonText}>Check connection</Text>
              <Text style={styles.buttonArrow}>↗</Text>
            </Pressable>
          </View>
        </View>

        <View style={styles.footer}>
          <Text style={styles.footerText}>BUILT FOR WHAT'S NEXT</Text>
          <Text style={styles.footerText}>CASHI 2026</Text>
        </View>
      </View>
    </SafeAreaView>
  );
}

function CardJourney({ card, transactions, state, onLoad }: { card: CardSummary | null; transactions: Transaction[]; state: LoadState; onLoad: () => void }) {
  return (
    <View style={styles.journey}>
      <View style={styles.sectionHeader}>
        <Text style={styles.sectionLabel}>CASHI EVERYDAY</Text>
        {card && <Text style={styles.cardNumber}>•••• {card.lastFour}</Text>}
      </View>
      {card ? (
        <>
          <View style={styles.balanceRow}>
            <View><Text style={styles.metricLabel}>CURRENT BALANCE</Text><Text style={styles.metricValue}>{money(card.currentBalance)}</Text></View>
            <View><Text style={styles.metricLabel}>AVAILABLE</Text><Text style={styles.metricValue}>{money(card.availableCredit)}</Text></View>
          </View>
          <Text style={styles.dueDate}>Payment due {card.paymentDueDate}</Text>
          <Text style={styles.listLabel}>RECENT TRANSACTIONS</Text>
          {transactions.map((transaction) => <View key={transaction.id} style={styles.listRow}><View><Text style={styles.listTitle}>{transaction.merchant}</Text><Text style={styles.listMeta}>{transaction.category} · {transaction.date}</Text></View><Text style={styles.listAmount}>-{money(transaction.amount)}</Text></View>)}
        </>
      ) : <Text style={styles.emptyText}>{state === 'error' ? 'Could not load your card right now.' : 'Load your card overview to see your latest activity.'}</Text>}
      <LoadButton label={state === 'error' ? 'Try again' : 'Load card overview'} loading={state === 'loading'} onPress={onLoad} />
    </View>
  );
}

function TradingJourney({ portfolio, state, onLoad }: { portfolio: Portfolio | null; state: LoadState; onLoad: () => void }) {
  return (
    <View style={styles.journey}>
      <View style={styles.sectionHeader}><Text style={styles.sectionLabel}>PORTFOLIO</Text><Text style={styles.cardNumber}>MARKET VIEW</Text></View>
      {portfolio ? (
        <>
          <Text style={styles.metricLabel}>TOTAL VALUE</Text>
          <Text style={styles.portfolioValue}>{money(portfolio.marketValue)}</Text>
          <Text style={styles.positive}>+{money(portfolio.dailyChange)} (+{portfolio.dailyChangePercent}%) today</Text>
          <Text style={styles.listLabel}>POSITIONS</Text>
          {portfolio.positions.map((position) => <View key={position.symbol} style={styles.listRow}><View><Text style={styles.listTitle}>{position.symbol}</Text><Text style={styles.listMeta}>{position.name} · {position.shares} shares</Text></View><View><Text style={styles.listAmount}>{money(position.marketValue)}</Text><Text style={styles.positiveSmall}>+{money(position.dailyChange)}</Text></View></View>)}
        </>
      ) : <Text style={styles.emptyText}>{state === 'error' ? 'Could not load your portfolio right now.' : 'Load your portfolio to see your positions.'}</Text>}
      <LoadButton label={state === 'error' ? 'Try again' : 'Load portfolio'} loading={state === 'loading'} onPress={onLoad} />
    </View>
  );
}

function LoadButton({ label, loading, onPress }: { label: string; loading: boolean; onPress: () => void }) {
  return <Pressable accessibilityRole="button" disabled={loading} onPress={onPress} style={({ pressed }) => [styles.button, pressed && styles.buttonPressed]}><Text style={styles.buttonText}>{loading ? 'Loading...' : label}</Text><Text style={styles.buttonArrow}>↗</Text></Pressable>;
}

const styles = StyleSheet.create({
  safeArea: { flex: 1, backgroundColor: '#f4f7f5' },
  container: { flex: 1, paddingHorizontal: 24 },
  topLine: {
    height: 68,
    flexDirection: 'row',
    alignItems: 'center',
    borderBottomWidth: 1,
    borderBottomColor: '#dce5df',
  },
  mark: {
    width: 30,
    height: 30,
    borderRadius: 9,
    backgroundColor: '#16764f',
    alignItems: 'center',
    justifyContent: 'center',
  },
  markText: { color: '#ffffff', fontSize: 21, fontWeight: '700', lineHeight: 25 },
  wordmark: { marginLeft: 9, color: '#183a2c', fontSize: 20, fontWeight: '700' },
  version: { marginLeft: 'auto', color: '#789085', fontSize: 10, fontWeight: '700', letterSpacing: 1 },
  tabs: { flexDirection: 'row', gap: 24, height: 54, alignItems: 'flex-end', borderBottomWidth: 1, borderBottomColor: '#dce5df' },
  tab: { height: 54, justifyContent: 'center', borderBottomWidth: 2, borderBottomColor: 'transparent' },
  activeTab: { borderBottomColor: '#16764f' },
  tabText: { color: '#789085', fontSize: 13, fontWeight: '600' },
  activeTabText: { color: '#16764f' },
  content: { flex: 1, paddingTop: 30, paddingBottom: 22 },
  eyebrow: { marginBottom: 16, color: '#287854', fontSize: 11, fontWeight: '700', letterSpacing: 1.5 },
  title: { color: '#17392b', fontSize: 34, fontWeight: '600', lineHeight: 40 },
  description: { maxWidth: 330, marginTop: 12, color: '#60756a', fontSize: 15, lineHeight: 22 },
  journey: { marginTop: 26, paddingTop: 18, borderTopWidth: 1, borderTopColor: '#dce5df' },
  sectionHeader: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  sectionLabel: { color: '#789085', fontSize: 10, fontWeight: '700', letterSpacing: 1 },
  cardNumber: { color: '#52695d', fontSize: 12, fontWeight: '600' },
  balanceRow: { flexDirection: 'row', gap: 42, marginTop: 18 },
  metricLabel: { color: '#789085', fontSize: 9, fontWeight: '700', letterSpacing: 1 },
  metricValue: { marginTop: 5, color: '#17392b', fontSize: 22, fontWeight: '600' },
  portfolioValue: { marginTop: 5, color: '#17392b', fontSize: 32, fontWeight: '600' },
  dueDate: { marginTop: 10, color: '#287854', fontSize: 12, fontWeight: '600' },
  positive: { marginTop: 5, color: '#23885c', fontSize: 13, fontWeight: '600' },
  positiveSmall: { marginTop: 3, color: '#23885c', fontSize: 11, textAlign: 'right' },
  listLabel: { marginTop: 22, marginBottom: 5, color: '#789085', fontSize: 9, fontWeight: '700', letterSpacing: 1 },
  listRow: { minHeight: 48, flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', borderBottomWidth: 1, borderBottomColor: '#e3ebe6' },
  listTitle: { color: '#294b3d', fontSize: 13, fontWeight: '600' },
  listMeta: { marginTop: 3, color: '#789085', fontSize: 11 },
  listAmount: { color: '#294b3d', fontSize: 13, fontWeight: '600', textAlign: 'right' },
  emptyText: { marginTop: 18, color: '#60756a', fontSize: 14, lineHeight: 21 },
  connection: {
    marginTop: 24,
    paddingTop: 19,
    borderTopWidth: 1,
    borderTopColor: '#dce5df',
  },
  connectionHeader: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  connectionLabel: { color: '#789085', fontSize: 10, fontWeight: '700', letterSpacing: 1 },
  statusGroup: { flexDirection: 'row', alignItems: 'center', gap: 7 },
  dot: { width: 8, height: 8, borderRadius: 4, backgroundColor: '#9caea4' },
  dotConnected: { backgroundColor: '#23885c' },
  dotError: { backgroundColor: '#c35748' },
  statusText: { color: '#365547', fontSize: 12, fontWeight: '600' },
  endpoint: { marginTop: 12, color: '#52695d', fontSize: 13 },
  button: {
    height: 52,
    marginTop: 18,
    paddingHorizontal: 17,
    borderRadius: 7,
    backgroundColor: '#16764f',
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  buttonPressed: { backgroundColor: '#105d3d' },
  buttonText: { color: '#ffffff', fontSize: 14, fontWeight: '600' },
  buttonArrow: { color: '#ffffff', fontSize: 19 },
  footer: {
    height: 50,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    borderTopWidth: 1,
    borderTopColor: '#dce5df',
  },
  footerText: { color: '#8a9c92', fontSize: 9, fontWeight: '700', letterSpacing: 1 },
});