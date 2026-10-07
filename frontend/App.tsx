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

  return (
    <SafeAreaView style={styles.safeArea}>
      <StatusBar barStyle="dark-content" />
      <ScrollView contentContainerStyle={styles.scrollContent}>
        <View style={styles.topLine}>
          <View style={styles.mark}>
            <Text style={styles.markText}>c</Text>
          </View>
          <Text style={styles.wordmark}>cashi</Text>
          <Text style={styles.version}>FLASH / 01</Text>
        </View>

        <Text style={styles.headline}>Flash Cashback</Text>
        <Text style={styles.subhead}>
          5% on payments ≥ {money(20000)}. Daily cap {money(50000)}. Campaign budget{' '}
          {money(10000000)}.
        </Text>

        <Text style={styles.meta}>
          API {apiUrl} · {connection === 'ok' ? 'ready' : connection === 'error' ? 'offline' : '…'}
        </Text>

        <View style={styles.users}>
          {USERS.map((u) => (
            <Pressable
              key={u.id}
              onPress={() => setUserId(u.id)}
              style={[styles.userChip, userId === u.id && styles.userChipActive]}
            >
              <Text style={[styles.userChipText, userId === u.id && styles.userChipTextActive]}>
                {u.label}
              </Text>
            </Pressable>
          ))}
          <Pressable onPress={() => refresh()} style={styles.refreshBtn}>
            <Text style={styles.refreshText}>Refresh</Text>
          </Pressable>
        </View>

        {campaign ? (
          <View style={styles.campaign}>
            <Text style={styles.sectionLabel}>Campaign</Text>
            <Text style={styles.campaignStatus}>
              {campaign.status === 'active' ? 'Active' : 'Exhausted'} · left{' '}
              {money(campaign.budgetLeftIdr)}
            </Text>
            <View style={styles.barTrack}>
              <View
                style={[
                  styles.barFill,
                  {
                    width: `${Math.min(
                      100,
                      (campaign.budgetSpentIdr / Math.max(campaign.budgetTotalIdr, 1)) * 100,
                    )}%`,
                  },
                ]}
              />
            </View>
          </View>
        ) : null}

        {summary ? (
          <View style={styles.balanceBlock}>
            <Text style={styles.sectionLabel}>Your cashback</Text>
            <Text style={styles.balance}>{money(summary.availableIdr)}</Text>
            <Text style={styles.balanceMeta}>
              Earned today {money(summary.earnedTodayIdr)} / {money(summary.dailyCapIdr)} · Redeemed{' '}
              {money(summary.redeemedIdr)}
            </Text>
          </View>
        ) : null}

        <View style={styles.panel}>
          <Text style={styles.sectionLabel}>Make a payment</Text>
          <TextInput
            value={payAmount}
            onChangeText={setPayAmount}
            keyboardType="number-pad"
            style={styles.input}
            placeholder="Amount IDR"
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

        <View style={styles.panel}>
          <Text style={styles.sectionLabel}>Redeem</Text>
          <TextInput
            value={redeemAmount}
            onChangeText={setRedeemAmount}
            keyboardType="number-pad"
            style={styles.input}
            placeholder="Amount IDR"
          />
          <Pressable onPress={onRedeem} style={styles.secondaryBtn} disabled={loading}>
            <Text style={styles.secondaryBtnText}>Redeem to payout</Text>
          </Pressable>
        </View>

        {loading ? <ActivityIndicator style={{ marginTop: 12 }} /> : null}
        {message ? <Text style={styles.message}>{message}</Text> : null}
        {error ? <Text style={styles.error}>{error}</Text> : null}

        <View style={styles.panel}>
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
  safeArea: { flex: 1, backgroundColor: '#F7F3EE' },
  scrollContent: { padding: 20, paddingBottom: 48 },
  topLine: { flexDirection: 'row', alignItems: 'center', gap: 10, marginBottom: 20 },
  mark: {
    width: 28,
    height: 28,
    borderRadius: 8,
    backgroundColor: '#0F3D2E',
    alignItems: 'center',
    justifyContent: 'center',
  },
  markText: { color: '#F7F3EE', fontWeight: '700', fontSize: 16 },
  wordmark: {
    fontSize: 22,
    fontWeight: '700',
    color: '#0F3D2E',
    letterSpacing: -0.5,
    flex: 1,
  },
  version: { fontSize: 11, color: '#6B7280', letterSpacing: 1 },
  headline: {
    fontSize: 34,
    fontWeight: '700',
    color: '#122017',
    letterSpacing: -1,
    marginBottom: 8,
  },
  subhead: { fontSize: 15, lineHeight: 22, color: '#3F4A43', marginBottom: 8 },
  meta: { fontSize: 12, color: '#6B7280', marginBottom: 16 },
  users: { flexDirection: 'row', alignItems: 'center', gap: 8, marginBottom: 18, flexWrap: 'wrap' },
  userChip: {
    paddingHorizontal: 14,
    paddingVertical: 8,
    borderRadius: 999,
    backgroundColor: '#E8E2D9',
  },
  userChipActive: { backgroundColor: '#0F3D2E' },
  userChipText: { color: '#122017', fontWeight: '600' },
  userChipTextActive: { color: '#F7F3EE' },
  refreshBtn: { marginLeft: 'auto', padding: 8 },
  refreshText: { color: '#0F3D2E', fontWeight: '600' },
  campaign: { marginBottom: 18 },
  sectionLabel: {
    fontSize: 12,
    fontWeight: '700',
    letterSpacing: 1,
    color: '#6B7280',
    textTransform: 'uppercase',
    marginBottom: 6,
  },
  campaignStatus: { fontSize: 16, fontWeight: '600', color: '#122017', marginBottom: 8 },
  barTrack: { height: 8, backgroundColor: '#E0D8CC', borderRadius: 4, overflow: 'hidden' },
  barFill: { height: 8, backgroundColor: '#C45C26' },
  balanceBlock: { marginBottom: 20 },
  balance: { fontSize: 40, fontWeight: '700', color: '#0F3D2E', letterSpacing: -1 },
  balanceMeta: { fontSize: 13, color: '#3F4A43', marginTop: 4 },
  panel: {
    marginBottom: 16,
    paddingTop: 4,
  },
  input: {
    backgroundColor: '#FFF',
    borderWidth: 1,
    borderColor: '#D9D1C5',
    borderRadius: 12,
    paddingHorizontal: 14,
    paddingVertical: 12,
    fontSize: 18,
    marginBottom: 10,
    color: '#122017',
  },
  primaryBtn: {
    backgroundColor: '#0F3D2E',
    borderRadius: 12,
    paddingVertical: 14,
    alignItems: 'center',
  },
  primaryBtnText: { color: '#F7F3EE', fontWeight: '700', fontSize: 16 },
  secondaryBtn: {
    backgroundColor: '#FFF',
    borderWidth: 1.5,
    borderColor: '#0F3D2E',
    borderRadius: 12,
    paddingVertical: 14,
    alignItems: 'center',
  },
  secondaryBtnText: { color: '#0F3D2E', fontWeight: '700', fontSize: 16 },
  hint: { marginTop: 8, color: '#6B7280', fontSize: 13 },
  message: { marginTop: 8, color: '#0F3D2E', fontWeight: '600' },
  error: { marginTop: 8, color: '#B42318', fontWeight: '600' },
  ledgerRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    paddingVertical: 10,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: '#D9D1C5',
  },
  ledgerType: { color: '#3F4A43', fontWeight: '500' },
  ledgerAmount: { color: '#122017', fontWeight: '700' },
});
