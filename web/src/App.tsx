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
  KeyRound,
  LogOut,
  Monitor,
  Moon,
  Play,
  Plus,
  Power,
  QrCode,
  ReceiptText,
  RefreshCw,
  Save,
  Send,
  Settings,
  Ticket,
  Trash2,
  Users,
  Wrench,
  X,
} from 'lucide-react';
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

type PC = {
  id: string;
  external_pc_id: string;
  number: number;
  label: string;
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
};

type ClubSettings = {
  id: string;
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
    <main className="shell narrow auth-shell">
      <PageHeader eyebrow="Clubpay" title="Вход в панель" />
      <Panel>
        <form className="stack" onSubmit={submit}>
          <label className="form-block">
            Логин
            <input value={login} onChange={(event) => setLogin(event.target.value)} placeholder="email или телефон" />
          </label>
          <label className="form-block">
            Пароль
            <input value={password} onChange={(event) => setPassword(event.target.value)} type="password" />
          </label>
          <Button full icon={loading ? <RefreshCw className="spin" size={18} /> : <KeyRound size={18} />} disabled={loading}>
            Войти
          </Button>
          {message && <Notice tone="danger">{message}</Notice>}
        </form>
      </Panel>
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
    <main className="shell">
      <WorkspaceHeader {...props} eyebrow="Clubpay" title="Рабочая панель" />
      <Panel>
        <div className="link-grid">
          <LinkButton href="/qr/pc-001" icon={<Monitor size={18} />}>Открыть QR PC #01</LinkButton>
          <LinkButton href="/admin" icon={<Activity size={18} />}>Панель менеджера</LinkButton>
          {canOpenOwner && <LinkButton href="/reports" icon={<Banknote size={18} />}>Дашборд</LinkButton>}
          {canOpenSettings && <LinkButton href="/settings" icon={<Settings size={18} />}>Настройки клуба</LinkButton>}
        </div>
      </Panel>
    </main>
  );
}

function QRPage({ token }: { token: string }) {
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
  const [error, setError] = useState('');

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
  const canStartOrExtend = data ? isPayableStatus(data.pc.status) || (data.pc.status === 'occupied' && isSessionExtendQR) : false;
  const isExtension = data?.pc.status === 'occupied' && isSessionExtendQR;
  const isStaticBusyQR = data?.pc.status === 'occupied' && !isSessionExtendQR;
  const busyUntilLabel = data?.active_session?.planned_ends_at ? formatTime(data.active_session.planned_ends_at) : '';
  const voucherReadyForAutoApply = Boolean(voucherCode.trim() && voucherCheck?.can_redeem);
  const voucherDurationSeconds = voucherCheck?.seconds_left || (voucherCheck?.minutes_left ? voucherCheck.minutes_left * 60 : 0);

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

        <section className="qr-voucher-card">
          <div>
            <p>Есть остаток времени?</p>
            <h2>Проверьте ваучер</h2>
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
              <span>
                Можно нажать “Применить” сразу. Если выбрать пакет или ввести сумму и оплатить, ваучер применится автоматически и добавится к оплаченному времени.
              </span>
            </div>
          )}
          {voucherCheck && !voucherCheck.can_redeem && (
            <div className="qr-voucher-status danger">
              <strong>Ваучер найден, но применить здесь нельзя</strong>
              <span>Проверьте клуб, зону или статус компьютера.</span>
            </div>
          )}
          {checkingVoucher && <p className="qr-method-message">Проверяем ваучер...</p>}
        </section>

        {data.telegram?.bot_link && (
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
              setCustomAmount(event.target.value.replace(/[^\d ]/g, ''));
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
      </section>

      <div className="qr-checkout-bar">
        <div className="qr-checkout-inner">
          <Button full className="qr-pay-button" disabled={isBusy || checkingVoucher || (!selected && !customAmountUZS) || !checkoutAmountUZS || creating || !paymentMethodReady} icon={creating ? <RefreshCw className="spin" size={18} /> : <CreditCard size={18} />} onClick={createCheckout}>
            {creating ? 'Открываем оплату' : payLabel}
          </Button>
        </div>
      </div>
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
  const [endPhoneByGrant, setEndPhoneByGrant] = useState<Record<string, string>>({});
  const [telegramPrompt, setTelegramPrompt] = useState<TelegramPrompt | null>(null);
  const [message, setMessage] = useState('');
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
        reason: 'cash',
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

  async function endGrant(grant: Grant) {
    setTelegramPrompt(null);
    const result = await api<{ voucher?: { code: string; minutes_left: number; seconds_left?: number }; voucher_delivery?: VoucherDelivery }>(`/api/admin/grants/${grant.id}/end`, {
      method: 'POST',
      body: JSON.stringify({ reason: 'admin_request', recipient_phone: endPhoneByGrant[grant.id] || '' }),
    });
    if (result.voucher_delivery?.telegram_link) {
      setTelegramPrompt({
        link: result.voucher_delivery.telegram_link,
        phone: result.voucher_delivery.phone,
        code: result.voucher?.code,
        minutes: result.voucher?.minutes_left,
        seconds: result.voucher?.seconds_left,
      });
    }
    const deliverySuffix = result.voucher_delivery?.status ? ` · Telegram: ${telegramDeliveryLabel(result.voucher_delivery.status)}` : '';
    setMessage(result.voucher ? `Сессия завершена. Ваучер: ${result.voucher.code}${deliverySuffix}` : 'Сессия завершена без ваучера');
    setEndPhoneByGrant((current) => {
      const next = { ...current };
      delete next[grant.id];
      return next;
    });
    refresh();
  }

  async function syncOrder(order: Order) {
    await api(`/api/payments/sync/${order.invoice_id}`, { method: 'POST' });
    refresh();
  }

  async function testPayOrder(order: Order) {
    await api(`/api/payments/mock/success/${order.invoice_id}`, { method: 'POST' });
    refresh();
  }

  return (
    <main className="shell">
      <WorkspaceHeader auth={auth} selectedClubID={selectedClubID} currentPath={currentPath} onClubChange={onClubChange} onLogout={onLogout} eyebrow="Панель менеджера" title="Зал, оплаты и сессии" />

      {message && <Notice tone="success">{message}</Notice>}
      {telegramPrompt && <TelegramVoucherModal prompt={telegramPrompt} onClose={() => setTelegramPrompt(null)} onCopied={() => setMessage('Ссылка Telegram скопирована')} />}

      <Panel className="pc-table-panel">
        <div className="table-wrap">
          <table className="pc-table">
            <thead>
              <tr>
                <th>ПК</th>
                <th>Зона</th>
                <th>Статус</th>
                <th>Осталось</th>
                <th className="actions-col">Действия</th>
              </tr>
            </thead>
            <tbody>
              {catalog.pcs.map((pc) => {
                const remainingSeconds = remainingSecondsForPC(pc, nowMs, catalogFetchedAtMs);
                return (
                  <tr key={pc.id}>
                    <td>
                      <div className="pc-cell">
                        <Monitor size={18} />
                        <div>
                          <strong>{pc.label}</strong>
                          <span>{pc.external_pc_id}</span>
                        </div>
                      </div>
                    </td>
                    <td>{pc.zone}</td>
                    <td><StatusBadge status={pc.status} /></td>
                    <td>
                      <span className={`remaining ${pc.status === 'occupied' ? 'active' : ''}`}>
                        {pc.status === 'occupied' ? formatRemainingTime(remainingSeconds) : '—'}
                      </span>
                    </td>
                    <td>
                      <div className="table-actions">
                        <Button size="sm" variant="ghost" icon={<Power size={13} />} onClick={() => setPCStatus(pc.id, 'available')}>Свободен</Button>
                        <Button size="sm" variant="ghost" icon={<Moon size={13} />} onClick={() => setPCStatus(pc.id, 'sleeping')}>Сон</Button>
                        <Button size="sm" variant="danger" icon={<Wrench size={13} />} onClick={() => setPCStatus(pc.id, 'maintenance')}>Ремонт</Button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </Panel>

      <section className="split">
        <Panel className="stack">
          <SectionTitle icon={<Banknote size={18} />} title="Наличная оплата" caption="Ручной запуск, когда онлайн невозможен" />
          <div className="form-grid">
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
                  setCashAmount(event.target.value.replace(/[^\d ]/g, ''));
                  setCashTariffID('');
                }}
                placeholder={selectedCashZone ? `1 час: ${formatUZS(selectedCashZone.hourly_price_uzs)}` : 'Например: 20 000'}
                inputMode="numeric"
              />
            </label>
          </div>
          <Button full icon={<Banknote size={18} />} onClick={startCashSession} disabled={!cashPCID || (!cashTariffID && !cashAmountUZS)}>
            {cashButtonAmount ? `Запустить на ${formatUZS(cashButtonAmount)}` : 'Запустить наличную сессию'}
          </Button>
        </Panel>

        <Panel className="scroll-panel">
          <SectionTitle icon={<Gamepad2 size={18} />} title="Игровые сессии" caption="То, что уже отправлено в систему управления ПК" />
          <div className="data-list scroll-list">
            {grants.length === 0 && <EmptyState text="Активных и завершённых сессий пока нет" />}
            {grants.map((grant) => (
              <div className="list-row" key={grant.id}>
                <div>
                  <strong>{grant.pc_label} · {formatDurationClock(grant.duration_seconds || grant.duration_minutes * 60)}</strong>
                  <span>Источник: {sourceLabel(grant.source)} · {grantStatusLabel(grant.status)}</span>
                  {grant.planned_ends_at && <small>Плановое окончание: {formatDateTime(grant.planned_ends_at)}</small>}
                  {grant.last_error && <small className="danger-text">{grant.last_error}</small>}
                </div>
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
                    <Button size="sm" variant="secondary" icon={<Power size={13} />} onClick={() => endGrant(grant)}>Завершить</Button>
                  </div>
                )}
              </div>
            ))}
          </div>
        </Panel>
      </section>

      <Panel className="orders-panel scroll-panel">
        <SectionTitle icon={<ReceiptText size={18} />} title="Оплаты" caption="Онлайн-платежи Click/Payme, тестовые оплаты и статус фискализации" />
        <div className="data-list scroll-list">
          {orders.length === 0 && <EmptyState text="Оплат пока нет" />}
          {orders.map((order) => (
            <div className="list-row" key={order.id}>
              <div>
                <strong>{order.pc_label} · {order.tariff}</strong>
                <span>{providerLabel(order.provider)} · {formatUZS(order.amount_uzs)} · {orderStatusLabel(order.status)} · {fiscalStatusLabel(order.fiscal_status)}</span>
                <small>{order.invoice_id}</small>
              </div>
              <div className="button-row">
                {order.status !== 'paid' && (
                  <Button size="sm" variant="ghost" icon={<RefreshCw size={13} />} onClick={() => syncOrder(order)}>Обновить</Button>
                )}
                {order.status !== 'paid' && (
                  <Button size="sm" variant="success" icon={<Play size={13} />} onClick={() => testPayOrder(order)}>Тестовая оплата</Button>
                )}
              </div>
            </div>
          ))}
        </div>
      </Panel>
    </main>
  );
}

function ReportsPage({ auth, selectedClubID, currentPath, onClubChange, onLogout }: WorkspaceProps) {
  const [summary, setSummary] = useState<Summary | null>(null);
  const refreshingRef = useRef(false);

  useEffect(() => {
    async function refreshSummary() {
      if (!selectedClubID || refreshingRef.current) return;
      refreshingRef.current = true;
      try {
        setSummary(await api<Summary>(clubPath('/api/owner/summary', selectedClubID)));
      } finally {
        refreshingRef.current = false;
      }
    }

    refreshSummary();
    const timer = window.setInterval(refreshSummary, OWNER_REFRESH_MS);
    return () => window.clearInterval(timer);
  }, [selectedClubID]);

  return (
    <main className="shell">
      <WorkspaceHeader auth={auth} selectedClubID={selectedClubID} currentPath={currentPath} onClubChange={onClubChange} onLogout={onLogout} eyebrow="Дашборд" title="Финансы и загрузка клуба" />
      {!summary ? (
        <Panel><EmptyState text="Считаем сводку" /></Panel>
      ) : (
        <section className="metrics">
          <Metric icon={<CreditCard />} label="Онлайн на кассу" value={formatUZS(summary.club_online_revenue_uzs || summary.online_revenue_uzs)} />
          <Metric icon={<Banknote />} label="Наличные" value={formatUZS(summary.cash_revenue_uzs)} />
          <Metric icon={<Activity />} label="Итого клубу" value={formatUZS(summary.club_total_revenue_uzs)} />
          <Metric icon={<ReceiptText />} label="Онлайн-оплаты" value={String(summary.paid_orders)} />
          <Metric icon={<Banknote />} label="Наличные сессии" value={String(summary.cash_sessions)} />
          <Metric icon={<Monitor />} label="Активные сессии" value={String(summary.active_grants)} />
        </section>
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
  };
}

function defaultUserForm(): Partial<ClubUser> & { password?: string } {
  return { name: '', email: '', phone: '', role: 'admin', status: 'active', password: '' };
}

function SettingsPage({ auth, selectedClubID, currentPath, onClubChange, onLogout, onReloadAuth }: WorkspaceProps & { onReloadAuth: () => void | Promise<void> }) {
  const canManageNetwork = auth.user.global_role === 'super_admin';
  const [settings, setSettings] = useState<ClubSettingsPayload | null>(null);
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
      setTariffZoneFilter((current) => current && payload.zones.some((zone) => zone.id === current) ? current : payload.zones[0]?.id || '');
      setPCZoneFilter((current) => current && payload.zones.some((zone) => zone.id === current) ? current : '');
      setError('');
    } catch (err) {
      setError(String((err as Error).message || err));
    }
  }

  useEffect(() => {
    setSettings(null);
    loadSettings();
  }, [selectedClubID]);

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
  }

  function startNewClub() {
    setCreatingClub(true);
    setSettings({ club: { ...EMPTY_CLUB_FORM }, zones: [], tariffs: [], pcs: [], users: [] });
    setClubForm({ ...EMPTY_CLUB_FORM });
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

  async function saveUser() {
    const path = userForm.id ? `/api/backoffice/users/${userForm.id}/clubs/${selectedClubID}` : `/api/backoffice/clubs/${selectedClubID}/users`;
    const payload = { ...userForm, role: canManageNetwork && userForm.role === 'owner' ? 'owner' : 'admin' };
    await persistSettings(
      () => api(path, { method: 'POST', body: JSON.stringify(payload) }),
      userForm.id ? 'Доступ пользователя обновлён' : 'Пользователь добавлен',
    );
  }

  async function deleteUser() {
    if (!userForm.id || !window.confirm(`Удалить доступ пользователя "${userForm.name}" к этому клубу?`)) return;
    await persistSettings(
      () => api(`/api/backoffice/users/${userForm.id}/clubs/${selectedClubID}`, { method: 'DELETE' }),
      'Доступ пользователя удалён',
    );
  }

  const settingsSection = currentPath.startsWith('/settings/zones')
    ? 'zones'
    : currentPath.startsWith('/settings/pcs')
      ? 'pcs'
      : currentPath.startsWith('/settings/users')
        ? 'users'
        : 'club';
  const settingsTitle = {
    club: 'Клуб',
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

  return (
    <main className="shell">
      <WorkspaceHeader
        auth={auth}
        selectedClubID={selectedClubID}
        currentPath={currentPath}
        onClubChange={onClubChange}
        onLogout={onLogout}
        eyebrow="Настройки"
        title={settingsTitle}
        clubAction={canManageNetwork ? <Button size="sm" variant="secondary" icon={<Plus size={13} />} onClick={startNewClub}>Новый клуб</Button> : undefined}
      />
      {message && <Notice tone="success">{message}</Notice>}
      {error && <Notice tone="danger">{error}</Notice>}
      {!creatingClub && clubForm?.id && (
        <div className="settings-subnav">
          <LinkButton href="/settings" variant={settingsSection === 'club' ? 'secondary' : 'ghost'} icon={<Building2 size={16} />}>Клуб</LinkButton>
          <LinkButton href="/settings/zones" variant={settingsSection === 'zones' ? 'secondary' : 'ghost'} icon={<ReceiptText size={16} />}>Зоны и пакеты</LinkButton>
          <LinkButton href="/settings/pcs" variant={settingsSection === 'pcs' ? 'secondary' : 'ghost'} icon={<Monitor size={16} />}>Компьютеры</LinkButton>
          <LinkButton href="/settings/users" variant={settingsSection === 'users' ? 'secondary' : 'ghost'} icon={<Users size={16} />}>Доступы</LinkButton>
        </div>
      )}
      {!settings || !clubForm ? (
        <Panel><EmptyState text="Загружаем настройки" /></Panel>
      ) : (
        <section className="settings-grid">
          {settingsSection === 'club' && (
          <Panel className="stack settings-wide">
            <SectionTitle
              icon={<Building2 size={18} />}
              title={creatingClub ? 'Новый клуб' : 'Клуб'}
              caption={canManageNetwork ? 'Основные данные клуба, прямые подключения Click/Payme и опциональные данные для чеков.' : 'Основные данные клуба и состояние платежного подключения.'}
            />
            <div className="form-mode">
              <strong>{creatingClub ? 'Создание клуба' : 'Редактирование клуба'}</strong>
              <span>{creatingClub ? 'Заполните название и платежные настройки. Зоны, пакеты и ПК добавите после сохранения.' : 'Изменения применятся к выбранному клубу.'}</span>
            </div>
            <div className="form-grid two">
              <Field label="Название клуба" value={clubForm.name} onChange={updateClubName} help="Так клуб будет называться в панели и на QR-странице." />
              <Field label="Юр. название" value={clubForm.legal_name} onChange={(value) => setClubForm({ ...clubForm, legal_name: value })} help="Официальное название юрлица для договора и чеков." />
              <Field label="ИНН" value={clubForm.tin} onChange={(value) => setClubForm({ ...clubForm, tin: value })} help="Налоговый номер клуба." />
              <Field label="Адрес" value={clubForm.address} onChange={(value) => setClubForm({ ...clubForm, address: value })} help="Адрес клуба или юрлица." />
              {canManageNetwork && (
                <>
                  <Field label="Click merchant ID" value={clubForm.click_merchant_id} onChange={(value) => setClubForm({ ...clubForm, click_merchant_id: value })} help="Обязательный ID поставщика Click для платежной ссылки." />
                  <Field label="Click service ID" value={clubForm.click_service_id} onChange={(value) => setClubForm({ ...clubForm, click_service_id: value })} help="Обязательный ID сервиса Click, по нему создается платежная ссылка." />
                  <Field label="Click merchant user ID" value={clubForm.click_merchant_user_id} onChange={(value) => setClubForm({ ...clubForm, click_merchant_user_id: value })} help="ID пользователя мерчанта Click, который выдают вместе с service ID." />
                  <Field label="Click secret key" type="password" value={clubForm.click_secret_key} onChange={(value) => setClubForm({ ...clubForm, click_secret_key: value })} help="Секрет для проверки Prepare/Complete callback." />
                  <Field label="Payme merchant ID" value={clubForm.payme_merchant_id} onChange={(value) => setClubForm({ ...clubForm, payme_merchant_id: value })} help="ID кассы/мерчанта Payme для checkout-ссылки." />
                  <Field label="Payme secret key" type="password" value={clubForm.payme_secret_key} onChange={(value) => setClubForm({ ...clubForm, payme_secret_key: value })} help="Для sandbox укажите TEST_KEY из Payme Business; для production — боевой secret." />
                  <SelectField label="Статус клуба" value={clubForm.status} options={statusOptions()} onChange={(value) => setClubForm({ ...clubForm, status: value })} help="Отключенный клуб не должен принимать новые оплаты." />
                  <Field label="Название услуги для чека" value={clubForm.ofd_service_name} onChange={(value) => setClubForm({ ...clubForm, ofd_service_name: value })} help="Например: Компьютерное время. Можно оставить по умолчанию." />
                  <Field label="ИКПУ / MXIK" value={clubForm.ofd_mxik} onChange={(value) => setClubForm({ ...clubForm, ofd_mxik: value })} help="Заполняем после бухгалтера. Если пусто, Payme уйдет без detail.items и сможет работать со статичной кассой." />
                  <Field label="package_code" value={clubForm.ofd_package_code} onChange={(value) => setClubForm({ ...clubForm, ofd_package_code: value })} help="Заполняем после бухгалтера вместе с IKPU/MXIK." />
                  <Field label="unit_code" value={clubForm.ofd_unit_code} onChange={(value) => setClubForm({ ...clubForm, ofd_unit_code: value })} help="Опционально: код единицы измерения, если его требует платежка или OFD." />
                  <Field label="НДС, %" type="number" min="0" value={String(clubForm.ofd_vat_percent ?? 0)} onChange={(value) => setClubForm({ ...clubForm, ofd_vat_percent: Number(value || 0) })} help="Заполняем вместе с кодами. 0 — без НДС или ставка не применяется." />
                </>
              )}
            </div>
            {!canManageNetwork && <ClubConnectionSummary club={clubForm} />}
            <div className="button-row">
              {creatingClub && selectedClubID && <Button variant="ghost" onClick={loadSettings}>Отменить</Button>}
              {!creatingClub && canManageNetwork && <Button variant="danger" icon={<Trash2 size={16} />} onClick={deleteClub}>Удалить клуб</Button>}
              <Button icon={<Save size={16} />} onClick={saveClub}>{creatingClub ? 'Создать клуб' : 'Сохранить клуб'}</Button>
            </div>
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
              <>
                <div className="form-mode">
                  <strong>{zoneForm.id ? 'Редактирование зоны' : 'Новая зона'}</strong>
                  <span>Клик по зоне выше открывает редактирование. Для создания нажмите “Новая зона”.</span>
                </div>
                <div className="form-grid">
                  <Field label="Название зоны" value={zoneForm.name || ''} onChange={(value) => setZoneForm({ ...zoneForm, name: value })} />
                  <Field label="Стоимость 1 часа, сум" type="number" value={String(zoneForm.hourly_price_uzs || 0)} onChange={(value) => setZoneForm({ ...zoneForm, hourly_price_uzs: Number(value || 0) })} help="Используется, когда клиент или менеджер вводит сумму вручную без пакета." />
                  <SelectField label="Статус" value={zoneForm.status || 'active'} options={zoneStatusOptions()} onChange={(value) => setZoneForm({ ...zoneForm, status: value })} />
                </div>
                <div className="button-row">
                  {zoneForm.id && <Button variant="danger" icon={<Trash2 size={16} />} onClick={deleteZone}>Удалить</Button>}
                  <Button icon={<Save size={16} />} onClick={saveZone}>{zoneForm.id ? 'Сохранить зону' : 'Добавить зону'}</Button>
                </div>
              </>
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
              <>
                <div className="form-mode">
                  <strong>{tariffForm.id ? 'Редактирование пакета' : 'Новый пакет'}</strong>
                  <span>Если хотите добавить пакет, сначала нажмите “Новый пакет”.</span>
                </div>
                <div className="form-grid two">
                  <SelectField label="Зона" value={tariffForm.zone_id || ''} options={settings.zones.map((zone) => ({ value: zone.id, label: zone.name }))} onChange={(value) => setTariffForm({ ...tariffForm, zone_id: value })} />
                  <Field label="Название" value={tariffForm.name || ''} onChange={(value) => setTariffForm({ ...tariffForm, name: value })} />
                  <Field label="Минуты" type="number" value={String(tariffForm.duration_minutes || 0)} onChange={(value) => setTariffForm({ ...tariffForm, duration_minutes: Number(value || 0) })} />
                  <Field label="Цена, сум" type="number" value={String(tariffForm.price_uzs || 0)} onChange={(value) => setTariffForm({ ...tariffForm, price_uzs: Number(value || 0) })} />
                  <SelectField label="Статус" value={tariffForm.status || 'active'} options={statusOptions()} onChange={(value) => setTariffForm({ ...tariffForm, status: value })} />
                </div>
                <div className="button-row">
                  {tariffForm.id && <Button variant="danger" icon={<Trash2 size={16} />} onClick={deleteTariff}>Удалить</Button>}
                  <Button icon={<Save size={16} />} onClick={saveTariff}>{tariffForm.id ? 'Сохранить пакет' : 'Добавить пакет'}</Button>
                </div>
              </>
            )}
          </Panel>
            </>
          )}

          {!creatingClub && settingsSection === 'pcs' && (
          <Panel className="stack settings-wide">
            <SectionTitle icon={<Monitor size={18} />} title="Компьютеры" caption="Добавьте компьютеры клуба. QR создается автоматически после сохранения ПК." />
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
                      <td>{pc.qr_url ? <AppLink className="text-link" href={pc.qr_url}>{pc.qr_token}</AppLink> : '—'}</td>
                      <td><Button size="sm" variant="ghost" onClick={() => { setPCForm(pc); setShowPCForm(true); }}>Изменить</Button></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="button-row">
              <Button size="sm" variant="secondary" icon={<Plus size={13} />} onClick={resetPC}>Новый ПК</Button>
            </div>
            {showPCForm && (
              <>
                <div className="form-mode">
                  <strong>{pcForm.id ? 'Редактирование ПК' : 'Новый ПК'}</strong>
                  <span>QR и системный ID создаются автоматически после сохранения.</span>
                </div>
                <div className="form-grid two">
                  <SelectField label="Зона" value={pcForm.zone_id || ''} options={settings.zones.map((zone) => ({ value: zone.id, label: zone.name }))} onChange={(value) => setPCForm({ ...pcForm, zone_id: value })} />
                  <Field label="Номер" type="number" value={String(pcForm.number || 0)} onChange={(value) => setPCForm({ ...pcForm, number: Number(value || 0) })} />
                  <Field label="Название" value={pcForm.label || ''} onChange={updatePCLabel} />
                </div>
                <div className="button-row">
                  {pcForm.id && <Button variant="danger" icon={<Trash2 size={16} />} onClick={deletePC}>Удалить</Button>}
                  <Button icon={<Save size={16} />} onClick={savePC}>{pcForm.id ? 'Сохранить ПК' : 'Добавить ПК'}</Button>
                </div>
              </>
            )}
          </Panel>
          )}

          {!creatingClub && settingsSection === 'users' && (
          <Panel className="stack settings-wide">
            <SectionTitle icon={<Users size={18} />} title="Доступы" caption="Кто может заходить в панель выбранного клуба." />
            <div className="compact-list">
              {visibleUsers.map((user) => (
                <button className="editable-row" key={user.id} onClick={() => setUserForm({ ...user, role: canManageNetwork && user.role === 'owner' ? 'owner' : 'admin', password: '' })}>
                  <span>
                    <strong>{user.name} · {roleLabel(user.role)}</strong>
                    <small>{user.email || user.phone} · {statusLabel(user.status)}</small>
                  </span>
                  <em>Изменить</em>
                </button>
              ))}
              {visibleUsers.length === 0 && <EmptyState text="Менеджеры клуба пока не добавлены" />}
            </div>
            <div className="button-row">
              <Button size="sm" variant="secondary" icon={<Plus size={13} />} onClick={resetUser}>{canManageNetwork ? 'Новый пользователь' : 'Новый менеджер'}</Button>
            </div>
            <div className="form-mode">
              <strong>{userForm.id ? 'Редактирование доступа' : 'Новый пользователь'}</strong>
              <span>{canManageNetwork ? 'Суперадмин может добавить владельца клуба или менеджера.' : 'Владелец может добавлять только менеджеров. Владельцев добавляет команда Clubpay.'}</span>
            </div>
            <div className="form-grid two">
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
          </Panel>
          )}
        </section>
      )}
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

function WorkspaceHeader({ auth, selectedClubID, currentPath, onClubChange, onLogout, eyebrow, title, clubAction }: WorkspaceProps & { eyebrow: string; title: string; clubAction?: React.ReactNode }) {
  const canOpenOwner = canViewOwner(auth, selectedClubID);
  const canOpenSettings = canViewSettings(auth, selectedClubID);
  const selectedClub = auth.clubs.find((club) => club.id === selectedClubID) || auth.clubs[0];

  return (
    <header className="topbar">
      <div>
        <p className="eyebrow">{eyebrow}</p>
        <h1>{title}</h1>
      </div>
      <div className="workspace-actions">
        {auth.clubs.length > 1 ? (
          <select value={selectedClubID} onChange={(event) => onClubChange(event.target.value)} aria-label="Клуб">
            {auth.clubs.map((club) => <option key={club.id} value={club.id}>{club.name} · {roleLabel(club.role)}</option>)}
          </select>
        ) : (
          <span className="club-static">{selectedClub ? `${selectedClub.name} · ${roleLabel(selectedClub.role)}` : 'Клуб не выбран'}</span>
        )}
        {clubAction}
        {canOpenOwner && <AppLink className={`btn ghost sm ${currentPath.startsWith('/reports') || currentPath.startsWith('/owner') ? 'active' : ''}`} href="/reports"><Banknote size={13} /><span>Дашборд</span></AppLink>}
        {canOpenSettings && <AppLink className={`btn ghost sm ${currentPath.startsWith('/settings') ? 'active' : ''}`} href="/settings"><Settings size={13} /><span>Настройки</span></AppLink>}
        <Button size="sm" variant="secondary" icon={<LogOut size={13} />} onClick={onLogout}>Выйти</Button>
      </div>
    </header>
  );
}

function PageHeader({ eyebrow, title, aside }: { eyebrow: string; title: string; aside?: React.ReactNode }) {
  return (
    <header className="topbar">
      <div>
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

function TelegramVoucherModal({ prompt, onClose, onCopied }: { prompt: TelegramPrompt; onClose: () => void; onCopied: () => void }) {
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
            <h2 id="telegram-modal-title">Покажите QR клиенту</h2>
          </div>
        </div>
        <QRCodeImage value={prompt.link} size={220} label="QR для получения ваучера в Telegram" />
        <p className="telegram-modal-text">
          После нажатия Start бот привяжет номер{prompt.phone ? ` ${prompt.phone}` : ''} и отправит ваучер
          {prompt.code ? ` ${prompt.code}` : ''}{prompt.seconds || prompt.minutes ? ` на ${formatDurationClock(prompt.seconds || (prompt.minutes || 0) * 60)}` : ''}.
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

function SelectField({ label, value, options, onChange, help }: { label: string; value: string; options: Array<{ value: string; label: string }>; onChange: (value: string) => void; help?: string }) {
  return (
    <label>
      {label}
      <select value={value} onChange={(event) => onChange(event.target.value)}>
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
  };
  return dictionary[message] || message;
}

function clubPath(path: string, clubID: string) {
  const separator = path.includes('?') ? '&' : '?';
  return `${path}${separator}club_id=${encodeURIComponent(clubID)}`;
}

function formatUZS(value: number) {
  return new Intl.NumberFormat('ru-UZ').format(value) + ' сум';
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
  return formatRemainingTime(seconds);
}

function isPayableStatus(status: string) {
  return status === 'available' || status === 'sleeping';
}

function pcStatusLabel(status: string) {
  const labels: Record<string, string> = {
    available: 'Свободен',
    sleeping: 'Спит',
    occupied: 'Занят',
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

function telegramDeliveryLabel(status: string) {
  const labels: Record<string, string> = {
    sent: 'отправлено',
    telegram_waiting_for_user: 'ждём привязку номера',
    telegram_not_configured: 'бот не настроен',
    telegram_link_failed: 'ссылка не создана',
    telegram_failed: 'ошибка отправки',
  };
  return labels[status] || status;
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
