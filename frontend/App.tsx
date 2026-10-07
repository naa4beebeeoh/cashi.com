import { useEffect, useState } from 'react';
import {
  ActivityIndicator,
  Pressable,
  SafeAreaView,
  ScrollView,
  StatusBar,
  StyleSheet,
  Text,
  TextInput,
  View,
} from 'react-native';

import {
  api,
  apiUrl,
  Campaign,
  CashbackSummary,
  LedgerEntry,
  PaymentResult,
} from './api';

/** Brand tokens aligned with cashi.com / Cashi card app. */
const colors = {
  orange: '#FF5C00',
  orangeDeep: '#E24F00',
  white: '#FFFFFF',
  black: '#171719',
  muted: '#6B6B70',
  line: '#E8E8EA',
  danger: '#B42318',
  track: '#F0F0F2',
};

const USERS = [
  { id: 'user_a', label: 'Ayu' },
  { id: 'user_b', label: 'Budi' },
] as const;

const idr = new Intl.NumberFormat('id-ID', {
  style: 'currency',
  currency: 'IDR',
  maximumFractionDigits: 0,
});

function money(value: number) {
  return idr.format(value);
}

function reasonLabel(reason: string) {
  switch (reason) {
    case 'awarded':
      return 'Cashback awarded';
    case 'below_minimum':
      return 'Below Rp20.000 minimum — no cashback';
    case 'daily_cap_reached':
      return 'Daily cap reached — no cashback';
    case 'campaign_budget_exhausted':
      return 'Campaign budget exhausted — no cashback';
    case 'partial_daily_cap':
      return 'Partial award (daily cap)';
    case 'partial_budget':
      return 'Partial award (campaign budget)';
    default:
      return reason;
  }
}

export default function App() {
  const [userId, setUserId] = useState<string>(USERS[0].id);
  const [connection, setConnection] = useState<'idle' | 'ok' | 'error'>('idle');
  const [loading, setLoading] = useState(false);
  const [campaign, setCampaign] = useState<Campaign | null>(null);
  const [summary, setSummary] = useState<CashbackSummary | null>(null);
  const [ledger, setLedger] = useState<LedgerEntry[]>([]);
  const [payAmount, setPayAmount] = useState('100000');
  const [redeemAmount, setRedeemAmount] = useState('');
  const [lastPayment, setLastPayment] = useState<PaymentResult | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function refresh(nextUser = userId) {
    setLoading(true);
    setError(null);
    try {
      const [camp, bal, entries] = await Promise.all([
        api.campaign(),
        api.cashback(nextUser),
        api.ledger(nextUser),
      ]);
      setCampaign(camp);
      setSummary(bal);
      setLedger(entries);
      setConnection('ok');
      if (!redeemAmount && bal.availableIdr > 0) {
        setRedeemAmount(String(bal.availableIdr));
      }
    } catch (e) {
      setConnection('error');
      setError(e instanceof Error ? e.message : 'Failed to load');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    refresh(userId);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [userId]);

  async function onPay() {
    const amount = Number(payAmount);
    if (!Number.isFinite(amount) || amount <= 0) {
      setError('Enter a positive payment amount in IDR');
      return;
    }
    setLoading(true);
    setError(null);
    setMessage(null);
    try {
      const result = await api.pay(userId, Math.floor(amount));
      setLastPayment(result);
      setMessage(`${reasonLabel(result.awardReason)} · ${money(result.cashbackIdr)}`);
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Payment failed');
    } finally {
      setLoading(false);
    }
  }

  async function onRedeem() {
    const amount = Number(redeemAmount);
    if (!Number.isFinite(amount) || amount <= 0) {
      setError('Enter a positive redeem amount in IDR');
      return;
    }
    setLoading(true);
    setError(null);
    setMessage(null);
    try {
      const result = await api.redeem(userId, Math.floor(amount));
      setMessage(`Redeemed ${money(result.amountIdr)}. Available ${money(result.availableIdr)}`);
      setRedeemAmount('');
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Redeem failed');
    } finally {
      setLoading(false);
    }
  }

  const budgetPct = campaign
    ? Math.min(100, (campaign.budgetSpentIdr / Math.max(campaign.budgetTotalIdr, 1)) * 100)
    : 0;

  return (
    <SafeAreaView style={styles.safeArea}>
      <StatusBar barStyle="dark-content" backgroundColor={colors.white} />
      <ScrollView contentContainerStyle={styles.scrollContent}>
        <View style={styles.nav}>
          <Text style={styles.navWordmark}>cashi</Text>
          <Text style={styles.navMeta}>
            {connection === 'ok' ? 'ready' : connection === 'error' ? 'offline' : '…'}
          </Text>
        </View>

        <View style={styles.card}>
          <Text style={styles.cardLogo}>cashi</Text>
          <View style={styles.cardChip} />
          <View style={styles.cardFooter}>
            <View>
              <Text style={styles.cardProduct}>Flash Cashback</Text>
              <Text style={styles.cardRate}>5% · demo</Text>
            </View>
            <Text style={styles.cardNetwork}>VISA</Text>
          </View>
        </View>

        <Text style={styles.headline}>Flash Cashback</Text>
        <Text style={styles.subhead}>
          5% on payments ≥ {money(20000)}. Daily cap {money(50000)}. Campaign budget{' '}
          {money(10000000)}.
        </Text>
        <Text style={styles.meta}>API {apiUrl}</Text>

        <View style={styles.users}>
          {USERS.map((u) => {
            const active = userId === u.id;
            return (
              <Pressable
                key={u.id}
                onPress={() => setUserId(u.id)}
                style={[styles.userChip, active && styles.userChipActive]}
              >
                <Text style={[styles.userChipText, active && styles.userChipTextActive]}>
                  {u.label}
                </Text>
              </Pressable>
            );
          })}
          <Pressable onPress={() => refresh()} style={styles.refreshBtn}>
            <Text style={styles.refreshText}>Refresh</Text>
          </Pressable>
        </View>

        {campaign ? (
          <View style={styles.block}>
            <Text style={styles.sectionLabel}>Campaign</Text>
            <Text style={styles.campaignStatus}>
              {campaign.status === 'active' ? 'Active' : 'Exhausted'} · left{' '}
              {money(campaign.budgetLeftIdr)}
            </Text>
            <View style={styles.barTrack}>
              <View style={[styles.barFill, { width: `${budgetPct}%` }]} />
            </View>
          </View>
        ) : null}

        {summary ? (
          <View style={styles.block}>
            <Text style={styles.sectionLabel}>Your cashback</Text>
            <Text style={styles.balance}>{money(summary.availableIdr)}</Text>
            <Text style={styles.balanceMeta}>
              Earned today {money(summary.earnedTodayIdr)} / {money(summary.dailyCapIdr)} · Redeemed{' '}
              {money(summary.redeemedIdr)}
            </Text>
          </View>
        ) : null}

        <View style={styles.block}>
          <Text style={styles.sectionLabel}>Make a payment</Text>
          <TextInput
            value={payAmount}
            onChangeText={setPayAmount}
            keyboardType="number-pad"
            style={styles.input}
            placeholder="Amount IDR"
            placeholderTextColor={colors.muted}
          />
          <Pressable onPress={onPay} style={styles.primaryBtn} disabled={loading}>
            <Text style={styles.primaryBtnText}>Pay</Text>
          </Pressable>
          {lastPayment ? (
            <Text style={styles.hint}>
              Last: paid {money(lastPayment.amountIdr)} → {money(lastPayment.cashbackIdr)} cashback
            </Text>
          ) : null}
        </View>

        <View style={styles.block}>
          <Text style={styles.sectionLabel}>Redeem</Text>
          <TextInput
            value={redeemAmount}
            onChangeText={setRedeemAmount}
            keyboardType="number-pad"
            style={styles.input}
            placeholder="Amount IDR"
            placeholderTextColor={colors.muted}
          />
          <Pressable onPress={onRedeem} style={styles.secondaryBtn} disabled={loading}>
            <Text style={styles.secondaryBtnText}>Redeem to payout</Text>
          </Pressable>
        </View>

        {loading ? <ActivityIndicator style={{ marginTop: 12 }} color={colors.orange} /> : null}
        {message ? <Text style={styles.message}>{message}</Text> : null}
        {error ? <Text style={styles.error}>{error}</Text> : null}

        <View style={styles.block}>
          <Text style={styles.sectionLabel}>Ledger</Text>
          {ledger.length === 0 ? (
            <Text style={styles.hint}>No entries yet.</Text>
          ) : (
            ledger.map((e) => (
              <View key={e.id} style={styles.ledgerRow}>
                <Text style={styles.ledgerType}>
                  {e.entryType === 'cashback_credit' ? '+ credit' : '− redeem'}
                </Text>
                <Text style={styles.ledgerAmount}>{money(e.amountIdr)}</Text>
              </View>
            ))
          )}
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safeArea: { flex: 1, backgroundColor: colors.white },
  scrollContent: { paddingHorizontal: 20, paddingTop: 8, paddingBottom: 48 },
  nav: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: 16,
  },
  navWordmark: {
    fontSize: 22,
    fontWeight: '800',
    color: colors.black,
    letterSpacing: -0.8,
    textTransform: 'lowercase',
  },
  navMeta: { fontSize: 12, color: colors.muted, fontWeight: '600' },
  card: {
    backgroundColor: colors.orange,
    borderRadius: 18,
    paddingHorizontal: 22,
    paddingTop: 22,
    paddingBottom: 18,
    aspectRatio: 1.586,
    maxHeight: 210,
    justifyContent: 'space-between',
    marginBottom: 24,
    shadowColor: colors.orangeDeep,
    shadowOpacity: 0.28,
    shadowRadius: 16,
    shadowOffset: { width: 0, height: 10 },
    elevation: 6,
  },
  cardLogo: {
    color: colors.white,
    fontSize: 26,
    fontWeight: '800',
    letterSpacing: -0.6,
  },
  cardChip: {
    width: 42,
    height: 32,
    borderRadius: 6,
    backgroundColor: 'rgba(255,255,255,0.35)',
    marginTop: 8,
  },
  cardFooter: {
    flexDirection: 'row',
    alignItems: 'flex-end',
    justifyContent: 'space-between',
  },
  cardProduct: { color: colors.white, fontSize: 15, fontWeight: '700' },
  cardRate: { color: 'rgba(255,255,255,0.85)', fontSize: 12, marginTop: 2 },
  cardNetwork: {
    color: colors.white,
    fontSize: 18,
    fontWeight: '800',
    letterSpacing: 2,
  },
  headline: {
    fontSize: 32,
    fontWeight: '800',
    color: colors.black,
    letterSpacing: -1,
    marginBottom: 8,
  },
  subhead: { fontSize: 15, lineHeight: 22, color: colors.black, marginBottom: 6, opacity: 0.85 },
  meta: { fontSize: 12, color: colors.muted, marginBottom: 16 },
  users: { flexDirection: 'row', alignItems: 'center', gap: 8, marginBottom: 20, flexWrap: 'wrap' },
  userChip: {
    paddingHorizontal: 16,
    paddingVertical: 9,
    borderRadius: 999,
    backgroundColor: colors.white,
    borderWidth: 1.5,
    borderColor: colors.black,
  },
  userChipActive: { backgroundColor: colors.black },
  userChipText: { color: colors.black, fontWeight: '700' },
  userChipTextActive: { color: colors.white },
  refreshBtn: { marginLeft: 'auto', paddingVertical: 8, paddingHorizontal: 4 },
  refreshText: { color: colors.orange, fontWeight: '700' },
  block: { marginBottom: 22 },
  sectionLabel: {
    fontSize: 12,
    fontWeight: '700',
    letterSpacing: 1,
    color: colors.muted,
    textTransform: 'uppercase',
    marginBottom: 6,
  },
  campaignStatus: { fontSize: 16, fontWeight: '700', color: colors.black, marginBottom: 8 },
  barTrack: { height: 8, backgroundColor: colors.track, borderRadius: 4, overflow: 'hidden' },
  barFill: { height: 8, backgroundColor: colors.orange },
  balance: { fontSize: 40, fontWeight: '800', color: colors.black, letterSpacing: -1 },
  balanceMeta: { fontSize: 13, color: colors.muted, marginTop: 4 },
  input: {
    backgroundColor: colors.white,
    borderWidth: 1.5,
    borderColor: colors.line,
    borderRadius: 14,
    paddingHorizontal: 14,
    paddingVertical: 12,
    fontSize: 18,
    marginBottom: 10,
    color: colors.black,
  },
  primaryBtn: {
    backgroundColor: colors.orange,
    borderRadius: 999,
    paddingVertical: 15,
    alignItems: 'center',
  },
  primaryBtnText: { color: colors.white, fontWeight: '800', fontSize: 16 },
  secondaryBtn: {
    backgroundColor: colors.white,
    borderWidth: 1.5,
    borderColor: colors.black,
    borderRadius: 999,
    paddingVertical: 14,
    alignItems: 'center',
  },
  secondaryBtnText: { color: colors.black, fontWeight: '800', fontSize: 16 },
  hint: { marginTop: 8, color: colors.muted, fontSize: 13 },
  message: { marginTop: 8, color: colors.black, fontWeight: '700' },
  error: { marginTop: 8, color: colors.danger, fontWeight: '700' },
  ledgerRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    paddingVertical: 12,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: colors.line,
  },
  ledgerType: { color: colors.muted, fontWeight: '600' },
  ledgerAmount: { color: colors.black, fontWeight: '800' },
});
