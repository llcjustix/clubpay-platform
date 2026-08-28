import React, { useEffect, useMemo, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import {
  Activity,
  AlertCircle,
  Banknote,
  Building2,
  CheckCircle2,
  Copy,
  Clock3,
  CreditCard,
  Gamepad2,
  Network,
  KeyRound,
  LogOut,
  Monitor,
  Play,
  Plus,
  Power,
  QrCode,
  ReceiptText,
  RefreshCw,
  Search,
  Save,
  Send,
  Settings,
  Ticket,
  Trash2,
  Users,
  Wrench,
  X,
} from 'lucide-react';
import '@fontsource-variable/geist';
import '@fontsource-variable/geist-mono';
import './styles.css';

const API_BASE = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';
const ADMIN_REFRESH_MS = 2000;
const OWNER_REFRESH_MS = 5000;
const TOKEN_KEY = 'clubpay_token';
const CLUB_KEY = 'clubpay_club_id';
const NAVIGATION_EVENT = 'clubpay:navigate';

type Tariff = {
  id: string;
  zone_id: string;
  zone?: string;
  name: string;
  duration_minutes: number;
  price_tiyin: number;
  price_uzs: number;
  sort_order?: number;
  status?: string;
};

type PaymentProvider = 'payme' | 'click' | 'mock';

type QRPaymentProvider = {
  provider: PaymentProvider;
  label: string;
  configured: boolean;
  sandbox?: boolean;
  message: string;
};

type QRData = {
  qr_type?: 'static_pc' | 'session_extend' | string;
  club: { id: string; name: string };
  pc: { id: string; external_pc_id: string; number: number; label: string; status: string };
  zone: { id: string; name: string; hourly_price_tiyin: number; hourly_price_uzs: number };
  tariffs: Tariff[];
  payment_providers: QRPaymentProvider[];
  active_session?: { planned_ends_at?: string; remaining_seconds?: number; can_extend?: boolean } | null;
  telegram?: { bot_link?: string; bot_username?: string };
};

type VoucherCheck = {
  voucher_id: string;
  minutes_left: number;
  seconds_left: number;
  status: string;
  can_redeem?: boolean;
  pc_status?: string;
  zone?: { id: string; name: string };
};

type PlayerAuthResponse = {
  auth_token?: string;
  telegram_link?: string;
  status: 'active' | 'awaiting_contact' | 'verified' | 'expired' | string;
  expires_at?: string;
  player?: { id: string; phone: string; first_name?: string; balance_seconds: number };
};

type TelegramWebApp = {
  initData: string;
  initDataUnsafe?: { start_param?: string };
  ready: () => void;
  expand: () => void;
};

type PC = {
  id: string;
  external_pc_id: string;
  number: number;
  label: string;
	mac_address?: string;
  status: string;
  zone_id: string;
  zone: string;
  active_grant_id?: string;
  remaining_seconds?: number;
};

type ManagedPC = PC & {
  qr_token?: string;
  qr_url?: string;
};

type Zone = {
  id: string;
  name: string;
  hourly_price_tiyin: number;
  hourly_price_uzs: number;
  sort_order: number;
  status: string;
};

type ClubUser = {
  id: string;
  name: string;
  email: string;
  phone: string;
  role: string;
  status: string;
  scope?: 'club' | 'network';
};

type ClubSettings = {
  id: string;
  network_id: string;
  network_name: string;
  name: string;
  slug: string;
  legal_name: string;
  tin: string;
  address: string;
  timezone: string;
  status: string;
  click_merchant_id: string;
  click_service_id: string;
  click_merchant_user_id: string;
  click_secret_key: string;
  click_club_cntrg_id: string;
  click_platform_cntrg_id: string;
  payme_merchant_id: string;
  payme_secret_key: string;
  payme_club_receiver_id: string;
  payme_platform_receiver_id: string;
  platform_fee_bps: number;
  effective_platform_fee_bps?: number;
  ofd_mxik: string;
  ofd_package_code: string;
  ofd_service_name: string;
  ofd_unit_code: string;
  ofd_vat_percent: number;
  payment_connected?: boolean;
  click_connected?: boolean;
  payme_connected?: boolean;
  payouts_connected?: boolean;
  fiscal_connected?: boolean;
};

type ClubSettingsPayload = {
  club: ClubSettings;
  zones: Zone[];
  tariffs: Tariff[];
  pcs: ManagedPC[];
  users: ClubUser[];
};

type Order = {
  id: string;
  invoice_id: string;
  provider: string;
  provider_payment_id?: string;
  amount_uzs: number;
  duration_minutes: number;
  duration_seconds?: number;
  status: string;
  provider_status?: string;
  fiscal_status?: string;
  receipt_kind?: string;
  pc_label: string;
  external_pc_id?: string;
  tariff: string;
  checkout_url?: string;
  receipt_url?: string;
  created_at: string;
};

type CheckoutResponse = {
  checkout_url: string;
  order: {
    invoice_id: string;
    provider: PaymentProvider;
    amount_uzs: number;
    duration_minutes: number;
    duration_seconds?: number;
  };
};

type VoucherDelivery = {
  status: string;
  phone?: string;
  telegram_link?: string;
  link_expires_at?: string;
};

type TelegramPrompt = {
  link: string;
  phone?: string;
  code?: string;
  minutes?: number;
  seconds?: number;
  status?: string;
};

type Grant = {
  id: string;
  duration_minutes: number;
  duration_seconds?: number;
  status: string;
  source: string;
  pc_label: string;
  core_session_id?: string;
  planned_ends_at?: string;
  remaining_minutes: number;
  remaining_seconds?: number;
  last_error?: string;
  created_at: string;
};

type Summary = {
  online_revenue_uzs: number;
  cash_revenue_uzs: number;
  club_online_revenue_uzs: number;
  platform_fee_uzs: number;
  club_total_revenue_uzs: number;
  paid_orders: number;
  cash_sessions: number;
  active_grants: number;
};

type Catalog = {
  club_id: string;
  pcs: PC[];
  zones: Zone[];
  tariffs: Tariff[];
};

type AuthUser = {
  id: string;
  name: string;
  email: string;
  phone: string;
  global_role: string;
};

type ClubAccess = {
  id: string;
  network_id: string;
  network_name: string;
  name: string;
  slug: string;
  status: string;
  role: string;
};

type ClubNetwork = {
  id: string;
  name: string;
  slug: string;
  status: string;
  role: string;
};

type AuthPayload = {
  token?: string;
  expires_at?: string;
  user: AuthUser;
  clubs: ClubAccess[];
};

type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  icon?: React.ReactNode;
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger' | 'success';
  size?: 'sm' | 'md';
  full?: boolean;
};

const EMPTY_CLUB_FORM: ClubSettings = {
  id: '',
  network_id: '',
  network_name: '',
  name: '',
  slug: '',
  legal_name: '',
  tin: '',
  address: '',
  timezone: 'Asia/Tashkent',
  status: 'active',
  click_merchant_id: '',
  click_service_id: '',
  click_merchant_user_id: '',
  click_secret_key: '',
  click_club_cntrg_id: '',
  click_platform_cntrg_id: '',
  payme_merchant_id: '',
  payme_secret_key: '',
  payme_club_receiver_id: '',
  payme_platform_receiver_id: '',
  platform_fee_bps: 0,
  ofd_mxik: '',
  ofd_package_code: '',
  ofd_service_name: 'Компьютерное время',
  ofd_unit_code: '',
  ofd_vat_percent: 0,
};

function App() {
  const route = useRoute();
  const path = route.pathname;
  if (path.startsWith('/payment')) return <PaymentReturnPage />;
  if (path.startsWith('/miniapp')) return <MiniAppPage />;
  if (path.startsWith('/qr/')) return <QRPage token={path.split('/').pop() || ''} />;
  return <AuthenticatedApp path={path} />;
}

function useRoute() {
  const [route, setRoute] = useState(() => ({
    pathname: window.location.pathname,
    search: window.location.search,
    hash: window.location.hash,
  }));

  useEffect(() => {
    function syncRoute() {
      setRoute({
        pathname: window.location.pathname,
        search: window.location.search,
        hash: window.location.hash,
      });
    }

    window.addEventListener('popstate', syncRoute);
    window.addEventListener(NAVIGATION_EVENT, syncRoute);
    return () => {
      window.removeEventListener('popstate', syncRoute);
      window.removeEventListener(NAVIGATION_EVENT, syncRoute);
    };
  }, []);

  return route;
}

function navigateTo(href: string, replace = false) {
  const url = new URL(href, window.location.href);
  if (url.origin !== window.location.origin) {
    window.location.assign(url.toString());
    return;
  }

  const next = `${url.pathname}${url.search}${url.hash}`;
  const current = `${window.location.pathname}${window.location.search}${window.location.hash}`;
  if (next === current) return;

  if (replace) {
    window.history.replaceState({}, '', next);
  } else {
    window.history.pushState({}, '', next);
  }
  window.dispatchEvent(new Event(NAVIGATION_EVENT));
  window.scrollTo(0, 0);
}

function handleInternalLinkClick(event: React.MouseEvent<HTMLAnchorElement>, href: string) {
  if (
    event.defaultPrevented ||
    event.button !== 0 ||
    event.altKey ||
    event.ctrlKey ||
    event.metaKey ||
    event.shiftKey ||
    event.currentTarget.target
  ) {
    return;
  }

  const url = new URL(href, window.location.href);
  if (url.origin !== window.location.origin) return;

  event.preventDefault();
  navigateTo(href);
}

function AuthenticatedApp({ path }: { path: string }) {
  const [auth, setAuth] = useState<AuthPayload | null>(null);
  const [selectedClubID, setSelectedClubID] = useState(localStorage.getItem(CLUB_KEY) || '');
  const [loading, setLoading] = useState(Boolean(localStorage.getItem(TOKEN_KEY)));
  const [error, setError] = useState('');

  async function loadMe() {
    if (!localStorage.getItem(TOKEN_KEY)) {
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const payload = await api<AuthPayload>('/api/auth/me');
      setAuth(payload);
      const savedClubID = localStorage.getItem(CLUB_KEY) || selectedClubID;
      const firstClubID = payload.clubs.some((club) => club.id === savedClubID) ? savedClubID : payload.clubs[0]?.id || '';
      setSelectedClubID(firstClubID);
      if (firstClubID) localStorage.setItem(CLUB_KEY, firstClubID);
      else localStorage.removeItem(CLUB_KEY);
      setError('');
    } catch (err) {
      localStorage.removeItem(TOKEN_KEY);
      setAuth(null);
      setError(String((err as Error).message || err));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadMe();
  }, []);

  function handleLogin(payload: AuthPayload) {
    if (payload.token) localStorage.setItem(TOKEN_KEY, payload.token);
    const clubID = payload.clubs[0]?.id || '';
    if (clubID) localStorage.setItem(CLUB_KEY, clubID);
    setSelectedClubID(clubID);
    setAuth(payload);
    setError('');
  }

  function handleClubChange(clubID: string) {
    setSelectedClubID(clubID);
    localStorage.setItem(CLUB_KEY, clubID);
  }

  async function logout() {
    await api('/api/auth/logout', { method: 'POST' }).catch(() => undefined);
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(CLUB_KEY);
    setAuth(null);
    setSelectedClubID('');
  }

  if (loading) return <Centered text="Загружаем доступ" />;
  if (!auth) return <LoginPage onLogin={handleLogin} error={error} />;
  if (auth.clubs.length === 0 && auth.user.global_role !== 'super_admin') {
    return <Centered text="Нет доступа ни к одному клубу" />;
  }

  const currentClubID = selectedClubID || auth.clubs[0]?.id || '';
  const commonProps = { auth, selectedClubID: currentClubID, currentPath: path, onClubChange: handleClubChange, onLogout: logout };

  if (path.startsWith('/reports') || path.startsWith('/owner')) {
    if (!canViewOwner(auth, currentClubID)) return <AdminPage {...commonProps} currentPath="/admin" />;
    return <ReportsPage {...commonProps} />;
  }
  if (path.startsWith('/settings')) {
    if (!canViewSettings(auth, currentClubID)) return <AdminPage {...commonProps} currentPath="/admin" />;
    return <SettingsPage {...commonProps} onReloadAuth={loadMe} />;
  }
  if (path.startsWith('/admin')) {
    if (!canViewAdmin(auth, currentClubID)) return <Centered text="Недостаточно прав" />;
    return <AdminPage {...commonProps} />;
  }
  if (canViewOwner(auth, currentClubID)) {
    return <ReportsPage {...commonProps} currentPath="/reports" />;
  }
  if (!canViewOwner(auth, currentClubID) && !canViewSettings(auth, currentClubID) && canViewAdmin(auth, currentClubID)) {
    return <AdminPage {...commonProps} currentPath="/admin" />;
  }
  return <HomePage {...commonProps} />;
}

function LoginPage({ onLogin, error }: { onLogin: (payload: AuthPayload) => void; error?: string }) {
  const [login, setLogin] = useState('admin@clubpay.local');
  const [password, setPassword] = useState('admin123');
  const [message, setMessage] = useState(error || '');
  const [loading, setLoading] = useState(false);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setLoading(true);
    setMessage('');
    try {
      const payload = await api<AuthPayload>('/api/auth/login', {
        method: 'POST',
        body: JSON.stringify({ login, password }),
      });
      onLogin(payload);
    } catch (err) {
      setMessage(String((err as Error).message || err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="auth-shell">
      <header className="auth-brand">
        <div className="brand-mark" aria-hidden="true">CP</div>
        <div>
          <strong>ClubPay</strong>
          <span>Operations</span>
        </div>
      </header>
      <section className="auth-card" aria-labelledby="login-title">
        <div className="auth-card-head">
          <div>
            <p>Доступ к системе</p>
            <h1 id="login-title">Вход в панель</h1>
            <span>Рабочее пространство откроется согласно вашей роли.</span>
          </div>
        </div>
        <form className="stack" onSubmit={submit}>
          <label className="form-block">
            Логин
            <input
              autoComplete="username"
              value={login}
              onChange={(event) => setLogin(event.target.value)}
              placeholder="email или телефон"
            />
          </label>
          <label className="form-block">
            Пароль
            <input
              autoComplete="current-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              type="password"
            />
          </label>
          <Button full icon={loading ? <RefreshCw className="spin" size={18} /> : <KeyRound size={18} />} disabled={loading}>
            Войти
          </Button>
          {message && <Notice tone="danger">{message}</Notice>}
        </form>
      </section>
    </main>
  );
}

type WorkspaceProps = {
  auth: AuthPayload;
  selectedClubID: string;
  currentPath: string;
  onClubChange: (clubID: string) => void;
  onLogout: () => void;
};

function HomePage(props: WorkspaceProps) {
  const canOpenOwner = canViewOwner(props.auth, props.selectedClubID);
  const canOpenSettings = canViewSettings(props.auth, props.selectedClubID);

  return (
    <main className="shell workspace-shell">
      <WorkspaceHeader {...props} eyebrow="Clubpay" title="Рабочая панель" />
      <Panel>
        <div className="link-grid">
          {canOpenSettings && <LinkButton href="/settings/pcs" icon={<QrCode size={18} />}>QR-коды компьютеров</LinkButton>}
          <LinkButton href="/admin" icon={<Activity size={18} />}>Панель менеджера</LinkButton>
          {canOpenOwner && <LinkButton href="/reports" icon={<Banknote size={18} />}>Дашборд</LinkButton>}
          {canOpenSettings && <LinkButton href="/settings" icon={<Settings size={18} />}>Настройки клуба</LinkButton>}
        </div>
      </Panel>
    </main>
  );
}

function MiniAppPage() {
  const [auth, setAuth] = useState<PlayerAuthResponse | null>(null);
  const [launchError, setLaunchError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const miniApp = (window as Window & { Telegram?: { WebApp?: TelegramWebApp } }).Telegram?.WebApp;
    const qrToken = miniApp?.initDataUnsafe?.start_param?.trim() || '';
    if (!miniApp?.initData || !qrToken) {
      setLaunchError('Откройте Clubpay по QR-коду в Telegram. Эта страница работает только внутри приложения Telegram.');
      setLoading(false);
      return;
    }
    miniApp.ready();
    miniApp.expand();
    api<PlayerAuthResponse>('/api/player-auth/miniapp', {
      method: 'POST',
      body: JSON.stringify({ qr_token: qrToken, init_data: miniApp.initData }),
    })
      .then(setAuth)
      .catch((err) => setLaunchError(String(err.message || err)))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <Centered text="Открываем ваш профиль" />;
  if (launchError) return <MiniAppLaunchError text={launchError} />;
  const miniApp = (window as Window & { Telegram?: { WebApp?: TelegramWebApp } }).Telegram?.WebApp;
  const qrToken = miniApp?.initDataUnsafe?.start_param?.trim() || '';
  return <QRPage token={qrToken} miniApp initialPlayerAuth={auth} />;
}

function MiniAppLaunchError({ text }: { text: string }) {
  return <main className="miniapp-launch-error">
    <span className="miniapp-mark">Clubpay</span>
    <h1>Не удалось открыть профиль</h1>
    <p>{text}</p>
    <span>Отсканируйте QR-код компьютера ещё раз и выберите «Открыть в Telegram».</span>
  </main>;
}

function QRPage({ token, miniApp = false, initialPlayerAuth = null }: { token: string; miniApp?: boolean; initialPlayerAuth?: PlayerAuthResponse | null }) {
  const [data, setData] = useState<QRData | null>(null);
  const [selected, setSelected] = useState('');
  const [paymentProvider, setPaymentProvider] = useState<PaymentProvider>('payme');
  const [customAmount, setCustomAmount] = useState('');
  const [voucherCode, setVoucherCode] = useState('');
  const [voucherMessage, setVoucherMessage] = useState('');
  const [voucherError, setVoucherError] = useState('');
  const [voucherCheck, setVoucherCheck] = useState<VoucherCheck | null>(null);
  const [checkingVoucher, setCheckingVoucher] = useState(false);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [redeeming, setRedeeming] = useState(false);
  const [createdCheckout, setCreatedCheckout] = useState<CheckoutResponse | null>(null);
  const [playerAuth, setPlayerAuth] = useState<PlayerAuthResponse | null>(initialPlayerAuth);
  const [startingPlayerAuth, setStartingPlayerAuth] = useState(false);
  const [redeemingBalance, setRedeemingBalance] = useState(false);
  const [error, setError] = useState('');
  const [paymentEntry, setPaymentEntry] = useState<'choose' | 'guest'>('choose');
  const returnAuthToken = useMemo(() => new URLSearchParams(window.location.search).get('player_auth_token')?.trim() || '', []);

  function loadQR() {
    setLoading(true);
    api<QRData>(`/api/qr/${token}`)
      .then((payload) => {
        setData(payload);
        setSelected(payload.tariffs[0]?.id || '');
        const firstReadyProvider = payload.payment_providers.find((provider) => provider.configured)?.provider;
        setPaymentProvider(firstReadyProvider || 'payme');
      })
      .catch((err) => setError(String(err.message || err)))
      .finally(() => setLoading(false));
  }

  useEffect(loadQR, [token]);

  useEffect(() => {
    if (!initialPlayerAuth) return;
    setPlayerAuth(initialPlayerAuth);
    setPaymentEntry('choose');
  }, [initialPlayerAuth]);

  useEffect(() => {
    if (!returnAuthToken) return;
    let disposed = false;
    api<PlayerAuthResponse>(`/api/player-auth/${encodeURIComponent(returnAuthToken)}`)
      .then((status) => {
        if (!disposed) setPlayerAuth({ ...status, auth_token: returnAuthToken });
      })
      .catch((err) => {
        if (!disposed) setError(String(err.message || err));
      });
    return () => { disposed = true; };
  }, [returnAuthToken]);

  useEffect(() => {
    if (!playerAuth?.auth_token || playerAuth.status === 'verified' || playerAuth.status === 'expired') return;
    let disposed = false;
    const poll = async () => {
      try {
        const status = await api<PlayerAuthResponse>(`/api/player-auth/${encodeURIComponent(playerAuth.auth_token || '')}`);
        if (!disposed) setPlayerAuth((current) => current ? { ...current, ...status } : status);
      } catch (err) {
        if (!disposed) setError(String((err as Error).message || err));
      }
    };
    poll();
    const timer = window.setInterval(poll, 1800);
    return () => { disposed = true; window.clearInterval(timer); };
  }, [playerAuth?.auth_token, playerAuth?.status]);

  useEffect(() => {
    const code = voucherCode.trim();
    setVoucherMessage('');
    setVoucherError('');
    setVoucherCheck(null);
    if (code.length < 4) {
      setCheckingVoucher(false);
      return;
    }
    let cancelled = false;
    setCheckingVoucher(true);
    const timer = window.setTimeout(async () => {
      try {
        const payload = await api<VoucherCheck>('/api/vouchers/check', {
          method: 'POST',
          body: JSON.stringify({ code, qr_token: token }),
        });
        if (!cancelled) setVoucherCheck(payload);
      } catch (err) {
        if (!cancelled) setVoucherError(String((err as Error).message || err));
      } finally {
        if (!cancelled) setCheckingVoucher(false);
      }
    }, 450);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [voucherCode, token]);

  const selectedTariff = useMemo(() => data?.tariffs.find((tariff) => tariff.id === selected), [data, selected]);
  const providerOptions = useMemo(() => {
    return data?.payment_providers || [];
  }, [data]);
  const selectedProviderOption = providerOptions.find((provider) => provider.provider === paymentProvider);
  const paymentMethodReady = Boolean(selectedProviderOption?.configured);
  const customAmountUZS = parseUZSInput(customAmount);
  const checkoutAmountUZS = customAmountUZS || selectedTariff?.price_uzs || 0;
  const isSessionExtendQR = data?.qr_type === 'session_extend';
  const isSessionExtendable = data ? isSessionExtendableStatus(data.pc.status) : false;
  const canStartOrExtend = data ? isPayableStatus(data.pc.status) || (isSessionExtendable && isSessionExtendQR) : false;
  const isExtension = isSessionExtendable && isSessionExtendQR;
  const isStaticBusyQR = isSessionExtendable && !isSessionExtendQR;
  const canShowPlayerProfile = !isSessionExtendQR && !isStaticBusyQR;
  const busyUntilLabel = data?.active_session?.planned_ends_at ? formatTime(data.active_session.planned_ends_at) : '';
  const voucherReadyForAutoApply = Boolean(voucherCode.trim() && voucherCheck?.can_redeem);
  const voucherDurationSeconds = voucherCheck?.seconds_left || (voucherCheck?.minutes_left ? voucherCheck.minutes_left * 60 : 0);
  const playerBalanceSeconds = playerAuth?.status === 'verified' ? (playerAuth.player?.balance_seconds || 0) : 0;
  const hasVerifiedPlayer = playerAuth?.status === 'verified' && Boolean(playerAuth.player);
  const needsPaymentEntryChoice = canShowPlayerProfile && canStartOrExtend && !hasVerifiedPlayer && paymentEntry === 'choose';

  async function startPlayerAuth() {
    setStartingPlayerAuth(true);
    setError('');
    try {
      const auth = await api<PlayerAuthResponse>('/api/player-auth/start', { method: 'POST', body: JSON.stringify({ qr_token: token }) });
      setPlayerAuth(auth);
    } catch (err) {
      setError(String((err as Error).message || err));
    } finally {
      setStartingPlayerAuth(false);
    }
  }

  async function switchGuestToPlayerAuth() {
    setPaymentEntry('choose');
    await startPlayerAuth();
  }

  async function redeemPlayerBalance() {
    if (!playerAuth?.auth_token || !playerBalanceSeconds) return;
    setRedeemingBalance(true);
    setError('');
    try {
      const result = await api<{ seconds_used: number; extended?: boolean }>('/api/player-balance/redeem', {
        method: 'POST',
        body: JSON.stringify({ qr_token: token, player_auth_token: playerAuth.auth_token }),
      });
      setVoucherMessage(result.extended ? `Баланс применён: сессия продлена на ${formatDurationClock(result.seconds_used)}.` : `Баланс применён: сеанс начат на ${formatDurationClock(result.seconds_used)}.`);
      setPlayerAuth((current) => current?.player ? { ...current, player: { ...current.player, balance_seconds: 0 } } : current);
      loadQR();
    } catch (err) {
      setError(String((err as Error).message || err));
    } finally {
      setRedeemingBalance(false);
    }
  }

  async function createCheckout() {
    if (!selected && !customAmountUZS) return;
    if (!paymentMethodReady) {
      setError(selectedProviderOption?.message || 'Способ оплаты не настроен');
      return;
    }
    setCreating(true);
    setError('');
    setCreatedCheckout(null);
    try {
      const payload = await api<CheckoutResponse>('/api/checkouts', {
        method: 'POST',
        body: JSON.stringify({
          qr_token: token,
          payment_provider: paymentProvider,
          ...(!customAmountUZS && selected ? { tariff_block_id: selected } : {}),
          ...(customAmountUZS ? { amount_uzs: customAmountUZS } : {}),
          ...(voucherReadyForAutoApply ? { voucher_code: voucherCode.trim() } : {}),
          ...(playerAuth?.status === 'verified' && playerAuth.auth_token ? { player_auth_token: playerAuth.auth_token } : {}),
        }),
      });
      localStorage.setItem('clubpay:last_order_id', payload.order.invoice_id);
      if (paymentProvider === 'payme') {
        setCreatedCheckout(payload);
        return;
      }
      window.location.href = payload.checkout_url;
    } catch (err) {
      setError(String((err as Error).message || err));
    } finally {
      setCreating(false);
    }
  }

  async function copyCreatedOrderID() {
    if (!createdCheckout?.order.invoice_id) return;
    await navigator.clipboard.writeText(createdCheckout.order.invoice_id);
    setVoucherMessage('Order ID скопирован');
  }

  async function redeemVoucher() {
    if (!voucherCode.trim()) return;
    if (!voucherCheck?.can_redeem) {
      setVoucherError(voucherError || 'Ваучер не прошёл проверку. Проверьте код или дождитесь завершения проверки.');
      return;
    }
    setRedeeming(true);
    setVoucherMessage('');
    setVoucherError('');
    setError('');
    try {
      const payload = await api<{ grant_id: string; minutes_left: number; seconds_left?: number; extended?: boolean }>('/api/vouchers/redeem', {
        method: 'POST',
        body: JSON.stringify({ code: voucherCode.trim(), qr_token: token }),
      });
      const durationText = formatDurationClock(payload.seconds_left || payload.minutes_left * 60);
      setVoucherMessage(payload.extended ? `Ваучер применён: сессия продлена на ${durationText}` : `Ваучер применён: ${durationText}`);
      setVoucherCheck(null);
      setVoucherCode('');
      loadQR();
    } catch (err) {
      setVoucherError(String((err as Error).message || err));
    } finally {
      setRedeeming(false);
    }
  }

  if (loading) return <Centered text="Загружаем компьютер" />;
  if (error && !data) return <Centered text={error} />;
  if (!data) return <Centered text="Компьютер не найден" />;

  const isBusy = !canStartOrExtend;
  const deviceHint = isExtension
    ? 'QR с экрана активной сессии. Оплата или ваучер продлят текущее время.'
    : isStaticBusyQR
      ? `ПК занят${busyUntilLabel ? ` до ${busyUntilLabel}` : ''}. Оплата по наклейке недоступна. Для продления используйте QR на экране сессии.`
      : isBusy
        ? 'Этот компьютер сейчас нельзя оплатить по QR.'
        : `Пакет или своя сумма. 1 час: ${formatUZS(data.zone.hourly_price_uzs)}.`;
  const payLabel = isBusy
    ? (isStaticBusyQR ? 'ПК занят' : 'Компьютер недоступен')
    : isExtension && checkoutAmountUZS
      ? `Продлить на ${formatUZS(checkoutAmountUZS)}${voucherReadyForAutoApply ? ' + ваучер' : ''}`
    : checkoutAmountUZS
      ? `Оплатить ${formatUZS(checkoutAmountUZS)}${voucherReadyForAutoApply ? ' + ваучер' : ''}`
      : 'Нет доступных пакетов';

  return (
    <main className="qr-screen">
      <section className="qr-phone">
        <header className="qr-top">
          <div>
            <p className="qr-kicker">{data.club.name}</p>
            <h1>{data.pc.label}</h1>
          </div>
          <span className={`qr-status ${data.pc.status}`}>{pcStatusLabel(data.pc.status)}</span>
        </header>

        <section className="qr-device-card">
          <div>
            <span>Зона</span>
            <strong>{data.zone.name}</strong>
            <p>{deviceHint}</p>
          </div>
        </section>

        {needsPaymentEntryChoice ? (
          <section className="qr-entry-card" aria-live="polite">
            {playerAuth?.status === 'expired' ? (
              <>
                <div className="qr-entry-copy">
                  <p>Вход не завершён</p>
                  <h2>Ссылка истекла</h2>
                  <span>Откройте новую ссылку и подтвердите номер в Telegram.</span>
                </div>
                <Button full disabled={startingPlayerAuth} icon={<Send size={16} />} onClick={startPlayerAuth}>Получить новую ссылку</Button>
              </>
            ) : playerAuth?.telegram_link ? (
              <>
                <div className="qr-entry-copy">
                  <p>Профиль Clubpay</p>
                  <h2>{miniApp ? 'Подтвердите номер один раз' : 'Подтвердите вход в Telegram'}</h2>
                  <span>{miniApp ? 'Это нужно только для создания профиля и сохранения времени. Затем снова откроется PC.' : 'Откройте бота, поделитесь номером и вернитесь сюда. Пароль не нужен.'}</span>
                </div>
                <a className="btn primary qr-entry-primary" href={playerAuth.telegram_link}><Send size={16} /><span>{miniApp ? 'Подтвердить номер в Telegram' : 'Открыть Telegram'}</span></a>
              </>
            ) : (
              <>
                <div className="qr-entry-copy">
                  <p>Ваш Clubpay</p>
                  <h2>Войдите и сохраняйте время</h2>
                  <span>Один раз подтвердите Telegram — остаток после игры будет храниться в вашем профиле.</span>
                </div>
                <Button full disabled={startingPlayerAuth} icon={startingPlayerAuth ? <RefreshCw className="spin" size={16} /> : <Send size={16} />} onClick={startPlayerAuth}>
                  {startingPlayerAuth ? 'Открываем Telegram' : 'Открыть мой Clubpay'}
                </Button>
              </>
            )}
            <button type="button" className="qr-guest-entry" onClick={() => setPaymentEntry('guest')}>
              <strong>Продолжить без профиля</strong>
              <span>Остаток времени после сеанса не сохранится</span>
            </button>
          </section>
        ) : <>
          {paymentEntry === 'guest' && canShowPlayerProfile && (
            <section className="qr-guest-mode">
              <div>
                <p>Гостевой режим</p>
                <span>Время этого сеанса не сохранится в профиле.</span>
              </div>
              <Button variant="ghost" size="sm" disabled={startingPlayerAuth} icon={startingPlayerAuth ? <RefreshCw className="spin" size={14} /> : <Send size={14} />} onClick={switchGuestToPlayerAuth}>
                Открыть мой Clubpay
              </Button>
            </section>
          )}

          {canShowPlayerProfile && hasVerifiedPlayer && playerAuth?.player && <section className="qr-player-card">
            <div className="qr-player-copy">
              <p>Вы вошли в Clubpay</p>
              <h2>{playerAuth.player.first_name ? `${playerAuth.player.first_name}, ваш баланс` : 'Ваш баланс времени'}</h2>
              <strong>{formatPlayerBalanceDuration(playerBalanceSeconds)}</strong>
              <span>Время сохраняется в этом клубе после завершения сеанса.</span>
            </div>
            {playerBalanceSeconds > 0 ? (
              <Button variant="secondary" disabled={isBusy || redeemingBalance} icon={redeemingBalance ? <RefreshCw className="spin" size={16} /> : <Play size={16} />} onClick={redeemPlayerBalance}>
                {redeemingBalance ? 'Запускаем сеанс' : `Начать на ${formatPlayerBalanceDuration(playerBalanceSeconds)}`}
              </Button>
            ) : <p className="qr-player-empty">На балансе пока нет времени. Выберите пакет ниже: после оплаты сеанс начнётся сразу, а остаток сохранится в профиле.</p>}
          </section>}

          {paymentEntry === 'guest' && <section className="qr-voucher-card">
            <div>
              <p>Старый ваучер</p>
              <h2>Применить код</h2>
            </div>
            <div className="qr-voucher-form">
              <input
                id="voucher"
                value={voucherCode}
                onChange={(event) => setVoucherCode(event.target.value)}
                placeholder="Код ваучера"
                autoComplete="off"
                inputMode="text"
              />
              <Button variant="secondary" disabled={isBusy || checkingVoucher || redeeming || !voucherCode.trim()} icon={checkingVoucher ? <RefreshCw className="spin" size={16} /> : <Ticket size={16} />} onClick={redeemVoucher}>
                {redeeming ? 'Применяем' : 'Применить'}
              </Button>
            </div>
            {voucherCheck?.can_redeem && (
              <div className="qr-voucher-status success">
                <strong>Ваучер валиден: {formatDurationClock(voucherDurationSeconds)}</strong>
                <span>Можно применить сразу или добавить время к выбранной оплате.</span>
              </div>
            )}
            {voucherCheck && !voucherCheck.can_redeem && (
              <div className="qr-voucher-status danger">
                <strong>Ваучер нельзя применить</strong>
                <span>Проверьте клуб, зону или статус компьютера.</span>
              </div>
            )}
            {checkingVoucher && <p className="qr-method-message">Проверяем ваучер...</p>}
          </section>}

          {paymentEntry === 'guest' && data.telegram?.bot_link && (
            <section className="qr-telegram-card">
            <div className="qr-telegram-copy">
              <p>Telegram-бот</p>
              <h2>Получайте ваучеры</h2>
              <span>Если менеджер завершит сессию, остаток времени придёт сюда.</span>
              <LinkButton href={data.telegram.bot_link} variant="secondary" icon={<Send size={16} />}>Открыть бота</LinkButton>
            </div>
            </section>
          )}

          <section className="qr-section-heading">
          <div>
            <p>Пакеты</p>
            <h2>Сколько времени нужно?</h2>
          </div>
          <span>{formatPackageCount(data.tariffs.length)}</span>
          </section>

          <div className="qr-tariff-stack">
          {data.tariffs.map((tariff) => (
            <button
              type="button"
              className={`qr-tariff-card ${selected === tariff.id && !customAmountUZS ? 'selected' : ''}`}
              disabled={isBusy}
              key={tariff.id}
              onClick={() => {
                setSelected(tariff.id);
                setCustomAmount('');
              }}
              aria-pressed={selected === tariff.id && !customAmountUZS}
            >
              <span className="qr-card-label">{tariff.name}</span>
              <strong>{formatUZS(tariff.price_uzs)}</strong>
              <span className="qr-card-name">{data.zone.name}</span>
            </button>
          ))}
          </div>

          <label className="qr-custom-amount">
          <span>Своя сумма</span>
          <input
            value={customAmount}
            onChange={(event) => {
              setCustomAmount(formatUZSInput(event.target.value));
              setSelected('');
            }}
            placeholder={`1 час: ${formatUZS(data.zone.hourly_price_uzs)}`}
            inputMode="numeric"
            autoComplete="off"
            disabled={isBusy}
          />
          </label>

          <section className="qr-payment-methods">
          <div>
            <p>Способ оплаты</p>
            <span>Выберите, куда открыть платеж</span>
          </div>
          <div className="qr-method-grid">
            {providerOptions.map((provider) => (
              <button
                type="button"
                key={provider.provider}
                className={paymentProvider === provider.provider ? 'selected' : ''}
                disabled={isBusy || !provider.configured}
                onClick={() => setPaymentProvider(provider.provider)}
              >
                <strong>{provider.label}</strong>
                <span>{provider.configured ? (provider.sandbox ? 'Тестовый режим' : 'Готов') : 'Не настроен'}</span>
              </button>
            ))}
          </div>
          {selectedProviderOption && !selectedProviderOption.configured && <p className="qr-method-message">{selectedProviderOption.message}</p>}
          </section>

          {voucherMessage && <Notice tone="success">{voucherMessage}</Notice>}
          {createdCheckout && (
            <section className="qr-order-card">
            <div>
              <p>Payme заказ создан</p>
              <h2>Order ID</h2>
              <code>{createdCheckout.order.invoice_id}</code>
              <span>Этот ID нужен для проверки оплаты в Payme sandbox.</span>
            </div>
            <div className="qr-order-actions">
              <Button size="sm" variant="ghost" icon={<Copy size={13} />} onClick={copyCreatedOrderID}>Скопировать</Button>
              <LinkButton href={createdCheckout.checkout_url} variant="secondary" icon={<CreditCard size={16} />}>Открыть Payme</LinkButton>
            </div>
            </section>
          )}
          {voucherError && <Notice tone="danger">{voucherError}</Notice>}
          {error && <Notice tone="danger">{error}</Notice>}

          <div className="qr-checkout-bar">
            <div className="qr-checkout-inner">
              <Button full className="qr-pay-button" disabled={isBusy || checkingVoucher || (!selected && !customAmountUZS) || !checkoutAmountUZS || creating || !paymentMethodReady} icon={creating ? <RefreshCw className="spin" size={18} /> : <CreditCard size={18} />} onClick={createCheckout}>
                {creating ? 'Открываем оплату' : payLabel}
              </Button>
            </div>
          </div>
        </>}
      </section>
    </main>
  );
}

function AdminPage({ auth, selectedClubID, currentPath, onClubChange, onLogout }: WorkspaceProps) {
  const [catalog, setCatalog] = useState<Catalog>({ club_id: selectedClubID, pcs: [], zones: [], tariffs: [] });
  const [catalogFetchedAtMs, setCatalogFetchedAtMs] = useState(Date.now());
  const [nowMs, setNowMs] = useState(Date.now());
  const [orders, setOrders] = useState<Order[]>([]);
  const [grants, setGrants] = useState<Grant[]>([]);
  const [cashPCID, setCashPCID] = useState('');
  const [cashTariffID, setCashTariffID] = useState('');
  const [cashAmount, setCashAmount] = useState('');
  const [cashReason, setCashReason] = useState('cash_payment');
  const [endPhoneByGrant, setEndPhoneByGrant] = useState<Record<string, string>>({});
  const [endSessionDraft, setEndSessionDraft] = useState<{ grant: Grant; phone: string; recipientConsent: boolean; confirmWithoutPhone: boolean } | null>(null);
  const [telegramPrompt, setTelegramPrompt] = useState<TelegramPrompt | null>(null);
  const [message, setMessage] = useState('');
  const [pcQuery, setPCQuery] = useState('');
  const [pcStatusFilter, setPCStatusFilter] = useState('');
  const [pcZoneFilter, setPCZoneFilter] = useState('');
  const [selectedPCID, setSelectedPCID] = useState('');
  const refreshingRef = useRef(false);

  async function refresh() {
    if (!selectedClubID || refreshingRef.current) return;
    refreshingRef.current = true;
    try {
      const [catalogPayload, orderPayload, grantPayload] = await Promise.all([
        api<Catalog>(clubPath('/api/admin/catalog', selectedClubID)),
        api<{ orders: Order[] }>(clubPath('/api/admin/orders', selectedClubID)),
        api<{ grants: Grant[] }>(clubPath('/api/admin/grants', selectedClubID)),
      ]);
      const fetchedAt = Date.now();
      setCatalog({ ...catalogPayload, pcs: catalogPayload.pcs || [], zones: catalogPayload.zones || [], tariffs: catalogPayload.tariffs || [] });
      setCatalogFetchedAtMs(fetchedAt);
      setNowMs(fetchedAt);
      setOrders(orderPayload.orders || []);
      setGrants(grantPayload.grants || []);
      setCashPCID((current) => (catalogPayload.pcs || []).some((pc) => pc.id === current) ? current : catalogPayload.pcs?.[0]?.id || '');
    } finally {
      refreshingRef.current = false;
    }
  }

  useEffect(() => {
    refresh();
    const refreshTimer = window.setInterval(refresh, ADMIN_REFRESH_MS);
    return () => {
      window.clearInterval(refreshTimer);
    };
  }, [selectedClubID]);

  useEffect(() => {
    const hasElapsedOccupiedPC = catalog.pcs.some(
      (pc) => pc.status === 'occupied' && remainingSecondsForPC(pc, nowMs, catalogFetchedAtMs) <= 0,
    );
    if (!hasElapsedOccupiedPC) return;
    const timer = window.setTimeout(refresh, 250);
    return () => window.clearTimeout(timer);
  }, [catalog.pcs, catalogFetchedAtMs, nowMs]);

  const selectedCashPC = catalog.pcs.find((pc) => pc.id === cashPCID);
  const selectedCashZone = catalog.zones.find((zone) => zone.id === selectedCashPC?.zone_id);
  const cashTariffs = useMemo(
    () => catalog.tariffs.filter((tariff) => tariff.zone_id === selectedCashPC?.zone_id),
    [catalog.tariffs, selectedCashPC?.zone_id],
  );
  const selectedCashTariff = cashTariffs.find((tariff) => tariff.id === cashTariffID);
  const cashAmountUZS = parseUZSInput(cashAmount);
  const cashButtonAmount = cashAmountUZS || selectedCashTariff?.price_uzs || 0;

  useEffect(() => {
    if (cashAmountUZS) {
      if (cashTariffID) setCashTariffID('');
      return;
    }
    if (!cashTariffID && cashTariffs.length > 0) {
      setCashTariffID(cashTariffs[0]?.id || '');
      return;
    }
    if (cashTariffID && !cashTariffs.some((tariff) => tariff.id === cashTariffID)) {
      setCashTariffID(cashTariffs[0]?.id || '');
    }
  }, [cashAmountUZS, cashPCID, cashTariffID, cashTariffs]);

  async function startCashSession() {
    if (!cashPCID || (!cashTariffID && !cashAmountUZS)) return;
    await api('/api/admin/cash-sessions', {
      method: 'POST',
      body: JSON.stringify({
        pc_id: cashPCID,
        ...(!cashAmountUZS && cashTariffID ? { tariff_block_id: cashTariffID } : {}),
        reason: cashReason,
        ...(cashAmountUZS ? { amount_uzs: cashAmountUZS } : {}),
      }),
    });
    setCashAmount('');
    setMessage('Наличная сессия запущена');
    refresh();
  }

  async function setPCStatus(pcID: string, status: string) {
    await api(`/api/admin/pcs/${pcID}/status`, {
      method: 'POST',
      body: JSON.stringify({ status, reason: 'admin_panel' }),
    });
    refresh();
  }

  async function endGrant(grant: Grant, phone = '', recipientConsent = false) {
    setTelegramPrompt(null);
    const result = await api<{ voucher?: { code: string; minutes_left: number; seconds_left?: number }; voucher_delivery?: VoucherDelivery }>(`/api/admin/grants/${grant.id}/end`, {
      method: 'POST',
      body: JSON.stringify({ reason: 'admin_request', recipient_phone: phone, recipient_consent: recipientConsent }),
    });
    if (result.voucher_delivery?.telegram_link) {
      setTelegramPrompt({
        link: result.voucher_delivery.telegram_link,
        phone: result.voucher_delivery.phone,
        code: result.voucher?.code,
        minutes: result.voucher?.minutes_left,
        seconds: result.voucher?.seconds_left,
        status: result.voucher_delivery.status,
      });
    }
    const deliverySuffix = telegramDeliverySuffix(result.voucher_delivery?.status);
    setMessage(result.voucher ? `Сессия завершена. Ваучер: ${result.voucher.code}${deliverySuffix}` : 'Сессия завершена без ваучера');
    setEndSessionDraft(null);
    setEndPhoneByGrant((current) => {
      const next = { ...current };
      delete next[grant.id];
      return next;
    });
    window.scrollTo({ top: 0, behavior: 'smooth' });
    refresh();
  }

  function openEndSession(grant: Grant) {
    setEndSessionDraft({ grant, phone: endPhoneByGrant[grant.id] || '', recipientConsent: false, confirmWithoutPhone: false });
  }

  async function submitEndSessionDraft() {
    if (!endSessionDraft) return;
    const phone = endSessionDraft.phone.trim();
    if (!phone && !endSessionDraft.confirmWithoutPhone) {
      setEndSessionDraft({ ...endSessionDraft, confirmWithoutPhone: true });
      return;
    }
    if (phone && !endSessionDraft.recipientConsent) return;
    await endGrant(endSessionDraft.grant, phone, endSessionDraft.recipientConsent);
  }

  async function syncOrder(order: Order) {
    await api(`/api/payments/sync/${order.invoice_id}`, { method: 'POST' });
    refresh();
  }

  async function testPayOrder(order: Order) {
    await api(`/api/payments/mock/success/${order.invoice_id}`, { method: 'POST' });
    refresh();
  }

  const pcStats = useMemo(() => {
    const total = catalog.pcs.length;
    const available = catalog.pcs.filter((pc) => pc.status === 'available').length;
    const occupied = catalog.pcs.filter((pc) => pc.status === 'occupied').length;
    const maintenance = catalog.pcs.filter((pc) => pc.status === 'maintenance').length;
    return { total, available, occupied, maintenance };
  }, [catalog.pcs]);

  const filteredPCs = useMemo(() => {
    const query = pcQuery.trim().toLowerCase();
    return catalog.pcs.filter((pc) => {
      const matchesQuery = !query || `${pc.label} ${pc.external_pc_id} ${pc.zone}`.toLowerCase().includes(query);
      const matchesStatus = !pcStatusFilter || pc.status === pcStatusFilter;
      const matchesZone = !pcZoneFilter || pc.zone_id === pcZoneFilter;
      return matchesQuery && matchesStatus && matchesZone;
    });
  }, [catalog.pcs, pcQuery, pcStatusFilter, pcZoneFilter]);

  const adminSection = currentPath.startsWith('/admin/sessions')
    ? 'sessions'
    : currentPath.startsWith('/admin/payments')
      ? 'payments'
      : 'hall';
  const pageTitle = adminSection === 'sessions' ? 'Игровые сессии' : adminSection === 'payments' ? 'Оплаты клуба' : 'Состояние зала';

  const cashWorkspace = (
    <section className="work-surface cash-workspace">
      <div className="surface-heading">
        <div>
          <h2>Наличная сессия</h2>
          <p>Ручной запуск времени на выбранном компьютере.</p>
        </div>
        <Banknote size={18} />
      </div>
      <div className="form-grid one-col compact-form">
        <label>
          Компьютер
          <select value={cashPCID} onChange={(event) => setCashPCID(event.target.value)}>
            {catalog.pcs.map((pc) => <option key={pc.id} value={pc.id}>{pc.label} · {pc.zone}</option>)}
          </select>
        </label>
        <label>
          Пакет
          <select
            value={cashTariffID}
            onChange={(event) => {
              setCashTariffID(event.target.value);
              setCashAmount('');
            }}
          >
            <option value="">Без пакета</option>
            {cashTariffs.map((tariff) => <option key={tariff.id} value={tariff.id}>{tariff.name} · {formatUZS(tariff.price_uzs)}</option>)}
          </select>
        </label>
        <label>
          Своя сумма, сум
          <input
            value={cashAmount}
            onChange={(event) => {
              setCashAmount(formatUZSInput(event.target.value));
              setCashTariffID('');
            }}
            placeholder={selectedCashZone ? `1 час: ${formatUZS(selectedCashZone.hourly_price_uzs)}` : 'Например: 20 000'}
            inputMode="numeric"
          />
        </label>
        <label>
          Причина наличной операции
          <select value={cashReason} onChange={(event) => setCashReason(event.target.value)}>
            <option value="cash_payment">Оплата наличными</option>
            <option value="provider_unavailable">Платёжный провайдер недоступен</option>
            <option value="internet_unavailable">Нет интернета</option>
            <option value="terminal_fallback">Запасной сценарий менеджера</option>
          </select>
        </label>
      </div>
      <Button full icon={<Play size={16} />} onClick={startCashSession} disabled={!cashPCID || (!cashTariffID && !cashAmountUZS)}>
        {cashButtonAmount ? `Запустить на ${formatUZS(cashButtonAmount)}` : 'Запустить сессию'}
      </Button>
    </section>
  );

  const sessionsWorkspace = (
    <section className="work-surface sessions-workspace">
      <div className="surface-heading">
        <div>
          <h2>Сессии</h2>
          <p>{grants.length ? `${grants.length} записей в текущем клубе` : 'Нет активных или завершённых сессий'}</p>
        </div>
        <Gamepad2 size={18} />
      </div>
      <div className="operation-list">
        {grants.length === 0 && <EmptyState text="Сессий пока нет" />}
        {grants.map((grant) => (
          <div className="operation-row" key={grant.id}>
            <div className="operation-main">
              <strong>{grant.pc_label}</strong>
              <span>{sourceLabel(grant.source)} · {grantStatusLabel(grant.status)}</span>
              {grant.planned_ends_at && <small>До {formatDateTime(grant.planned_ends_at)}</small>}
              {grant.last_error && <small className="danger-text">{grant.last_error}</small>}
            </div>
            <code>{formatDurationClock(grant.duration_seconds || grant.duration_minutes * 60)}</code>
            {grant.status === 'accepted' && (
              <div className="end-session">
                <input
                  aria-label="Телефон для Telegram"
                  inputMode="tel"
                  value={endPhoneByGrant[grant.id] || ''}
                  onFocus={() => setEndPhoneByGrant((current) => ({ ...current, [grant.id]: current[grant.id] || '+998 ' }))}
                  onChange={(event) => setEndPhoneByGrant((current) => ({ ...current, [grant.id]: formatUzPhoneInput(event.target.value) }))}
                  placeholder="+998 00 000 00 00"
                />
                <Button size="sm" variant="secondary" icon={<Power size={14} />} onClick={() => openEndSession(grant)}>Завершить</Button>
              </div>
            )}
          </div>
        ))}
      </div>
    </section>
  );

  const paymentsWorkspace = (
    <section className="work-surface payments-workspace">
      <div className="surface-heading">
        <div>
          <h2>Последние оплаты</h2>
          <p>Click, Payme, тестовые операции и статус фискализации.</p>
        </div>
        <ReceiptText size={18} />
      </div>
      <div className="operation-list payment-list">
        {orders.length === 0 && <EmptyState text="Оплат пока нет" />}
        {orders.map((order) => (
          <div className="operation-row payment-row" key={order.id}>
            <div className="operation-main">
              <strong>{order.pc_label} · {order.tariff}</strong>
              <span>{providerLabel(order.provider)} · {orderStatusLabel(order.status)} · {fiscalStatusLabel(order.fiscal_status)}</span>
              <small>{order.invoice_id}</small>
            </div>
            <code>{formatUZS(order.amount_uzs)}</code>
            {order.status !== 'paid' && (
              <div className="row-actions">
                <Button className="icon-only" size="sm" variant="ghost" icon={<RefreshCw size={14} />} onClick={() => syncOrder(order)} aria-label="Обновить оплату" title="Обновить оплату" />
                <Button className="icon-only" size="sm" variant="success" icon={<Play size={14} />} onClick={() => testPayOrder(order)} aria-label="Тестовая оплата" title="Тестовая оплата" />
              </div>
            )}
          </div>
        ))}
      </div>
    </section>
  );

  const pcWorkspace = (
    <section className="work-surface pc-workspace">
      <div className="surface-heading pc-heading">
        <div>
          <h2>Компьютеры</h2>
          <p>Статус зала обновляется автоматически.</p>
        </div>
        <span className="surface-count">{filteredPCs.length} из {catalog.pcs.length}</span>
      </div>
      <div className="pc-filterbar">
        <select value={pcZoneFilter} onChange={(event) => setPCZoneFilter(event.target.value)} aria-label="Фильтр по зоне">
          <option value="">Все зоны</option>
          {catalog.zones.map((zone) => <option value={zone.id} key={zone.id}>{zone.name}</option>)}
        </select>
        <label className="search-control">
          <Search size={15} />
          <input value={pcQuery} onChange={(event) => setPCQuery(event.target.value)} placeholder="Найти компьютер" aria-label="Найти компьютер" />
        </label>
        <select value={pcStatusFilter} onChange={(event) => setPCStatusFilter(event.target.value)} aria-label="Фильтр по статусу">
          <option value="">Все статусы</option>
          <option value="available">Свободные</option>
          <option value="occupied">Занятые</option>
          <option value="sleeping">Сон</option>
          <option value="maintenance">Ремонт</option>
        </select>
      </div>
      <div className="table-wrap desktop-pc-table">
        <table className="pc-table">
          <thead>
            <tr>
              <th>Компьютер</th>
              <th>Зона</th>
              <th>Статус</th>
              <th>Осталось</th>
              <th className="actions-col">Действия</th>
            </tr>
          </thead>
          <tbody>
            {filteredPCs.map((pc) => {
              const remainingSeconds = remainingSecondsForPC(pc, nowMs, catalogFetchedAtMs);
              const activeGrant = grants.find((grant) => grant.id === pc.active_grant_id);
              return (
                <tr className={selectedPCID === pc.id ? 'selected' : ''} key={pc.id} onClick={() => setSelectedPCID(pc.id)}>
                  <td>
                    <div className="pc-cell">
                      <Monitor size={17} />
                      <div><strong>{pc.label}</strong><span>{pc.external_pc_id}</span></div>
                    </div>
                  </td>
                  <td>{pc.zone}</td>
                  <td><StatusBadge status={pc.status} /></td>
                  <td><span className={`remaining ${pc.status === 'occupied' ? 'active' : ''}`}>{pc.status === 'occupied' ? formatRemainingTime(remainingSeconds) : '-'}</span></td>
                  <td>
                    <div className="table-actions">
                      {pc.status === 'occupied' && activeGrant ? (
                        <Button size="sm" variant="secondary" icon={<Power size={14} />} onClick={() => openEndSession(activeGrant)}>Завершить</Button>
                      ) : (
                        <Button size="sm" variant={pc.status === 'maintenance' ? 'secondary' : 'danger'} icon={<Wrench size={14} />} onClick={() => setPCStatus(pc.id, pc.status === 'maintenance' ? 'available' : 'maintenance')}>
                          {pc.status === 'maintenance' ? 'Вернуть' : 'Ремонт'}
                        </Button>
                      )}
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      <div className="mobile-pc-list">
        {filteredPCs.map((pc) => {
          const remainingSeconds = remainingSecondsForPC(pc, nowMs, catalogFetchedAtMs);
          const activeGrant = grants.find((grant) => grant.id === pc.active_grant_id);
          return (
            <article className={`mobile-pc-row ${selectedPCID === pc.id ? 'selected' : ''}`} key={pc.id}>
              <button type="button" className="mobile-pc-main" onClick={() => setSelectedPCID(selectedPCID === pc.id ? '' : pc.id)}>
                <span><strong>{pc.label}</strong><small>{pc.zone} · {pc.external_pc_id}</small></span>
                <span className="mobile-pc-state"><StatusBadge status={pc.status} /><code>{pc.status === 'occupied' ? formatRemainingTime(remainingSeconds) : '-'}</code></span>
              </button>
              {selectedPCID === pc.id && (
                <div className="mobile-pc-actions">
                  {pc.status === 'occupied' && activeGrant ? (
                    <Button size="sm" variant="secondary" icon={<Power size={14} />} onClick={() => openEndSession(activeGrant)}>Завершить сессию</Button>
                  ) : (
                    <Button size="sm" variant={pc.status === 'maintenance' ? 'secondary' : 'danger'} icon={<Wrench size={14} />} onClick={() => setPCStatus(pc.id, pc.status === 'maintenance' ? 'available' : 'maintenance')}>
                      {pc.status === 'maintenance' ? 'Вернуть в зал' : 'Ремонт'}
                    </Button>
                  )}
                </div>
              )}
            </article>
          );
        })}
      </div>
      {filteredPCs.length === 0 && <EmptyState text="По фильтрам компьютеры не найдены" />}
    </section>
  );

  return (
    <main className="shell workspace-shell">
      <WorkspaceHeader auth={auth} selectedClubID={selectedClubID} currentPath={currentPath} onClubChange={onClubChange} onLogout={onLogout} eyebrow="Операции клуба" title={pageTitle} />

      {message && <Notice tone="success">{message}</Notice>}
      {telegramPrompt && <TelegramVoucherModal prompt={telegramPrompt} onClose={() => setTelegramPrompt(null)} onCopied={() => setMessage('Ссылка Telegram скопирована')} />}
      {endSessionDraft && (
        <EndSessionModal
          draft={endSessionDraft}
          onPhoneChange={(phone) => {
            setEndSessionDraft({ ...endSessionDraft, phone, recipientConsent: false, confirmWithoutPhone: false });
            setEndPhoneByGrant((current) => ({ ...current, [endSessionDraft.grant.id]: phone }));
          }}
          onRecipientConsentChange={(recipientConsent) => setEndSessionDraft({ ...endSessionDraft, recipientConsent })}
          onClose={() => setEndSessionDraft(null)}
          onSubmit={submitEndSessionDraft}
        />
      )}
      {adminSection === 'hall' && (
        <>
          <section className="summary-strip" aria-label="Сводка по залу">
            <div className="summary-item primary"><span>Всего ПК</span><strong>{pcStats.total}</strong><small>в текущем клубе</small></div>
            <div className="summary-item"><span>Свободны</span><strong>{pcStats.available}</strong><small>готовы к запуску</small></div>
            <div className="summary-item"><span>Заняты</span><strong>{pcStats.occupied}</strong><small>активные места</small></div>
            <div className={`summary-item ${pcStats.maintenance ? 'warning' : ''}`}><span>Ремонт</span><strong>{pcStats.maintenance}</strong><small>недоступны клиенту</small></div>
          </section>
          <div className="operations-layout">
            {pcWorkspace}
            <aside className="operator-column">{cashWorkspace}{sessionsWorkspace}</aside>
          </div>
          <div className="hall-payments-preview">{paymentsWorkspace}</div>
        </>
      )}
      {adminSection === 'sessions' && <div className="focus-layout">{sessionsWorkspace}{cashWorkspace}</div>}
      {adminSection === 'payments' && paymentsWorkspace}
    </main>
  );
}

function ReportsPage({ auth, selectedClubID, currentPath, onClubChange, onLogout }: WorkspaceProps) {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [catalog, setCatalog] = useState<Catalog | null>(null);
  const [orders, setOrders] = useState<Order[]>([]);
  const [clubSettings, setClubSettings] = useState<ClubSettings | null>(null);
  const refreshingRef = useRef(false);

  useEffect(() => {
    async function refreshSummary() {
      if (!selectedClubID || refreshingRef.current) return;
      refreshingRef.current = true;
      try {
        const [summaryPayload, catalogPayload, orderPayload, settingsPayload] = await Promise.all([
          api<Summary>(clubPath('/api/owner/summary', selectedClubID)),
          api<Catalog>(clubPath('/api/admin/catalog', selectedClubID)),
          api<{ orders: Order[] }>(clubPath('/api/admin/orders', selectedClubID)),
          api<ClubSettingsPayload>(`/api/backoffice/clubs/${selectedClubID}/settings`),
        ]);
        setSummary(summaryPayload);
        setCatalog({ ...catalogPayload, pcs: catalogPayload.pcs || [], zones: catalogPayload.zones || [], tariffs: catalogPayload.tariffs || [] });
        setOrders(orderPayload.orders || []);
        setClubSettings(settingsPayload.club);
      } finally {
        refreshingRef.current = false;
      }
    }

    refreshSummary();
    const timer = window.setInterval(refreshSummary, OWNER_REFRESH_MS);
    return () => window.clearInterval(timer);
  }, [selectedClubID]);

  const isSuperAdmin = auth.user.global_role === 'super_admin';
  const occupiedPCs = catalog?.pcs.filter((pc) => pc.status === 'occupied').length || 0;
  const maintenancePCs = catalog?.pcs.filter((pc) => pc.status === 'maintenance').length || 0;
  const pendingOrders = orders.filter((order) => order.status !== 'paid');
  const paymentReady = clubSettings?.payment_connected ?? Boolean(clubSettings?.click_connected || clubSettings?.payme_connected);
  const activeClubs = auth.clubs.filter((club) => club.status === 'active').length;

  return (
    <main className="shell workspace-shell">
      <WorkspaceHeader
        auth={auth}
        selectedClubID={selectedClubID}
        currentPath={currentPath}
        onClubChange={onClubChange}
        onLogout={onLogout}
        eyebrow={isSuperAdmin ? 'Контур сети' : 'Контроль клуба'}
        title={isSuperAdmin ? 'Контроль сети' : 'Обзор бизнеса'}
      />
      {!summary ? (
        <section className="work-surface"><EmptyState text="Считаем сводку" /></section>
      ) : isSuperAdmin ? (
        <>
          <section className="summary-strip owner-summary" aria-label="Сводка по сети">
            <div className="summary-item primary"><span>Клубы в сети</span><strong>{auth.clubs.length}</strong><small>доступны суперадмину</small></div>
            <div className="summary-item"><span>Активные клубы</span><strong>{activeClubs}</strong><small>{auth.clubs.length - activeClubs} отключены</small></div>
            <div className="summary-item"><span>Выбранный клуб</span><strong>{formatUZS(summary.club_total_revenue_uzs)}</strong><small>выручка клуба</small></div>
            <div className={`summary-item ${paymentReady ? '' : 'warning'}`}><span>Подключение</span><strong>{paymentReady ? 'Готово' : 'Проверить'}</strong><small>Click / Payme</small></div>
          </section>
          <div className="owner-overview-grid network-overview-grid">
            <section className="work-surface network-workspace">
              <div className="surface-heading">
                <div><h2>Клубы сети</h2><p>Реестр доступных клубов и их текущий статус.</p></div>
                <Building2 size={18} />
              </div>
              <div className="network-list">
                {auth.clubs.map((club) => (
                  <button className={`network-row ${club.id === selectedClubID ? 'selected' : ''}`} key={club.id} onClick={() => onClubChange(club.id)}>
                    <span><strong>{club.name}</strong><small>{club.slug || club.id}</small></span>
                    <span className={club.status === 'active' ? 'signal-ready' : 'signal-warning'}>{club.status === 'active' ? 'Активен' : 'Отключен'}</span>
                  </button>
                ))}
              </div>
            </section>
            <section className="work-surface pulse-workspace">
              <div className="surface-heading">
                <div><h2>Контур клуба</h2><p>Сигналы выбранного операционного контура.</p></div>
                <Activity size={18} />
              </div>
              <div className="signal-list">
                <AppLink className="signal-row" href="/admin">
                  <span><Monitor size={17} /><strong>Зал</strong></span>
                  <span>{occupiedPCs} заняты · {maintenancePCs} ремонт</span>
                </AppLink>
                <AppLink className="signal-row" href="/admin/payments">
                  <span><ReceiptText size={17} /><strong>Оплаты</strong></span>
                  <span>{pendingOrders.length} требуют проверки</span>
                </AppLink>
                <AppLink className="signal-row" href="/settings/connections">
                  <span><CreditCard size={17} /><strong>Подключение</strong></span>
                  <span className={paymentReady ? 'signal-ready' : 'signal-warning'}>{paymentReady ? 'Работает' : 'Требует настройки'}</span>
                </AppLink>
              </div>
            </section>
          </div>
        </>
      ) : (
        <>
          <section className="summary-strip owner-summary" aria-label="Финансовая сводка">
            <div className="summary-item primary"><span>Выручка клуба</span><strong>{formatUZS(summary.club_total_revenue_uzs)}</strong><small>онлайн и наличные</small></div>
            <div className="summary-item"><span>Онлайн на кассу</span><strong>{formatUZS(summary.club_online_revenue_uzs || summary.online_revenue_uzs)}</strong><small>{summary.paid_orders} оплат</small></div>
            <div className="summary-item"><span>Наличные</span><strong>{formatUZS(summary.cash_revenue_uzs)}</strong><small>{summary.cash_sessions} сессий</small></div>
            <div className="summary-item"><span>Активные сессии</span><strong>{summary.active_grants}</strong><small>сейчас в зале</small></div>
          </section>
          <div className="owner-overview-grid">
            <section className="work-surface pulse-workspace">
              <div className="surface-heading">
                <div><h2>Состояние клуба</h2><p>Операционные сигналы по выбранному клубу.</p></div>
                <Activity size={18} />
              </div>
              <div className="signal-list">
                <AppLink className="signal-row" href="/admin">
                  <span><Monitor size={17} /><strong>Зал</strong></span>
                  <span>{occupiedPCs} заняты · {maintenancePCs} ремонт</span>
                </AppLink>
                <AppLink className="signal-row" href="/admin/payments">
                  <span><ReceiptText size={17} /><strong>Оплаты</strong></span>
                  <span>{pendingOrders.length} требуют проверки</span>
                </AppLink>
                <AppLink className="signal-row" href={isSuperAdmin ? '/settings/connections' : '/settings'}>
                  <span><CreditCard size={17} /><strong>Подключение</strong></span>
                  <span className={paymentReady ? 'signal-ready' : 'signal-warning'}>{paymentReady ? 'Работает' : 'Требует настройки'}</span>
                </AppLink>
              </div>
            </section>
            <section className="work-surface attention-workspace">
              <div className="surface-heading">
                <div><h2>Требует внимания</h2><p>Незавершённые операции без искусственного скоринга.</p></div>
                <AlertCircle size={18} />
              </div>
              <div className="operation-list compact-operations">
                {pendingOrders.length === 0 && <EmptyState text="Проблемных оплат нет" />}
                {pendingOrders.slice(0, 5).map((order) => (
                  <AppLink className="operation-row" href="/admin/payments" key={order.id}>
                    <div className="operation-main"><strong>{order.pc_label} · {order.tariff}</strong><span>{providerLabel(order.provider)} · {orderStatusLabel(order.status)}</span></div>
                    <code>{formatUZS(order.amount_uzs)}</code>
                  </AppLink>
                ))}
              </div>
            </section>
          </div>
        </>
      )}
    </main>
  );
}

function defaultZoneForm(_zones: Zone[] = []): Partial<Zone> {
  return { name: '', hourly_price_uzs: 15000, sort_order: 0, status: 'active' };
}

function defaultTariffForm(zones: Zone[] = [], _tariffs: Tariff[] = []): Partial<Tariff> {
  const zoneID = zones[0]?.id || '';
  return { zone_id: zoneID, name: '', duration_minutes: 60, price_uzs: 0, sort_order: 0, status: 'active' };
}

function uniqueExternalPCID(label: string, pcs: ManagedPC[] = [], currentID?: string) {
  const base = slugifyClient(label || 'pc');
  const used = new Set(pcs.filter((pc) => pc.id !== currentID).map((pc) => pc.external_pc_id));
  if (!used.has(base)) return base;
  let index = 2;
  while (used.has(`${base}-${index}`)) index += 1;
  return `${base}-${index}`;
}

function defaultPCForm(zones: Zone[] = [], pcs: ManagedPC[] = []): Partial<ManagedPC> {
  const nextNumber = Math.max(0, ...pcs.map((pc) => pc.number || 0)) + 1;
  const label = `PC #${String(nextNumber).padStart(2, '0')}`;
  return {
    zone_id: zones[0]?.id || '',
    number: nextNumber,
    label,
    external_pc_id: uniqueExternalPCID(label, pcs),
    status: 'available',
		mac_address: '',
  };
}

function defaultUserForm(): Partial<ClubUser> & { password?: string } {
  return { name: '', email: '', phone: '', role: 'admin', status: 'active', password: '' };
}

function SettingsPage({ auth, selectedClubID, currentPath, onClubChange, onLogout, onReloadAuth }: WorkspaceProps & { onReloadAuth: () => void | Promise<void> }) {
  const canManageNetwork = auth.user.global_role === 'super_admin';
  const [settings, setSettings] = useState<ClubSettingsPayload | null>(null);
  const [networks, setNetworks] = useState<ClubNetwork[]>([]);
  const [networkForm, setNetworkForm] = useState<ClubNetwork>({ id: '', name: '', slug: '', status: 'active', role: 'super_admin' });
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [clubForm, setClubForm] = useState<ClubSettings | null>(null);
  const [creatingClub, setCreatingClub] = useState(false);
  const [zoneForm, setZoneForm] = useState<Partial<Zone>>(defaultZoneForm());
  const [tariffForm, setTariffForm] = useState<Partial<Tariff>>(defaultTariffForm());
  const [pcForm, setPCForm] = useState<Partial<ManagedPC>>(defaultPCForm());
  const [userForm, setUserForm] = useState<Partial<ClubUser> & { password?: string }>(defaultUserForm());
  const [showZoneForm, setShowZoneForm] = useState(false);
  const [showTariffForm, setShowTariffForm] = useState(false);
  const [showPCForm, setShowPCForm] = useState(false);
  // Keep a shareable fallback for the access form. This is useful when a
  // browser extension or an aggressive popup/click blocker interferes with
  // the inline "New user" button in the settings UI.
  const [showUserForm, setShowUserForm] = useState(
    () => new URLSearchParams(window.location.search).get('new-user') === '1',
  );
  const [showNetworkForm, setShowNetworkForm] = useState(false);
  const [tariffZoneFilter, setTariffZoneFilter] = useState('');
  const [pcZoneFilter, setPCZoneFilter] = useState('');

  async function loadSettings() {
    if (!selectedClubID) return;
    try {
      const payload = await api<ClubSettingsPayload>(`/api/backoffice/clubs/${selectedClubID}/settings`);
      setSettings(payload);
      setClubForm(payload.club);
      setCreatingClub(false);
      setZoneForm(defaultZoneForm(payload.zones));
      setTariffForm(defaultTariffForm(payload.zones, payload.tariffs));
      setPCForm(defaultPCForm(payload.zones, payload.pcs));
      setUserForm(defaultUserForm());
      setShowZoneForm(false);
      setShowTariffForm(false);
      setShowPCForm(false);
      // Do not immediately close the direct access form after the settings
      // payload arrives. The `?new-user=1` link is used as a safe fallback
      // when a browser extension prevents the normal button click.
      setShowUserForm(new URLSearchParams(window.location.search).get('new-user') === '1');
      setTariffZoneFilter((current) => current && payload.zones.some((zone) => zone.id === current) ? current : payload.zones[0]?.id || '');
      setPCZoneFilter((current) => current && payload.zones.some((zone) => zone.id === current) ? current : '');
      setError('');
    } catch (err) {
      setError(String((err as Error).message || err));
    }
  }

  async function loadNetworks() {
    if (!canManageNetwork) return [];
    try {
      const payload = await api<{ networks: ClubNetwork[] }>('/api/backoffice/networks');
      const nextNetworks = payload.networks || [];
      setNetworks(nextNetworks);
      return nextNetworks;
    } catch (err) {
      setNetworks([]);
      setError(`Не удалось загрузить сети клубов: ${String((err as Error).message || err)}`);
      return [];
    }
  }

  useEffect(() => {
    setSettings(null);
    loadSettings();
  }, [selectedClubID]);

  useEffect(() => {
    loadNetworks();
  }, [canManageNetwork]);

  async function persistSettings(action: () => Promise<unknown>, successMessage: string, reloadAuth = false) {
    try {
      setError('');
      setMessage('');
      await action();
      setMessage(successMessage);
      await loadSettings();
      if (reloadAuth) await onReloadAuth();
    } catch (err) {
      setError(String((err as Error).message || err));
    }
  }

  function resetZone() {
    setZoneForm(defaultZoneForm(settings?.zones || []));
    setShowZoneForm(true);
  }

  function resetTariff() {
    const next = defaultTariffForm(settings?.zones || [], settings?.tariffs || []);
    setTariffForm({ ...next, zone_id: tariffZoneFilter || next.zone_id });
    setShowTariffForm(true);
  }

  function resetPC() {
    setPCForm(defaultPCForm(settings?.zones || [], settings?.pcs || []));
    setShowPCForm(true);
  }

  function resetUser() {
    setUserForm(defaultUserForm());
    setShowUserForm(true);
  }

  function resetNetwork() {
    setNetworkForm({ id: '', name: '', slug: '', status: 'active', role: 'super_admin' });
    setShowNetworkForm(true);
  }

  function editNetwork(network: ClubNetwork) {
    setNetworkForm(network);
    setShowNetworkForm(true);
  }

  function startNewClub() {
    const network = networks.find((item) => item.status === 'active');
    if (!network) {
      setError('Сначала создайте активную сеть клубов');
      navigateTo('/settings/networks');
      return;
    }
    setCreatingClub(true);
    setSettings({ club: { ...EMPTY_CLUB_FORM }, zones: [], tariffs: [], pcs: [], users: [] });
    setClubForm({ ...EMPTY_CLUB_FORM, network_id: network.id, network_name: network.name });
    setZoneForm(defaultZoneForm());
    setTariffForm(defaultTariffForm());
    setPCForm(defaultPCForm());
    setUserForm(defaultUserForm());
    setMessage('');
    setError('');
  }

  function updateClubName(name: string) {
    if (!clubForm) return;
    setClubForm({ ...clubForm, name, slug: slugifyClient(name) });
  }

  function updatePCLabel(label: string) {
    setPCForm((current) => ({
      ...current,
      label,
      external_pc_id: uniqueExternalPCID(label, settings?.pcs || [], current.id),
    }));
  }

  async function saveClub() {
    if (!clubForm) return;
    if (canManageNetwork && !clubForm.network_id) {
      setError('Выберите сеть клубов');
      return;
    }
    const clubPayload = { ...clubForm, slug: slugifyClient(clubForm.name), timezone: 'Asia/Tashkent' };
    if (creatingClub) {
      try {
        setError('');
        setMessage('');
        const payload = await api<{ id: string }>('/api/backoffice/clubs', { method: 'POST', body: JSON.stringify(clubPayload) });
        onClubChange(payload.id);
        await onReloadAuth();
        setMessage('Клуб добавлен');
      } catch (err) {
        setError(String((err as Error).message || err));
      }
      return;
    }
    await persistSettings(
      () => api(`/api/backoffice/clubs/${clubForm.id}`, { method: 'POST', body: JSON.stringify(clubPayload) }),
      'Настройки клуба сохранены',
      true,
    );
  }

  async function saveNetwork() {
    if (!networkForm.name.trim()) {
      setError('Укажите название сети');
      return;
    }
    try {
      setError('');
      setMessage('');
      const path = networkForm.id ? `/api/backoffice/networks/${networkForm.id}` : '/api/backoffice/networks';
      await api(path, { method: 'POST', body: JSON.stringify({ name: networkForm.name, status: networkForm.status }) });
      const nextNetworks = await loadNetworks();
      await onReloadAuth();
      if (creatingClub && !clubForm?.network_id) {
        const activeNetwork = nextNetworks.find((network) => network.status === 'active');
        if (activeNetwork) setClubForm((current) => current ? { ...current, network_id: activeNetwork.id, network_name: activeNetwork.name } : current);
      }
      setShowNetworkForm(false);
      setMessage(networkForm.id ? 'Сеть обновлена' : 'Сеть добавлена');
    } catch (err) {
      setError(String((err as Error).message || err));
    }
  }

  async function deleteClub() {
    if (!clubForm?.id || !window.confirm(`Удалить клуб "${clubForm.name}"? Он пропадёт из панели и QR-оплаты.`)) return;
    try {
      setError('');
      setMessage('');
      await api(`/api/backoffice/clubs/${clubForm.id}`, { method: 'DELETE' });
      const nextClubID = auth.clubs.find((club) => club.id !== clubForm.id)?.id || '';
      if (nextClubID) onClubChange(nextClubID);
      await onReloadAuth();
      setMessage('Клуб удалён');
    } catch (err) {
      setError(String((err as Error).message || err));
    }
  }

  async function saveZone() {
    const payload = {
      ...zoneForm,
      hourly_price_uzs: Number(zoneForm.hourly_price_uzs || 0),
      sort_order: Number(zoneForm.sort_order || 0),
    };
    const path = zoneForm.id ? `/api/backoffice/zones/${zoneForm.id}` : `/api/backoffice/clubs/${selectedClubID}/zones`;
    await persistSettings(
      () => api(path, { method: 'POST', body: JSON.stringify(payload) }),
      zoneForm.id ? 'Зона обновлена' : 'Зона добавлена',
    );
  }

  async function deleteZone() {
    if (!zoneForm.id || !window.confirm(`Удалить зону "${zoneForm.name}" вместе с её пакетами и ПК?`)) return;
    await persistSettings(
      () => api(`/api/backoffice/zones/${zoneForm.id}`, { method: 'DELETE' }),
      'Зона удалена',
    );
  }

  async function saveTariff() {
    const payload = {
      ...tariffForm,
      duration_minutes: Number(tariffForm.duration_minutes || 0),
      price_uzs: Number(tariffForm.price_uzs || 0),
      sort_order: Number(tariffForm.sort_order || 0),
    };
    const path = tariffForm.id ? `/api/backoffice/tariffs/${tariffForm.id}` : `/api/backoffice/clubs/${selectedClubID}/tariffs`;
    await persistSettings(
      () => api(path, { method: 'POST', body: JSON.stringify(payload) }),
      tariffForm.id ? 'Пакет обновлён' : 'Пакет добавлен',
    );
  }

  async function deleteTariff() {
    if (!tariffForm.id || !window.confirm(`Удалить пакет "${tariffForm.name}"?`)) return;
    await persistSettings(
      () => api(`/api/backoffice/tariffs/${tariffForm.id}`, { method: 'DELETE' }),
      'Пакет удалён',
    );
  }

  async function savePC() {
    const payload = {
      zone_id: pcForm.zone_id,
      number: Number(pcForm.number || 0),
      label: pcForm.label,
      external_pc_id: pcForm.external_pc_id || uniqueExternalPCID(pcForm.label || '', settings?.pcs || [], pcForm.id),
		mac_address: pcForm.mac_address || '',
      status: pcForm.id ? pcForm.status || 'available' : 'available',
    };
    const path = pcForm.id ? `/api/backoffice/pcs/${pcForm.id}` : `/api/backoffice/clubs/${selectedClubID}/pcs`;
    await persistSettings(
      () => api(path, { method: 'POST', body: JSON.stringify(payload) }),
      pcForm.id ? 'Компьютер обновлён' : 'Компьютер добавлен',
    );
  }

  async function deletePC() {
    if (!pcForm.id || !window.confirm(`Удалить "${pcForm.label || 'ПК'}" и отключить его QR?`)) return;
    await persistSettings(
      () => api(`/api/backoffice/pcs/${pcForm.id}`, { method: 'DELETE' }),
      'Компьютер удалён',
    );
  }

  function printPCQR(pc: ManagedPC) {
    if (!pc.qr_url) {
      setError('Для этого ПК QR ещё не создан');
      return;
    }
    // Do not pass `noopener`/`noreferrer` here: Chromium intentionally returns
    // `null` for that combination even when it did create the blank popup. That
    // made the UI report a blocked popup and left the print window blank.
    const popup = window.open('', '_blank', 'width=520,height=720');
    if (!popup) {
      setError('Браузер заблокировал окно печати. Разрешите всплывающие окна для Clubpay.');
      return;
    }
    const escape = (value: string) => value.replace(/[&<>'"]/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[char] || char));
    const qrImage = `https://api.qrserver.com/v1/create-qr-code/?size=360x360&margin=10&data=${encodeURIComponent(pc.qr_url)}`;
    const opensTelegram = /^https:\/\/t\.me\//i.test(pc.qr_url);
    const note = opensTelegram
      ? 'Отсканируйте QR — откроется ваш профиль Clubpay в Telegram.'
      : 'Отсканируйте QR, чтобы оплатить игровое время на этом ПК.';
    popup.document.write(`<!doctype html><html lang="ru"><head><meta charset="utf-8"><title>QR ${escape(pc.label)}</title><style>body{margin:0;min-height:100vh;display:grid;place-items:center;font-family:Arial,sans-serif;color:#111}.card{width:390px;text-align:center;border:1px solid #ddd;border-radius:20px;padding:32px}.club{font-size:14px;color:#666}.pc{font-size:30px;font-weight:800;margin:8px 0 22px}.qr{width:360px;height:360px}.note{margin:20px 0 0;font-size:15px;line-height:1.45;color:#444}@media print{.card{border:0}}</style></head><body><main class="card"><div class="club">${escape(clubForm?.name || 'ClubPay')}</div><div class="pc">${escape(pc.label)}</div><img class="qr" src="${qrImage}" alt="QR-код оплаты" onload="window.print()"><p class="note">${note}</p></main></body></html>`);
    popup.document.close();
  }

  async function rotatePCQR(pc: ManagedPC) {
    if (!window.confirm(`Перевыпустить QR для «${pc.label}»? Старый напечатанный QR сразу перестанет работать.`)) return;
    await persistSettings(
      () => api(`/api/backoffice/pcs/${pc.id}/qr/rotate`, { method: 'POST' }),
      'Новый QR создан. Распечатайте и замените старую наклейку.',
    );
  }

  async function saveUser() {
    const path = userForm.id ? `/api/backoffice/users/${userForm.id}/clubs/${selectedClubID}` : `/api/backoffice/clubs/${selectedClubID}/users`;
    const payload = { ...userForm, role: canManageNetwork && userForm.role === 'owner' ? 'owner' : 'admin' };
    await persistSettings(
      () => api(path, { method: 'POST', body: JSON.stringify(payload) }),
      userForm.id ? 'Доступ пользователя обновлён' : 'Пользователь добавлен',
    );
  }

  async function deleteUser() {
    const scopeText = userForm.scope === 'network' ? ' ко всей сети клубов' : ' к этому клубу';
    if (!userForm.id || !window.confirm(`Удалить доступ пользователя "${userForm.name}"${scopeText}?`)) return;
    await persistSettings(
      () => api(`/api/backoffice/users/${userForm.id}/clubs/${selectedClubID}`, { method: 'DELETE' }),
      'Доступ пользователя удалён',
    );
  }

  const requestedSettingsSection = currentPath.startsWith('/settings/networks')
    ? 'networks'
    : currentPath.startsWith('/settings/zones')
    ? 'zones'
    : currentPath.startsWith('/settings/connections')
      ? 'connections'
      : currentPath.startsWith('/settings/fiscal')
        ? 'fiscal'
    : currentPath.startsWith('/settings/pcs')
      ? 'pcs'
      : currentPath.startsWith('/settings/users')
        ? 'users'
        : 'club';
  const settingsSection = requestedSettingsSection === 'networks' && !canManageNetwork ? 'club' : requestedSettingsSection;
  const settingsTitle = {
    networks: 'Сети',
    club: 'Клуб',
    connections: 'Подключения',
    fiscal: 'Фискализация',
    zones: 'Зоны и пакеты',
    pcs: 'Компьютеры',
    users: 'Доступы',
  }[settingsSection];
  const visibleUsers = settings
    ? canManageNetwork
      ? settings.users.filter((user) => user.role === 'owner' || user.role === 'admin' || user.role === 'manager')
      : settings.users.filter((user) => user.role === 'admin' || user.role === 'manager')
    : [];
  const activeZoneTabs = settings?.zones.filter((zone) => zone.status !== 'deleted') || [];
  const filteredTariffs = settings
    ? settings.tariffs.filter((tariff) => !tariffZoneFilter || tariff.zone_id === tariffZoneFilter)
    : [];
  const filteredPCs = settings
    ? settings.pcs.filter((pc) => !pcZoneFilter || pc.zone_id === pcZoneFilter)
    : [];
  const activeNetworks = networks.filter((network) => network.status === 'active');
  const networkOptions = networks
    .filter((network) => network.status === 'active' || network.id === clubForm?.network_id)
    .map((network) => ({ value: network.id, label: network.name }));

  return (
    <main className="shell workspace-shell">
      <WorkspaceHeader
        auth={auth}
        selectedClubID={selectedClubID}
        currentPath={currentPath}
        onClubChange={onClubChange}
        onLogout={onLogout}
        eyebrow="Настройки"
        title={settingsTitle}
        clubAction={canManageNetwork && settingsSection !== 'networks' ? <Button size="sm" variant="secondary" icon={<Plus size={13} />} onClick={startNewClub} disabled={activeNetworks.length === 0}>Новый клуб</Button> : undefined}
      />
      {message && <Notice tone="success">{message}</Notice>}
      {error && <Notice tone="danger">{error}</Notice>}
      <div className="settings-workbench">
        {!creatingClub && clubForm?.id && (
          <div className="settings-subnav">
            {canManageNetwork && <LinkButton href="/settings/networks" variant={settingsSection === 'networks' ? 'secondary' : 'ghost'} icon={<Network size={16} />}>Сети</LinkButton>}
            <LinkButton href="/settings" variant={settingsSection === 'club' ? 'secondary' : 'ghost'} icon={<Building2 size={16} />}>Клуб</LinkButton>
            {canManageNetwork && <LinkButton href="/settings/connections" variant={settingsSection === 'connections' ? 'secondary' : 'ghost'} icon={<CreditCard size={16} />}>Подключения</LinkButton>}
            {canManageNetwork && <LinkButton href="/settings/fiscal" variant={settingsSection === 'fiscal' ? 'secondary' : 'ghost'} icon={<ReceiptText size={16} />}>Фискализация</LinkButton>}
            <LinkButton href="/settings/zones" variant={settingsSection === 'zones' ? 'secondary' : 'ghost'} icon={<ReceiptText size={16} />}>Зоны и пакеты</LinkButton>
            <LinkButton href="/settings/pcs" variant={settingsSection === 'pcs' ? 'secondary' : 'ghost'} icon={<Monitor size={16} />}>Компьютеры</LinkButton>
            <LinkButton href="/settings/users" variant={settingsSection === 'users' ? 'secondary' : 'ghost'} icon={<Users size={16} />}>Доступы</LinkButton>
          </div>
        )}
        <div className="settings-stage">
      {!settings || !clubForm ? (
        <Panel><EmptyState text="Загружаем настройки" /></Panel>
      ) : (
        <section className="settings-grid">
          {!creatingClub && canManageNetwork && settingsSection === 'networks' && (
          <Panel className="stack settings-wide">
            <SectionTitle icon={<Network size={18} />} title="Сети клубов" caption="Сеть объединяет клубы под одним владельцем. Владелец сети получает доступ ко всем ее клубам." />
            <div className="compact-list">
              {networks.map((network) => (
                <button className="editable-row" key={network.id} onClick={() => editNetwork(network)}>
                  <span>
                    <strong>{network.name}</strong>
                    <small>{network.slug} · {statusLabel(network.status)}</small>
                  </span>
                  <em>Изменить</em>
                </button>
              ))}
              {networks.length === 0 && <EmptyState text="Сетей пока нет" />}
            </div>
            <div className="button-row">
              <Button size="sm" variant="secondary" icon={<Plus size={13} />} onClick={resetNetwork}>Новая сеть</Button>
            </div>
            {showNetworkForm && <div className="inline-editor">
              <div className="form-mode">
                <strong>{networkForm.id ? 'Редактирование сети' : 'Новая сеть'}</strong>
                <span>Название используется в переключателе клубов и в доступах владельцев.</span>
              </div>
              <div className="form-grid one-col">
                <Field label="Название сети" value={networkForm.name} onChange={(value) => setNetworkForm({ ...networkForm, name: value })} />
                <SelectField label="Статус" value={networkForm.status} options={statusOptions()} onChange={(value) => setNetworkForm({ ...networkForm, status: value })} />
              </div>
              <div className="button-row">
                <Button variant="ghost" onClick={() => setShowNetworkForm(false)}>Отменить</Button>
                <Button icon={<Save size={16} />} onClick={saveNetwork}>{networkForm.id ? 'Сохранить сеть' : 'Создать сеть'}</Button>
              </div>
            </div>}
          </Panel>
          )}

          {settingsSection === 'club' && (
          <Panel className="stack settings-wide">
            <SectionTitle
              icon={<Building2 size={18} />}
              title={creatingClub ? 'Новый клуб' : 'Клуб'}
              caption={creatingClub ? 'Создайте рабочий контур клуба. Подключения и каталог настраиваются после сохранения.' : 'Название, юридические данные и адрес выбранного клуба.'}
            />
            <div className="form-mode">
              <strong>{creatingClub ? 'Создание клуба' : 'Редактирование клуба'}</strong>
              <span>{creatingClub ? 'Заполните название и платежные настройки. Зоны, пакеты и ПК добавите после сохранения.' : 'Изменения применятся к выбранному клубу.'}</span>
            </div>
            <div className="form-grid two">
              {canManageNetwork && <SelectField label="Сеть клубов" value={clubForm.network_id} options={networkOptions} placeholder={networkOptions.length ? 'Выберите сеть' : 'Нет активных сетей'} disabled={networkOptions.length === 0} onChange={(value) => {
                const network = networks.find((item) => item.id === value);
                setClubForm({ ...clubForm, network_id: value, network_name: network?.name || '' });
              }} help="Владелец сети получает доступ ко всем клубам внутри нее." />}
              <Field label="Название клуба" value={clubForm.name} onChange={updateClubName} help="Так клуб будет называться в панели и на QR-странице." />
              <Field label="Юр. название" value={clubForm.legal_name} onChange={(value) => setClubForm({ ...clubForm, legal_name: value })} help="Официальное название юрлица для договора и чеков." />
              <Field label="ИНН" value={clubForm.tin} onChange={(value) => setClubForm({ ...clubForm, tin: value })} help="Налоговый номер клуба." />
              <Field label="Адрес" value={clubForm.address} onChange={(value) => setClubForm({ ...clubForm, address: value })} help="Адрес клуба или юрлица." />
            </div>
            {!canManageNetwork && <ClubConnectionSummary club={clubForm} />}
            <div className="button-row settings-savebar">
              {creatingClub && selectedClubID && <Button variant="ghost" onClick={loadSettings}>Отменить</Button>}
              {!creatingClub && canManageNetwork && <Button variant="danger" icon={<Trash2 size={16} />} onClick={deleteClub}>Удалить клуб</Button>}
              <Button icon={<Save size={16} />} onClick={saveClub}>{creatingClub ? 'Создать клуб' : 'Сохранить клуб'}</Button>
            </div>
          </Panel>
          )}

          {!creatingClub && canManageNetwork && settingsSection === 'connections' && (
          <Panel className="stack settings-wide">
            <SectionTitle icon={<CreditCard size={18} />} title="Платёжные подключения" caption="Технические параметры Click и Payme доступны только суперадмину." />
            <div className="connection-status-line">
              <span>Текущий статус клуба</span>
              <span className={`connection-pill ${clubForm.status === 'active' ? 'ready' : 'waiting'}`}>{clubForm.status === 'active' ? 'Активен' : 'Отключен'}</span>
            </div>
            <div className="form-grid two">
              <Field label="Click merchant ID" value={clubForm.click_merchant_id} onChange={(value) => setClubForm({ ...clubForm, click_merchant_id: value })} help="Обязательный ID поставщика Click для платежной ссылки." />
              <Field label="Click service ID" value={clubForm.click_service_id} onChange={(value) => setClubForm({ ...clubForm, click_service_id: value })} help="Обязательный ID сервиса Click, по нему создается платежная ссылка." />
              <Field label="Click merchant user ID" value={clubForm.click_merchant_user_id} onChange={(value) => setClubForm({ ...clubForm, click_merchant_user_id: value })} help="ID пользователя мерчанта Click, который выдают вместе с service ID." />
              <Field label="Click secret key" type="password" value={clubForm.click_secret_key} onChange={(value) => setClubForm({ ...clubForm, click_secret_key: value })} help="Секрет для проверки Prepare/Complete callback." />
              <Field label="Payme merchant ID" value={clubForm.payme_merchant_id} onChange={(value) => setClubForm({ ...clubForm, payme_merchant_id: value })} help="ID кассы/мерчанта Payme для checkout-ссылки." />
              <Field label="Payme secret key" type="password" value={clubForm.payme_secret_key} onChange={(value) => setClubForm({ ...clubForm, payme_secret_key: value })} help="Для sandbox укажите TEST_KEY из Payme Business; для production - боевой secret." />
              <SelectField label="Статус клуба" value={clubForm.status} options={statusOptions()} onChange={(value) => setClubForm({ ...clubForm, status: value })} help="Отключенный клуб не должен принимать новые оплаты." />
            </div>
            <div className="button-row settings-savebar"><Button icon={<Save size={16} />} onClick={saveClub}>Сохранить подключения</Button></div>
          </Panel>
          )}

          {!creatingClub && canManageNetwork && settingsSection === 'fiscal' && (
          <Panel className="stack settings-wide">
            <SectionTitle icon={<ReceiptText size={18} />} title="Фискализация" caption="Коды услуги и параметры чека для платёжных провайдеров." />
            <div className="form-grid two">
              <Field label="Название услуги для чека" value={clubForm.ofd_service_name} onChange={(value) => setClubForm({ ...clubForm, ofd_service_name: value })} help="Например: Компьютерное время. Можно оставить по умолчанию." />
              <Field label="ИКПУ / MXIK" value={clubForm.ofd_mxik} onChange={(value) => setClubForm({ ...clubForm, ofd_mxik: value })} help="Заполняем после бухгалтера. Если пусто, Payme уйдет без detail.items и сможет работать со статичной кассой." />
              <Field label="package_code" value={clubForm.ofd_package_code} onChange={(value) => setClubForm({ ...clubForm, ofd_package_code: value })} help="Заполняем после бухгалтера вместе с IKPU/MXIK." />
              <Field label="unit_code" value={clubForm.ofd_unit_code} onChange={(value) => setClubForm({ ...clubForm, ofd_unit_code: value })} help="Опционально: код единицы измерения, если его требует платежка или OFD." />
              <Field label="НДС, %" type="number" min="0" value={String(clubForm.ofd_vat_percent ?? 0)} onChange={(value) => setClubForm({ ...clubForm, ofd_vat_percent: Number(value || 0) })} help="Заполняем вместе с кодами. 0 - без НДС или ставка не применяется." />
            </div>
            <div className="button-row settings-savebar"><Button icon={<Save size={16} />} onClick={saveClub}>Сохранить фискализацию</Button></div>
          </Panel>
          )}

          {!creatingClub && settingsSection === 'zones' && (
            <>
          <Panel className="stack">
            <SectionTitle icon={<Settings size={18} />} title="Зоны" caption="Зона группирует ПК с одной базовой ценой за час, например Standard, VIP или Bootcamp." />
            <div className="compact-list">
              {settings.zones.map((zone) => (
                <button className="editable-row" key={zone.id} onClick={() => { setZoneForm(zone); setShowZoneForm(true); }}>
                  <span>
                    <strong>{zone.name}</strong>
                    <small>1 час: {formatUZS(zone.hourly_price_uzs)} · {statusLabel(zone.status)}</small>
                  </span>
                  <em>Изменить</em>
                </button>
              ))}
            </div>
            <div className="button-row">
              <Button size="sm" variant="secondary" icon={<Plus size={13} />} onClick={resetZone}>Новая зона</Button>
            </div>
            {showZoneForm && (
              <div className="inline-editor">
                <div className="form-mode">
                  <strong>{zoneForm.id ? 'Редактирование зоны' : 'Новая зона'}</strong>
                  <span>Клик по зоне выше открывает редактирование. Для создания нажмите “Новая зона”.</span>
                </div>
                <div className="form-grid one-col">
                  <Field label="Название зоны" value={zoneForm.name || ''} onChange={(value) => setZoneForm({ ...zoneForm, name: value })} />
                  <CurrencyField label="Стоимость 1 часа, сум" value={zoneForm.hourly_price_uzs || 0} onChange={(value) => setZoneForm({ ...zoneForm, hourly_price_uzs: value })} help="Используется, когда клиент или менеджер вводит сумму вручную без пакета." />
                  <SelectField label="Статус" value={zoneForm.status || 'active'} options={zoneStatusOptions()} onChange={(value) => setZoneForm({ ...zoneForm, status: value })} />
                </div>
                <div className="button-row">
                  {zoneForm.id && <Button variant="danger" icon={<Trash2 size={16} />} onClick={deleteZone}>Удалить</Button>}
                  <Button icon={<Save size={16} />} onClick={saveZone}>{zoneForm.id ? 'Сохранить зону' : 'Добавить зону'}</Button>
                </div>
              </div>
            )}
          </Panel>

          <Panel className="stack">
            <SectionTitle icon={<ReceiptText size={18} />} title="Пакеты" caption="Пакет выбирает клиент на QR-странице. Цена пакета фиксированная и не зависит от стоимости часа зоны." />
            {activeZoneTabs.length > 1 && (
              <div className="tab-row">
                {activeZoneTabs.map((zone) => (
                  <button type="button" key={zone.id} className={tariffZoneFilter === zone.id ? 'active' : ''} onClick={() => setTariffZoneFilter(zone.id)}>
                    {zone.name}
                  </button>
                ))}
              </div>
            )}
            <div className="compact-list">
              {filteredTariffs.map((tariff) => (
                <button className="editable-row" key={tariff.id} onClick={() => { setTariffForm(tariff); setShowTariffForm(true); }}>
                  <span>
                    <strong>{tariff.zone} · {tariff.name}</strong>
                    <small>{tariff.duration_minutes} мин · {formatUZS(tariff.price_uzs)} · {statusLabel(tariff.status || '')}</small>
                  </span>
                  <em>Изменить</em>
                </button>
              ))}
            </div>
            <div className="button-row">
              <Button size="sm" variant="secondary" icon={<Plus size={13} />} onClick={resetTariff}>Новый пакет</Button>
            </div>
            {showTariffForm && (
              <div className="inline-editor">
                <div className="form-mode">
                  <strong>{tariffForm.id ? 'Редактирование пакета' : 'Новый пакет'}</strong>
                  <span>Если хотите добавить пакет, сначала нажмите “Новый пакет”.</span>
                </div>
                <div className="form-grid two">
                  <SelectField label="Зона" value={tariffForm.zone_id || ''} options={settings.zones.map((zone) => ({ value: zone.id, label: zone.name }))} onChange={(value) => setTariffForm({ ...tariffForm, zone_id: value })} />
                  <Field label="Название" value={tariffForm.name || ''} onChange={(value) => setTariffForm({ ...tariffForm, name: value })} />
                  <Field label="Минуты" type="number" value={String(tariffForm.duration_minutes || 0)} onChange={(value) => setTariffForm({ ...tariffForm, duration_minutes: Number(value || 0) })} />
                  <CurrencyField label="Цена, сум" value={tariffForm.price_uzs || 0} onChange={(value) => setTariffForm({ ...tariffForm, price_uzs: value })} />
                  <SelectField label="Статус" value={tariffForm.status || 'active'} options={statusOptions()} onChange={(value) => setTariffForm({ ...tariffForm, status: value })} />
                </div>
                <div className="button-row">
                  {tariffForm.id && <Button variant="danger" icon={<Trash2 size={16} />} onClick={deleteTariff}>Удалить</Button>}
                  <Button icon={<Save size={16} />} onClick={saveTariff}>{tariffForm.id ? 'Сохранить пакет' : 'Добавить пакет'}</Button>
                </div>
              </div>
            )}
          </Panel>
            </>
          )}

          {!creatingClub && settingsSection === 'pcs' && (
          <Panel className="stack settings-wide">
            <SectionTitle icon={<Monitor size={18} />} title="Компьютеры" caption="Для каждого ПК QR создаётся автоматически. Распечатайте его и разместите у компьютера; системный ID в QR не раскрывается." />
            <div className="table-filter">
              <label>
                Зона
                <select value={pcZoneFilter} onChange={(event) => setPCZoneFilter(event.target.value)}>
                  <option value="">Все зоны</option>
                  {settings.zones.map((zone) => <option key={zone.id} value={zone.id}>{zone.name}</option>)}
                </select>
              </label>
            </div>
            <div className="table-wrap">
              <table className="pc-table settings-table">
                <thead>
                  <tr>
                    <th>ПК</th>
                    <th>Зона</th>
                    <th>QR</th>
                    <th className="actions-col">Действие</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredPCs.map((pc) => (
                    <tr key={pc.id}>
                      <td>{pc.label} · #{pc.number}</td>
                      <td>{pc.zone}</td>
                      <td>{pc.qr_url ? <span className="text-link">создан</span> : '-'}</td>
                      <td>
                        <div className="row-actions">
                          <Button size="sm" variant="ghost" icon={<QrCode size={14} />} onClick={() => printPCQR(pc)}>Печать</Button>
                          <Button size="sm" variant="ghost" icon={<RefreshCw size={14} />} onClick={() => rotatePCQR(pc)}>Перевыпустить</Button>
                          <Button size="sm" variant="ghost" onClick={() => { setPCForm(pc); setShowPCForm(true); }}>Изменить</Button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="button-row">
              <Button size="sm" variant="secondary" icon={<Plus size={13} />} onClick={resetPC}>Новый ПК</Button>
            </div>
            {showPCForm && (
              <div className="inline-editor">
                <div className="form-mode">
                  <strong>{pcForm.id ? 'Редактирование ПК' : 'Новый ПК'}</strong>
                  <span>QR и системный ID создаются автоматически после сохранения.</span>
                </div>
                <div className="form-grid two">
                  <SelectField label="Зона" value={pcForm.zone_id || ''} options={settings.zones.map((zone) => ({ value: zone.id, label: zone.name }))} onChange={(value) => setPCForm({ ...pcForm, zone_id: value })} />
                  <Field label="Номер" type="number" value={String(pcForm.number || 0)} onChange={(value) => setPCForm({ ...pcForm, number: Number(value || 0) })} />
                  <Field label="Название" value={pcForm.label || ''} onChange={updatePCLabel} />
				  <Field label="MAC-адрес (для Wake-on-LAN)" value={pcForm.mac_address || ''} onChange={(value) => setPCForm({ ...pcForm, mac_address: value })} />
                </div>
                <div className="button-row">
                  {pcForm.id && <Button variant="danger" icon={<Trash2 size={16} />} onClick={deletePC}>Удалить</Button>}
                  <Button icon={<Save size={16} />} onClick={savePC}>{pcForm.id ? 'Сохранить ПК' : 'Добавить ПК'}</Button>
                </div>
              </div>
            )}
          </Panel>
          )}

          {!creatingClub && settingsSection === 'users' && (
          <Panel className="stack settings-wide">
            <SectionTitle icon={<Users size={18} />} title="Доступы" caption="Кто может заходить в панель выбранного клуба." />
            <div className="compact-list">
              {visibleUsers.map((user) => (
                <button className="editable-row" key={user.id} onClick={() => { setUserForm({ ...user, role: canManageNetwork && user.role === 'owner' ? 'owner' : 'admin', password: '' }); setShowUserForm(true); }}>
                  <span>
                    <strong>{user.name} · {roleLabel(user.role)}</strong>
                    <small>{user.email || user.phone} · {user.scope === 'network' ? 'Доступ ко всей сети · ' : ''}{statusLabel(user.status)}</small>
                  </span>
                  <em>{user.scope === 'network' ? 'Сеть' : 'Изменить'}</em>
                </button>
              ))}
              {visibleUsers.length === 0 && <EmptyState text="Менеджеры клуба пока не добавлены" />}
            </div>
            <div className="button-row">
              <Button size="sm" variant="secondary" icon={<Plus size={13} />} onClick={resetUser}>{canManageNetwork ? 'Новый пользователь' : 'Новый менеджер'}</Button>
            </div>
            {showUserForm && <div className="inline-editor user-editor">
              <div className="form-mode">
                <strong>{userForm.id ? 'Редактирование доступа' : 'Новый пользователь'}</strong>
                <span>{canManageNetwork ? 'Владелец получает доступ ко всем клубам выбранной сети. Менеджер добавляется только в выбранный клуб.' : 'Владелец может добавлять только менеджеров. Владельцев добавляет команда Clubpay.'}</span>
              </div>
              <div className="form-grid">
                <Field label="Имя" value={userForm.name || ''} onChange={(value) => setUserForm({ ...userForm, name: value })} />
                <Field label="Email" value={userForm.email || ''} onChange={(value) => setUserForm({ ...userForm, email: value })} />
                <Field label="Телефон" value={userForm.phone || ''} onChange={(value) => setUserForm({ ...userForm, phone: value })} />
                <Field label="Пароль" type="password" value={userForm.password || ''} onChange={(value) => setUserForm({ ...userForm, password: value })} />
                {canManageNetwork ? (
                  <SelectField label="Роль" value={userForm.role || 'admin'} options={roleOptions(canManageNetwork)} onChange={(value) => setUserForm({ ...userForm, role: value })} />
                ) : (
                  <label className="form-block">
                    Роль
                    <input value="Менеджер" readOnly />
                  </label>
                )}
                <SelectField label="Статус" value={userForm.status || 'active'} options={statusOptions()} onChange={(value) => setUserForm({ ...userForm, status: value })} />
              </div>
              <div className="button-row">
                {userForm.id && <Button variant="danger" icon={<Trash2 size={16} />} onClick={deleteUser}>Удалить</Button>}
                <Button icon={<Save size={16} />} onClick={saveUser}>{userForm.id ? 'Сохранить доступ' : canManageNetwork ? 'Добавить пользователя' : 'Добавить менеджера'}</Button>
              </div>
            </div>}
          </Panel>
          )}
        </section>
      )}
        </div>
      </div>
    </main>
  );
}

function PaymentReturnPage() {
  const params = new URLSearchParams(window.location.search);
  const invoiceID = params.get('invoice_id') || '';
  const [order, setOrder] = useState<Order | null>(null);
  const [error, setError] = useState('');

  async function loadOrder() {
    if (!invoiceID) return;
    try {
      const payload = await api<{ order: Order }>(`/api/orders/${invoiceID}`);
      setOrder(payload.order);
    } catch (err) {
      setError(String((err as Error).message || err));
    }
  }

  useEffect(() => {
    loadOrder();
    const timer = window.setInterval(loadOrder, 3000);
    return () => window.clearInterval(timer);
  }, [invoiceID]);

  async function syncProvider() {
    if (!order?.invoice_id) return;
    await api(`/api/payments/sync/${order.invoice_id}`, { method: 'POST' });
    loadOrder();
  }

  async function testPay() {
    if (!invoiceID) return;
    await api(`/api/payments/mock/success/${invoiceID}`, { method: 'POST' });
    loadOrder();
  }

  if (!invoiceID) return <Centered text="Не найден номер заказа" />;
  if (error && !order) return <Centered text={error} />;

  const paid = order?.status === 'paid';
  const failed = order?.status === 'failed' || order?.status === 'refunded';
  const retryHref = order?.external_pc_id ? `/qr/${order.external_pc_id}` : '/';

  return (
    <main className="shell narrow">
      <Panel className={`payment-state ${paid ? 'paid' : failed ? 'failed' : 'waiting'}`}>
        {paid ? <CheckCircle2 size={44} /> : failed ? <AlertCircle size={44} /> : <Clock3 size={44} />}
        <h1>{paid ? 'Оплата принята' : failed ? 'Оплата не прошла' : 'Ждём подтверждение оплаты'}</h1>
        <p>{order ? `${order.pc_label} · ${order.tariff} · ${providerLabel(order.provider)} · ${orderStatusLabel(order.status)}` : 'Загружаем статус заказа...'}</p>
        <div className="payment-order-meta">
          <span>Order ID</span>
          <code>{order?.invoice_id || invoiceID}</code>
        </div>
        {order?.receipt_url && <LinkButton href={order.receipt_url} icon={<ReceiptText size={16} />}>Квитанция оплаты</LinkButton>}
        <div className="button-row centered-actions">
          <Button variant="ghost" icon={<RefreshCw size={16} />} onClick={syncProvider} disabled={!order || paid || failed}>
            Обновить статус
          </Button>
          <Button variant="success" icon={<Play size={16} />} onClick={testPay} disabled={paid}>
            Тестовая оплата
          </Button>
          {failed && <LinkButton href={retryHref} variant="primary" icon={<CreditCard size={16} />}>Оплатить заново</LinkButton>}
        </div>
      </Panel>
    </main>
  );
}

type WorkspaceNavItem = {
  href: string;
  label: string;
  icon: React.ReactNode;
  prefix?: boolean;
};

function workspaceNavItems(auth: AuthPayload, clubID: string): WorkspaceNavItem[] {
  const role = clubRole(auth, clubID);
  if (role === 'super_admin') {
    return [
      { href: '/reports', label: 'Обзор', icon: <Activity size={18} /> },
      { href: '/admin', label: 'Операции', icon: <Monitor size={18} /> },
      { href: '/settings/networks', label: 'Сети', icon: <Network size={18} /> },
      { href: '/settings', label: 'Клубы', icon: <Building2 size={18} /> },
      { href: '/settings/connections', label: 'Подключения', icon: <CreditCard size={18} /> },
      { href: '/settings/users', label: 'Доступы', icon: <Users size={18} /> },
    ];
  }
  if (role === 'owner') {
    return [
      { href: '/reports', label: 'Обзор', icon: <Activity size={18} /> },
      { href: '/admin', label: 'Зал', icon: <Monitor size={18} /> },
      { href: '/admin/payments', label: 'Оплаты', icon: <ReceiptText size={18} /> },
      { href: '/settings', label: 'Настройки', icon: <Settings size={18} />, prefix: true },
      { href: '/settings/users', label: 'Команда', icon: <Users size={18} /> },
    ];
  }
  return [
    { href: '/admin', label: 'Зал', icon: <Monitor size={18} /> },
    { href: '/admin/sessions', label: 'Сессии', icon: <Gamepad2 size={18} /> },
    { href: '/admin/payments', label: 'Оплаты', icon: <ReceiptText size={18} /> },
  ];
}

function WorkspaceHeader({ auth, selectedClubID, currentPath, onClubChange, onLogout, eyebrow, title, clubAction }: WorkspaceProps & { eyebrow: string; title: string; clubAction?: React.ReactNode }) {
  const selectedClub = auth.clubs.find((club) => club.id === selectedClubID) || auth.clubs[0];
  const role = clubRole(auth, selectedClubID);
  const navItems = workspaceNavItems(auth, selectedClubID);

  return (
    <>
      <aside className="workspace-rail">
        <AppLink className="rail-brand" href={canViewOwner(auth, selectedClubID) ? '/reports' : '/admin'}>
          <span className="rail-mark" aria-hidden="true">CP</span>
          <span className="rail-brand-copy">
            <strong>ClubPay</strong>
            <small>{roleLabel(role)}</small>
          </span>
        </AppLink>
        <nav className="rail-nav" aria-label="Основная навигация">
          {navItems.map((item) => {
            const hasExactMatch = navItems.some((candidate) => !candidate.prefix && currentPath === candidate.href);
            const active = item.prefix ? currentPath.startsWith(item.href) && !hasExactMatch : currentPath === item.href;
            return (
              <AppLink className={`rail-link ${active ? 'active' : ''}`} href={item.href} key={`${item.href}-${item.label}`}>
                {item.icon}
                <span>{item.label}</span>
              </AppLink>
            );
          })}
        </nav>
        <div className="rail-user">
          <strong>{auth.user.name}</strong>
          <span>{auth.user.email || auth.user.phone}</span>
        </div>
      </aside>
      <header className="commandbar">
        <div className="command-title">
          <span>{eyebrow}</span>
          <h1>{title}</h1>
        </div>
        <div className="command-context">
          {auth.clubs.length > 1 ? (
            <label className="club-switcher">
              <span>Клуб</span>
              <select value={selectedClubID} onChange={(event) => onClubChange(event.target.value)} aria-label="Клуб">
                {auth.clubs.map((club) => <option key={club.id} value={club.id}>{club.network_name ? `${club.network_name} / ${club.name}` : club.name} · {roleLabel(club.role)}</option>)}
              </select>
            </label>
          ) : (
            <div className="club-context">
              <span>{selectedClub?.network_name ? 'Сеть / клуб' : 'Клуб'}</span>
              <strong>{selectedClub ? (selectedClub.network_name ? `${selectedClub.network_name} / ${selectedClub.name}` : selectedClub.name) : 'Не выбран'}</strong>
            </div>
          )}
          {clubAction}
          <Button className="icon-only" size="sm" variant="ghost" icon={<LogOut size={16} />} onClick={onLogout} aria-label="Выйти" title="Выйти" />
        </div>
      </header>
    </>
  );
}

function PageHeader({ eyebrow, title, aside }: { eyebrow: string; title: string; aside?: React.ReactNode }) {
  return (
    <header className="page-commandbar">
      <div className="topbar-title">
        <p className="eyebrow">{eyebrow}</p>
        <h1>{title}</h1>
      </div>
      {aside}
    </header>
  );
}

function SectionTitle({ icon, title, caption }: { icon: React.ReactNode; title: string; caption?: string }) {
  return (
    <div className="section-title">
      <span>{icon}</span>
      <div>
        <h2>{title}</h2>
        {caption && <p>{caption}</p>}
      </div>
    </div>
  );
}

function Panel({ children, className = '' }: { children: React.ReactNode; className?: string }) {
  return <section className={`panel ${className}`}>{children}</section>;
}

function Button({ children, icon, variant = 'primary', size = 'md', full, className = '', ...props }: ButtonProps) {
  return (
    <button className={`btn ${variant} ${size} ${full ? 'full' : ''} ${className}`} {...props}>
      {icon}
      <span>{children}</span>
    </button>
  );
}

function LinkButton({
  href,
  children,
  icon,
  variant = 'primary',
}: {
  href: string;
  children: React.ReactNode;
  icon?: React.ReactNode;
  variant?: 'primary' | 'secondary' | 'ghost' | 'success';
}) {
  return (
    <AppLink className={`btn ${variant}`} href={href}>
      {icon}
      <span>{children}</span>
    </AppLink>
  );
}

function AppLink({ href, children, className = '' }: { href: string; children: React.ReactNode; className?: string }) {
  return (
    <a className={className} href={href} onClick={(event) => handleInternalLinkClick(event, href)}>
      {children}
    </a>
  );
}

function Notice({ children, tone }: { children: React.ReactNode; tone: 'success' | 'danger' }) {
  return <p className={`notice ${tone}`}>{children}</p>;
}

function QRCodeImage({ value, size = 160, label }: { value: string; size?: number; label: string }) {
  const src = `https://api.qrserver.com/v1/create-qr-code/?size=${size}x${size}&margin=10&data=${encodeURIComponent(value)}`;
  return (
    <div className="qr-image-box" style={{ width: size, height: size }}>
      <img src={src} width={size} height={size} alt={label} loading="lazy" />
    </div>
  );
}

function EndSessionModal({
  draft,
  onPhoneChange,
  onRecipientConsentChange,
  onClose,
  onSubmit,
}: {
  draft: { grant: Grant; phone: string; recipientConsent: boolean; confirmWithoutPhone: boolean };
  onPhoneChange: (phone: string) => void;
  onRecipientConsentChange: (value: boolean) => void;
  onClose: () => void;
  onSubmit: () => void | Promise<void>;
}) {
  const remainingSeconds = draft.grant.remaining_seconds || draft.grant.duration_seconds || draft.grant.duration_minutes * 60;
  return (
    <div className="telegram-modal-backdrop" role="dialog" aria-modal="true" aria-labelledby="end-session-title">
      <section className="telegram-modal end-session-modal">
        <button className="modal-close" type="button" onClick={onClose} aria-label="Закрыть">
          <X size={18} />
        </button>
        <div className="telegram-modal-head">
          <span><Power size={22} /></span>
          <div>
            <p>{draft.grant.pc_label}</p>
            <h2 id="end-session-title">Завершить сессию</h2>
          </div>
        </div>
        <p className="telegram-modal-text">
          Остаток времени будет сохранен ваучером. Укажите телефон клиента, если нужно привязать ваучер к Telegram.
        </p>
        <label className="modal-field">
          Телефон клиента
          <input
            inputMode="tel"
            value={draft.phone}
            onFocus={() => !draft.phone && onPhoneChange('+998 ')}
            onChange={(event) => onPhoneChange(formatUzPhoneInput(event.target.value))}
            placeholder="+998 00 000 00 00"
          />
        </label>
        {draft.phone.trim() && (
          <label className="consent-control">
            <input
              type="checkbox"
              checked={draft.recipientConsent}
              onChange={(event) => onRecipientConsentChange(event.target.checked)}
            />
            <span>Клиент дал согласие на хранение номера и получение ваучера в Telegram.</span>
          </label>
        )}
        <div className={`end-session-warning ${draft.confirmWithoutPhone ? 'active' : ''}`}>
          {draft.confirmWithoutPhone
            ? 'Телефон не указан. Ваучер будет создан, но клиенту нужно будет получить код у администратора.'
            : `Будет завершено примерно ${formatDurationClock(remainingSeconds)}.`}
        </div>
        <div className="telegram-modal-actions horizontal">
          <Button type="button" variant="ghost" onClick={onClose}>Отменить</Button>
          <Button type="button" variant={draft.confirmWithoutPhone ? 'danger' : 'secondary'} icon={<Power size={16} />} onClick={onSubmit} disabled={Boolean(draft.phone.trim() && !draft.recipientConsent)}>
            {draft.phone.trim() ? 'Завершить и отправить' : draft.confirmWithoutPhone ? 'Да, завершить без телефона' : 'Завершить'}
          </Button>
        </div>
      </section>
    </div>
  );
}

function TelegramVoucherModal({ prompt, onClose, onCopied }: { prompt: TelegramPrompt; onClose: () => void; onCopied: () => void }) {
  const notConfigured = prompt.status === 'telegram_not_configured';
  return (
    <div className="telegram-modal-backdrop" role="dialog" aria-modal="true" aria-labelledby="telegram-modal-title">
      <section className="telegram-modal">
        <button className="modal-close" type="button" onClick={onClose} aria-label="Закрыть">
          <X size={18} />
        </button>
        <div className="telegram-modal-head">
          <span><QrCode size={22} /></span>
          <div>
            <p>Telegram-бот</p>
            <h2 id="telegram-modal-title">{notConfigured ? 'Telegram не подключен' : 'Покажите QR клиенту'}</h2>
          </div>
        </div>
        <QRCodeImage value={prompt.link} size={220} label="QR для получения ваучера в Telegram" />
        <p className="telegram-modal-text">
          {notConfigured
            ? `Серверу нужен TELEGRAM_BOT_TOKEN для автоматической отправки. Сейчас передайте клиенту код${prompt.code ? ` ${prompt.code}` : ''} вручную.`
            : `После нажатия Start бот привяжет номер${prompt.phone ? ` ${prompt.phone}` : ''} и отправит ваучер${prompt.code ? ` ${prompt.code}` : ''}${prompt.seconds || prompt.minutes ? ` на ${formatDurationClock(prompt.seconds || (prompt.minutes || 0) * 60)}` : ''}.`}
        </p>
        <div className="telegram-modal-actions">
          <LinkButton href={prompt.link} variant="secondary" icon={<Send size={16} />}>Открыть Telegram</LinkButton>
          <Button
            type="button"
            variant="ghost"
            icon={<Copy size={16} />}
            onClick={() => {
              navigator.clipboard?.writeText(prompt.link);
              onCopied();
            }}
          >
            Скопировать ссылку
          </Button>
        </div>
      </section>
    </div>
  );
}

function EmptyState({ text }: { text: string }) {
  return <p className="empty">{text}</p>;
}

function Metric({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <Panel className="metric">
      {icon}
      <span>{label}</span>
      <strong>{value}</strong>
    </Panel>
  );
}

function ClubConnectionSummary({ club }: { club: ClubSettings }) {
  const clickReady = club.click_connected ?? Boolean(club.click_merchant_id && club.click_service_id && club.click_merchant_user_id);
  const paymeReady = club.payme_connected ?? Boolean(club.payme_merchant_id);
  const paymentReady = club.payment_connected ?? (clickReady || paymeReady);
  const fiscalCodesReady = club.fiscal_connected ?? Boolean(club.ofd_mxik && club.ofd_package_code && club.ofd_service_name);
  const clubActive = club.status === 'active';
  const connectedProviders = [clickReady && 'Click', paymeReady && 'Payme'].filter(Boolean).join(', ');

  return (
    <div className="connection-summary">
      <div className="connection-summary-head">
        <div>
          <strong>Подключение клуба</strong>
          <span>Технические ключи Click/Payme заполняет команда Clubpay после подключения клуба. Коды чеков добавим после бухгалтера.</span>
        </div>
        <span className={`connection-pill ${clubActive ? 'ready' : 'waiting'}`}>{clubActive ? 'Клуб активен' : 'Клуб отключен'}</span>
      </div>
      <div className="connection-grid">
        <ConnectionItem
          icon={<CreditCard size={18} />}
          ready={paymentReady}
          title="Онлайн-оплата"
          value={paymentReady ? 'Подключена' : 'Ждёт подключения'}
          caption={paymentReady ? `Доступно: ${connectedProviders}.` : 'Нужно получить доступы Click и/или Payme.'}
        />
        <ConnectionItem
          icon={<Banknote size={18} />}
          ready={paymentReady}
          title="Деньги клуба"
          value={paymentReady ? 'На кассу клуба' : 'После подключения'}
          caption="В MVP онлайн-оплата идет на мерчант клуба. Clubpay оплачивается отдельно по подписке."
        />
        <ConnectionItem
          icon={<ReceiptText size={18} />}
          ready
          title="Фискальные чеки"
          value={fiscalCodesReady ? 'Коды сохранены' : 'Через кассу клуба'}
          caption={fiscalCodesReady ? 'Передадим данные услуги платежке.' : 'До кодов бухгалтера можно использовать статичную настройку кассы провайдера.'}
        />
        <ConnectionItem
          icon={<Settings size={18} />}
          ready={clubActive}
          title="Статус клуба"
          value={clubActive ? 'Активен' : 'Отключен'}
          caption="Отключенный клуб не принимает новые оплаты."
        />
      </div>
    </div>
  );
}

function ConnectionItem({ icon, ready, title, value, caption }: { icon: React.ReactNode; ready: boolean; title: string; value: string; caption: string }) {
  return (
    <div className={`connection-item ${ready ? 'ready' : 'waiting'}`}>
      <span className="connection-icon">{icon}</span>
      <div>
        <small>{title}</small>
        <strong>{value}</strong>
        <p>{caption}</p>
      </div>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  return <span className={`status ${status}`}>{pcStatusLabel(status)}</span>;
}

function Centered({ text }: { text: string }) {
  return <main className="centered"><AlertCircle size={24} /> {text}</main>;
}

function Field({
  label,
  value,
  onChange,
  type = 'text',
  step,
  min,
  help,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  type?: string;
  step?: string | number;
  min?: string | number;
  help?: string;
}) {
  return (
    <label>
      {label}
      <input type={type} value={value} step={step} min={min} onChange={(event) => onChange(event.target.value)} />
      {help && <small className="field-help">{help}</small>}
    </label>
  );
}

function CurrencyField({ label, value, onChange, help }: { label: string; value: number; onChange: (value: number) => void; help?: string }) {
  return (
    <label>
      {label}
      <input
        type="text"
        value={formatUZSInput(value)}
        inputMode="numeric"
        onChange={(event) => onChange(parseUZSInput(event.target.value))}
      />
      {help && <small className="field-help">{help}</small>}
    </label>
  );
}

function SelectField({
  label,
  value,
  options,
  onChange,
  help,
  placeholder,
  disabled,
}: {
  label: string;
  value: string;
  options: Array<{ value: string; label: string }>;
  onChange: (value: string) => void;
  help?: string;
  placeholder?: string;
  disabled?: boolean;
}) {
  return (
    <label>
      {label}
      <select value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)}>
        {placeholder && <option value="" disabled>{placeholder}</option>}
        {options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
      </select>
      {help && <small className="field-help">{help}</small>}
    </label>
  );
}

async function api<T = unknown>(path: string, init?: RequestInit): Promise<T> {
  const token = localStorage.getItem(TOKEN_KEY);
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(init?.headers || {}),
    },
  });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(localizeError(payload.error || `HTTP ${response.status}`));
  }
  return payload as T;
}

function localizeError(message: string) {
  const dictionary: Record<string, string> = {
    'invalid login or password': 'Неверный логин или пароль',
    'login and password are required': 'Введите логин и пароль',
    'auth required': 'Нужно войти в систему',
    'not enough permissions': 'Недостаточно прав',
    'club access required': 'Нет доступа к клубу',
    'PC is not available': 'Компьютер сейчас недоступен',
    'PC is occupied. Use session QR to extend': 'ПК занят. Для продления используйте QR на экране сессии.',
    'active session for extension not found': 'Активная сессия для продления не найдена',
    'voucher not found or expired': 'Ваучер не найден или истёк',
    'QR token not found': 'QR-код не найден',
    'voucher belongs to another club': 'Ваучер относится к другому клубу',
    'recipient consent is required': 'Нужно согласие клиента на хранение номера и отправку ваучера в Telegram.',
    'valid cash reason is required': 'Выберите причину наличной операции.',
  };
  return dictionary[message] || message;
}

function clubPath(path: string, clubID: string) {
  const separator = path.includes('?') ? '&' : '?';
  return `${path}${separator}club_id=${encodeURIComponent(clubID)}`;
}

function formatUZS(value: number) {
  return `${formatUZSInput(value)} сум`;
}

function formatUZSInput(value: number | string) {
  const raw = String(value).replace(/\D/g, '');
  if (!raw) return typeof value === 'number' ? '0' : '';
  return raw.replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
}

function parseUZSInput(value: string) {
  return Number(value.replace(/\D/g, '')) || 0;
}

function formatUzPhoneInput(value: string) {
  const digits = value.replace(/\D/g, '');
  const localDigits = (digits.startsWith('998') ? digits.slice(3) : digits).slice(0, 9);
  const parts = [
    localDigits.slice(0, 2),
    localDigits.slice(2, 5),
    localDigits.slice(5, 7),
    localDigits.slice(7, 9),
  ].filter(Boolean);
  return `+998${parts.length ? ` ${parts.join(' ')}` : ' '}`;
}

function formatPackageCount(count: number) {
  const mod10 = count % 10;
  const mod100 = count % 100;
  const word = mod10 === 1 && mod100 !== 11
    ? 'пакет'
    : mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)
      ? 'пакета'
      : 'пакетов';
  return `${count} ${word}`;
}

function formatDateTime(value: string) {
  return new Intl.DateTimeFormat('ru-UZ', { hour: '2-digit', minute: '2-digit', day: '2-digit', month: '2-digit' }).format(new Date(value));
}

function formatTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return new Intl.DateTimeFormat('ru-UZ', { hour: '2-digit', minute: '2-digit' }).format(date);
}

function slugifyClient(value: string) {
  const slug = value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return slug || 'club';
}

function remainingSecondsForPC(pc: PC, nowMs: number, fetchedAtMs: number) {
  if (pc.status !== 'occupied') return 0;
  const serverSeconds = Math.max(0, Math.floor(pc.remaining_seconds || 0));
  const elapsedSeconds = Math.max(0, Math.floor((nowMs - fetchedAtMs) / ADMIN_REFRESH_MS) * (ADMIN_REFRESH_MS / 1000));
  return Math.max(0, Math.floor(serverSeconds - elapsedSeconds));
}

function formatRemainingTime(seconds?: number) {
  const safeSeconds = Math.max(0, Math.floor(seconds || 0));
  const hours = Math.floor(safeSeconds / 3600);
  const minutes = Math.floor((safeSeconds % 3600) / 60);
  const restSeconds = safeSeconds % 60;
  return [hours, minutes, restSeconds].map((part) => String(part).padStart(2, '0')).join(':');
}

function formatDurationClock(seconds?: number) {
  const safeSeconds = Math.max(0, Math.floor(seconds || 0));
  const totalMinutes = Math.floor(safeSeconds / 60);
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;

  if (safeSeconds <= 24 * 60 * 60) {
    return [hours, minutes].map((part) => String(part).padStart(2, '0')).join(':');
  }

  const days = Math.floor(hours / 24);
  return [days, hours % 24, minutes].map((part) => String(part).padStart(2, '0')).join(':');
}

// The QR payment screen is currently Russian-only. Keep the profile balance
// self-explanatory even at a glance: 05ч:48м (or 01д:05ч:48м).
function formatPlayerBalanceDuration(seconds?: number) {
  const safeSeconds = Math.max(0, Math.floor(seconds || 0));
  const totalMinutes = Math.floor(safeSeconds / 60);
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;

  if (safeSeconds <= 24 * 60 * 60) {
    return `${String(hours).padStart(2, '0')}ч:${String(minutes).padStart(2, '0')}м`;
  }

  const days = Math.floor(hours / 24);
  return `${String(days).padStart(2, '0')}д:${String(hours % 24).padStart(2, '0')}ч:${String(minutes).padStart(2, '0')}м`;
}

function isPayableStatus(status: string) {
  return status === 'available' || status === 'sleeping';
}

function isSessionExtendableStatus(status: string) {
  return status === 'occupied' || status === 'frozen';
}

function pcStatusLabel(status: string) {
  const labels: Record<string, string> = {
    available: 'Свободен',
    sleeping: 'Спит',
    occupied: 'Занят',
    frozen: 'Время закончилось',
    maintenance: 'Ремонт',
    offline: 'Нет связи',
    blocked: 'Заблокирован',
  };
  return labels[status] || status;
}

function statusLabel(status: string) {
  const labels: Record<string, string> = {
    active: 'Активен',
    inactive: 'Отключен',
    maintenance: 'Ремонт',
    deleted: 'Удален',
  };
  return labels[status] || status;
}

function grantStatusLabel(status: string) {
  const labels: Record<string, string> = {
    pending: 'Ожидает запуска',
    accepted: 'Сессия запущена',
    ended: 'Завершена',
    start_failed: 'Ошибка запуска',
  };
  return labels[status] || status;
}

function orderStatusLabel(status: string) {
  const labels: Record<string, string> = {
    created: 'Создана',
    payment_pending: 'Ожидает оплаты',
    paid: 'Оплачено',
    failed: 'Ошибка оплаты',
    refunded: 'Возврат',
  };
  return labels[status] || status;
}

function providerLabel(provider?: string) {
  const labels: Record<string, string> = {
    click: 'Click',
    payme: 'Payme',
    mock: 'Тестовая оплата',
  };
  return labels[provider || ''] || provider || 'Оплата';
}

function sourceLabel(source: string) {
  const labels: Record<string, string> = {
    online_payment: 'Онлайн-оплата',
    cash: 'Наличные',
    voucher: 'Ваучер',
  };
  return labels[source] || source;
}

function fiscalStatusLabel(status?: string) {
  const labels: Record<string, string> = {
    not_requested: 'Чек не запрошен',
    pending: 'Чек в обработке',
    not_confirmed: 'Чек не подтверждён',
    confirmed: 'Чек подтверждён',
    failed: 'Ошибка чека',
  };
  return labels[status || ''] || 'Чек не подтверждён';
}

function telegramDeliverySuffix(status?: string) {
  if (!status) return '';
  const labels: Record<string, string> = {
    sent: ' · Ваучер отправлен в Telegram',
    telegram_waiting_for_user: ' · Для отправки ваучера привяжите номер к Telegram',
    telegram_not_configured: ' · Telegram не подключен к серверу, код покажите клиенту вручную',
    telegram_link_failed: ' · Ссылка на Telegram не создана',
    telegram_failed: ' · Ваучер не отправлен в Telegram',
  };
  return labels[status] || '';
}

function clubRole(auth: AuthPayload, clubID: string) {
  if (auth.user.global_role === 'super_admin') return 'super_admin';
  return auth.clubs.find((club) => club.id === clubID)?.role || '';
}

function canViewAdmin(auth: AuthPayload, clubID: string) {
  return ['super_admin', 'owner', 'admin', 'manager'].includes(clubRole(auth, clubID));
}

function canViewOwner(auth: AuthPayload, clubID: string) {
  return ['super_admin', 'owner'].includes(clubRole(auth, clubID));
}

function canViewSettings(auth: AuthPayload, clubID: string) {
  return ['super_admin', 'owner'].includes(clubRole(auth, clubID));
}

function roleLabel(role: string) {
  const labels: Record<string, string> = {
    super_admin: 'Суперадмин',
    owner: 'Владелец',
    admin: 'Менеджер',
    manager: 'Менеджер',
  };
  return labels[role] || role;
}

function roleOptions(canManageNetwork = false) {
  return canManageNetwork
    ? [
        { value: 'owner', label: 'Владелец' },
        { value: 'admin', label: 'Менеджер' },
      ]
    : [{ value: 'admin', label: 'Менеджер' }];
}

function statusOptions() {
  return [
    { value: 'active', label: 'Активен' },
    { value: 'inactive', label: 'Отключен' },
  ];
}

function zoneStatusOptions() {
  return [
    { value: 'active', label: 'Активна' },
    { value: 'maintenance', label: 'Ремонт' },
    { value: 'inactive', label: 'Отключена' },
  ];
}

createRoot(document.getElementById('root')!).render(<App />);
