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

const apiUrl = process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8080';

type ConnectionState = 'idle' | 'checking' | 'connected' | 'error';

export default function App() {
  const [connection, setConnection] = useState<ConnectionState>('idle');

  async function checkConnection() {
    setConnection('checking');
    try {
      const response = await fetch(`${apiUrl}/healthz`);
      if (!response.ok) throw new Error('API returned an error');
      const body: { status?: string } = await response.json();
      setConnection(body.status === 'ok' ? 'connected' : 'error');
    } catch {
      setConnection('error');
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

        <View style={styles.content}>
          <Text style={styles.eyebrow}>YOUR MONEY, IN MOTION</Text>
          <Text style={styles.title}>A little more{ '\n' }room to grow.</Text>
          <Text style={styles.description}>
            Your Cashi experience starts here. The app is ready to connect to your API.
          </Text>

          <View style={styles.connection}>
            <View style={styles.connectionHeader}>
              <Text style={styles.connectionLabel}>BACKEND CONNECTION</Text>
              <View style={styles.statusGroup}>
                {connection === 'checking' ? (
                  <ActivityIndicator size="small" color="#257a58" />
                ) : (
                  <View style={[styles.dot, connection === 'connected' && styles.dotConnected, connection === 'error' && styles.dotError]} />
                )}
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
  content: { flex: 1, justifyContent: 'center', paddingBottom: 38 },
  eyebrow: { marginBottom: 16, color: '#287854', fontSize: 11, fontWeight: '700', letterSpacing: 1.5 },
  title: { color: '#17392b', fontSize: 42, fontWeight: '600', lineHeight: 48 },
  description: { maxWidth: 310, marginTop: 16, color: '#60756a', fontSize: 16, lineHeight: 24 },
  connection: {
    marginTop: 38,
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