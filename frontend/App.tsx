import { useEffect, useMemo, useRef, useState } from 'react';
import {
  ActivityIndicator,
  Keyboard,
  KeyboardAvoidingView,
  Modal,
  Platform,
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
  reward: '#22C55E',
  drawerScrim: 'rgba(23,23,25,0.35)',
};

const USERS = [
  { id: 'user_a', label: 'Ayu' },
  { id: 'user_b', label: 'Budi' },
] as const;

/** Demo merchandise — amount is editable so payment ↔ reward pairs are easy to stage. */
const PRODUCT = {
  name: 'Spotify Gift Card',
  hint: 'Digital code · choose your load amount',
  defaultAmount: '100000',
} as const;

const RATE_BPS = 500;
const MIN_PAYMENT_IDR = 20_000;
const DAILY_CAP_IDR = 50_000;
/** Soft client max for gift-card load (avoids absurd fat-finger amounts). */
const MAX_PAYMENT_IDR = 2_000_000;

function digitsOnlyAmount(raw: string, max: number): string {
  const digits = raw.replace(/\D/g, '');
  if (digits === '') {
    return '';
  }
  const n = Number(digits);
  if (!Number.isFinite(n)) {
    return '';
  }
  return String(Math.min(n, max));
}

const idr = new Intl.NumberFormat('id-ID', {
  style: 'currency',
  currency: 'IDR',
  maximumFractionDigits: 0,
});

function money(value: number) {
  return idr.format(value);
}

/** Client preview only — server still clamps by daily cap / campaign budget. */
function estimateCashback(amountIdr: number): number {
  if (!Number.isFinite(amountIdr) || amountIdr < MIN_PAYMENT_IDR) {
    return 0;
  }
  return Math.floor((amountIdr * RATE_BPS) / 10_000);
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
  const [debugOpen, setDebugOpen] = useState(false);
  const [connection, setConnection] = useState<'idle' | 'ok' | 'error'>('idle');
  const [loading, setLoading] = useState(false);
  const [campaign, setCampaign] = useState<Campaign | null>(null);
  const [summary, setSummary] = useState<CashbackSummary | null>(null);
  const [ledger, setLedger] = useState<LedgerEntry[]>([]);
  const [payAmount, setPayAmount] = useState(PRODUCT.defaultAmount);
  const [redeemAmount, setRedeemAmount] = useState('');
  const [lastPayment, setLastPayment] = useState<PaymentResult | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [keyboardPad, setKeyboardPad] = useState(0);
  /** Only Redeem needs scroll-to-end; Pay amount is higher on the screen. */
  const focusedFieldRef = useRef<'pay' | 'redeem' | null>(null);
  const scrollRef = useRef<ScrollView>(null);

  const amountNum = Number(payAmount);
  const previewCashback = useMemo(() => estimateCashback(amountNum), [amountNum]);

  useEffect(() => {
    const showEvent = Platform.OS === 'ios' ? 'keyboardWillShow' : 'keyboardDidShow';
    const hideEvent = Platform.OS === 'ios' ? 'keyboardWillHide' : 'keyboardDidHide';
    const showSub = Keyboard.addListener(showEvent, (e) => {
      setKeyboardPad(e.endCoordinates.height);
      if (focusedFieldRef.current === 'redeem') {
        setTimeout(() => scrollRef.current?.scrollToEnd({ animated: true }), 50);
      }
    });
    const hideSub = Keyboard.addListener(hideEvent, () => setKeyboardPad(0));
    return () => {
      showSub.remove();
      hideSub.remove();
    };
  }, []);

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
      setError('Enter a gift card amount');
      return;
    }
    if (amount > MAX_PAYMENT_IDR) {
      setError(`Max gift card amount is ${money(MAX_PAYMENT_IDR)}`);
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
  const activeUser = USERS.find((u) => u.id === userId)?.label ?? userId;

  const androidTopInset = Platform.OS === 'android' ? (StatusBar.currentHeight ?? 28) : 0;

  return (
    <SafeAreaView style={[styles.safeArea, { paddingTop: androidTopInset }]}>
      <StatusBar barStyle="dark-content" backgroundColor={colors.white} translucent={false} />
      <KeyboardAvoidingView
        style={styles.flex}
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
        keyboardVerticalOffset={androidTopInset}
      >
        <ScrollView
          ref={scrollRef}
          contentContainerStyle={[styles.scrollContent, { paddingBottom: 48 + keyboardPad }]}
          keyboardShouldPersistTaps="handled"
          keyboardDismissMode="on-drag"
          automaticallyAdjustKeyboardInsets={Platform.OS === 'ios'}
        >
        <View style={styles.nav}>
          <Text style={styles.navWordmark}>cashi</Text>
          <Pressable
            onPress={() => setDebugOpen(true)}
            style={styles.debugBtn}
            accessibilityLabel="Open developer tools"
          >
            <Text style={styles.debugBtnText}>Dev</Text>
          </Pressable>
        </View>

        <View style={styles.cardWrap}>
          <View style={styles.card}>
            <Text style={styles.cardLogo}>cashi</Text>
            <View style={styles.cardChip} />
            <View style={styles.cardFooter}>
              <View>
                <Text style={styles.cardProduct}>Flash Cashback</Text>
                <Text style={styles.cardRate}>5% on eligible spend</Text>
              </View>
              <Text style={styles.cardNetwork}>VISA</Text>
            </View>
          </View>
        </View>

        <Text style={styles.eyebrow}>Spend & earn</Text>
        <Text style={styles.headline}>Pay with Cashi. Earn cashback.</Text>
        <Text style={styles.subhead}>
          Buy a Spotify gift card and get rewarded automatically on eligible purchases.
        </Text>

        <View style={styles.merchantCard}>
          <Text style={styles.merchantName}>{PRODUCT.name}</Text>
          <Text style={styles.merchantHint}>{PRODUCT.hint}</Text>

          <View style={styles.amountRow}>
            <TextInput
              value={payAmount}
              onChangeText={(t) => setPayAmount(digitsOnlyAmount(t, MAX_PAYMENT_IDR))}
              keyboardType="number-pad"
              inputMode="numeric"
              maxLength={String(MAX_PAYMENT_IDR).length}
              style={styles.amountInput}
              placeholder="0"
              placeholderTextColor={colors.muted}
              onFocus={() => {
                focusedFieldRef.current = 'pay';
              }}
              onBlur={() => {
                if (focusedFieldRef.current === 'pay') {
                  focusedFieldRef.current = null;
                }
              }}
            />
            <Text style={styles.amountCurrency}>IDR</Text>
          </View>

          <Text style={styles.receiveLine}>
            {"You'll earn ≈ "}
            <Text style={styles.receiveStrong}>
              {previewCashback > 0 ? money(previewCashback) : 'Rp0'} cashback
            </Text>
          </Text>
          <Text style={styles.tnc}>
            {`Terms apply. Earn 5% cashback when you spend from ${money(MIN_PAYMENT_IDR)}. Up to ${money(DAILY_CAP_IDR)} cashback per day. Your reward may be less if today's limit or the campaign budget is running low.`}
          </Text>

          <Pressable
            onPress={onPay}
            style={styles.primaryBtn}
            disabled={loading}
            accessibilityLabel="Pay gift card"
            accessibilityState={{ disabled: loading }}
          >
            <Text style={styles.primaryBtnText}>
              Pay {Number.isFinite(amountNum) && amountNum > 0 ? money(amountNum) : '…'}
            </Text>
          </Pressable>
        </View>

        {lastPayment ? (
          <View style={styles.rewardBanner}>
            <Text style={styles.rewardMerchant}>{PRODUCT.name}</Text>
            <View style={styles.rewardPill}>
              <Text style={styles.rewardPillText}>
                {money(lastPayment.cashbackIdr)} cashback reward!
              </Text>
            </View>
            <Text style={styles.rewardMeta}>{reasonLabel(lastPayment.awardReason)}</Text>
          </View>
        ) : null}

        {summary ? (
          <View style={styles.block}>
            <Text style={styles.sectionLabel}>Your cashback wallet</Text>
            <Text style={styles.balance}>{money(summary.availableIdr)}</Text>
            <Text style={styles.balanceMeta}>
              Earned today {money(summary.earnedTodayIdr)} / {money(summary.dailyCapIdr)}
            </Text>
          </View>
        ) : null}

        <View style={styles.block}>
          <Text style={styles.sectionLabel}>Redeem</Text>
          <TextInput
            value={redeemAmount}
            onChangeText={(t) => setRedeemAmount(digitsOnlyAmount(t, MAX_PAYMENT_IDR))}
            keyboardType="number-pad"
            inputMode="numeric"
            maxLength={String(MAX_PAYMENT_IDR).length}
            style={styles.input}
            placeholder="Amount IDR"
            placeholderTextColor={colors.muted}
            onFocus={() => {
              focusedFieldRef.current = 'redeem';
              requestAnimationFrame(() => {
                setTimeout(() => scrollRef.current?.scrollToEnd({ animated: true }), 80);
              });
            }}
            onBlur={() => {
              if (focusedFieldRef.current === 'redeem') {
                focusedFieldRef.current = null;
              }
            }}
          />
          <Pressable
            onPress={onRedeem}
            style={styles.secondaryBtn}
            disabled={loading}
            accessibilityLabel="Redeem to payout"
            accessibilityState={{ disabled: loading }}
          >
            <Text style={styles.secondaryBtnText}>Redeem to payout</Text>
          </Pressable>
        </View>

        {loading ? <ActivityIndicator style={{ marginTop: 12 }} color={colors.orange} /> : null}
        {message && !lastPayment ? <Text style={styles.message}>{message}</Text> : null}
        {error ? <Text style={styles.error}>{error}</Text> : null}
        </ScrollView>
      </KeyboardAvoidingView>

      <Modal
        visible={debugOpen}
        animationType="slide"
        transparent
        onRequestClose={() => setDebugOpen(false)}
      >
        <View style={styles.drawerRoot}>
          <Pressable style={styles.drawerScrim} onPress={() => setDebugOpen(false)} />
          <SafeAreaView style={styles.drawer}>
            <ScrollView contentContainerStyle={styles.drawerContent}>
              <View style={styles.drawerHeader}>
                <Text style={styles.drawerTitle}>Developer tools</Text>
                <Pressable onPress={() => setDebugOpen(false)}>
                  <Text style={styles.drawerClose}>Close</Text>
                </Pressable>
              </View>
              <Text style={styles.drawerNote}>
                Interview / ops helpers — not part of the customer experience.
              </Text>

              <Text style={styles.sectionLabel}>Session</Text>
              <Text style={styles.debugLine}>
                API {apiUrl} · {connection === 'ok' ? 'ready' : connection === 'error' ? 'offline' : '…'}
              </Text>
              <Text style={styles.debugLine}>Acting as {activeUser} ({userId})</Text>

              <Text style={[styles.sectionLabel, { marginTop: 16 }]}>Switch user</Text>
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
                  <Text style={styles.sectionLabel}>Campaign (DB)</Text>
                  <Text style={styles.campaignStatus}>
                    {campaign.status === 'active' ? 'Active' : 'Exhausted'} · left{' '}
                    {money(campaign.budgetLeftIdr)}
                  </Text>
                  <Text style={styles.debugLine}>
                    Spent {money(campaign.budgetSpentIdr)} / {money(campaign.budgetTotalIdr)}
                  </Text>
                  <View style={styles.barTrack}>
                    <View style={[styles.barFill, { width: `${budgetPct}%` }]} />
                  </View>
                </View>
              ) : null}

              {summary ? (
                <View style={styles.block}>
                  <Text style={styles.sectionLabel}>Daily usage (DB)</Text>
                  <Text style={styles.debugLine}>
                    Earned today {money(summary.earnedTodayIdr)} / {money(summary.dailyCapIdr)} ·
                    redeemed lifetime {money(summary.redeemedIdr)}
                  </Text>
                </View>
              ) : null}

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
        </View>
      </Modal>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safeArea: { flex: 1, backgroundColor: colors.white },
  flex: { flex: 1 },
  scrollContent: { paddingHorizontal: 20, paddingTop: 12, flexGrow: 1 },
  nav: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: 16,
    marginTop: 4,
  },
  navWordmark: {
    fontSize: 22,
    fontWeight: '800',
    color: colors.black,
    letterSpacing: -0.8,
    textTransform: 'lowercase',
  },
  debugBtn: {
    paddingHorizontal: 12,
    paddingVertical: 6,
    borderRadius: 999,
    borderWidth: 1,
    borderColor: colors.line,
    backgroundColor: colors.track,
  },
  debugBtnText: { fontSize: 12, fontWeight: '800', color: colors.muted, letterSpacing: 0.5 },
  cardWrap: {
    width: '100%',
    alignItems: 'center',
    marginBottom: 22,
  },
  card: {
    backgroundColor: colors.orange,
    borderRadius: 18,
    paddingHorizontal: 22,
    paddingTop: 22,
    paddingBottom: 18,
    // Standard card ratio; avoid maxHeight — it breaks width/centering on Android.
    width: '92%',
    maxWidth: 340,
    aspectRatio: 1.586,
    alignSelf: 'center',
    justifyContent: 'space-between',
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
  eyebrow: {
    fontSize: 12,
    fontWeight: '800',
    letterSpacing: 1,
    color: colors.orange,
    textTransform: 'uppercase',
    marginBottom: 6,
  },
  headline: {
    fontSize: 28,
    fontWeight: '800',
    color: colors.black,
    letterSpacing: -1,
    marginBottom: 8,
  },
  subhead: { fontSize: 15, lineHeight: 22, color: colors.muted, marginBottom: 20 },
  sectionLabel: {
    fontSize: 12,
    fontWeight: '700',
    letterSpacing: 1,
    color: colors.muted,
    textTransform: 'uppercase',
    marginBottom: 8,
  },
  merchantCard: {
    borderWidth: 1,
    borderColor: colors.line,
    borderRadius: 18,
    padding: 16,
    marginBottom: 16,
    backgroundColor: colors.white,
  },
  merchantName: { fontSize: 18, fontWeight: '800', color: colors.black },
  merchantHint: { fontSize: 13, color: colors.muted, marginTop: 2, marginBottom: 16 },
  amountRow: { flexDirection: 'row', alignItems: 'flex-end', gap: 10, marginBottom: 8 },
  amountInput: {
    flex: 1,
    fontSize: 40,
    fontWeight: '800',
    color: colors.black,
    letterSpacing: -1,
    paddingVertical: 0,
    minWidth: 120,
  },
  amountCurrency: {
    fontSize: 28,
    fontWeight: '600',
    color: colors.muted,
    marginBottom: 6,
  },
  receiveLine: { fontSize: 15, color: colors.muted, marginBottom: 8 },
  receiveStrong: { color: colors.black, fontWeight: '800' },
  tnc: { fontSize: 12, lineHeight: 18, color: colors.muted, marginBottom: 16 },
  primaryBtn: {
    backgroundColor: colors.orange,
    borderRadius: 999,
    paddingVertical: 15,
    alignItems: 'center',
  },
  primaryBtnText: { color: colors.white, fontWeight: '800', fontSize: 16 },
  rewardBanner: {
    borderRadius: 16,
    padding: 14,
    marginBottom: 18,
    backgroundColor: colors.track,
  },
  rewardMerchant: { color: colors.black, fontWeight: '800', fontSize: 15, marginBottom: 8 },
  rewardPill: {
    alignSelf: 'flex-start',
    backgroundColor: colors.reward,
    borderRadius: 999,
    paddingHorizontal: 12,
    paddingVertical: 6,
    marginBottom: 6,
  },
  rewardPillText: { color: colors.white, fontWeight: '800', fontSize: 13 },
  rewardMeta: { fontSize: 12, color: colors.muted },
  block: { marginBottom: 22 },
  balance: { fontSize: 36, fontWeight: '800', color: colors.black, letterSpacing: -1 },
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
  users: { flexDirection: 'row', alignItems: 'center', gap: 8, marginBottom: 12, flexWrap: 'wrap' },
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
  campaignStatus: { fontSize: 16, fontWeight: '700', color: colors.black, marginBottom: 6 },
  barTrack: { height: 8, backgroundColor: colors.track, borderRadius: 4, overflow: 'hidden', marginTop: 8 },
  barFill: { height: 8, backgroundColor: colors.orange },
  debugLine: { fontSize: 13, color: colors.black, marginBottom: 4 },
  ledgerRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    paddingVertical: 12,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: colors.line,
  },
  ledgerType: { color: colors.muted, fontWeight: '600' },
  ledgerAmount: { color: colors.black, fontWeight: '800' },
  drawerRoot: { flex: 1, flexDirection: 'row' },
  drawerScrim: { flex: 1, backgroundColor: colors.drawerScrim },
  drawer: {
    width: '86%',
    maxWidth: 360,
    backgroundColor: colors.white,
    borderTopLeftRadius: 16,
    borderBottomLeftRadius: 16,
  },
  drawerContent: { padding: 20, paddingBottom: 40 },
  drawerHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: 6,
  },
  drawerTitle: { fontSize: 20, fontWeight: '800', color: colors.black },
  drawerClose: { color: colors.orange, fontWeight: '800' },
  drawerNote: { fontSize: 12, color: colors.muted, marginBottom: 18, lineHeight: 18 },
});
