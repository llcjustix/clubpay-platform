import React, { useEffect, useMemo, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import {
  Activity,
  AlertCircle,
  Banknote,
  Building2,
  CheckCircle2,
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
  ReceiptText,
  RefreshCw,
  Save,
  Settings,
  Square,
  Ticket,
  Trash2,
  Users,
  Wrench,
} from 'lucide-react';
import './styles.css';

const API_BASE = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';
const IS_DEV = import.meta.env.DEV;
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
  club: { id: string; name: string };
  pc: { id: string; external_pc_id: string; number: number; label: string; status: string };
  zone: { id: string; name: string; hourly_price_tiyin: number; hourly_price_uzs: number };
  tariffs: Tariff[];
  payment_providers: QRPaymentProvider[];
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
  click_secret_key: string;
  payme_merchant_id: string;
  payme_secret_key: string;
  platform_fee_bps: number;
  ofd_mxik: string;
  ofd_package_code: string;
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

type Grant = {
  id: string;
  duration_minutes: number;
  status: string;
  source: string;
  pc_label: string;
  core_session_id?: string;
  planned_ends_at?: string;
  remaining_minutes: number;
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
  click_secret_key: '',
  payme_merchant_id: '',
  payme_secret_key: '',
  platform_fee_bps: 0,
  ofd_mxik: '',
  ofd_package_code: '',
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
          <LinkButton href="/admin" icon={<Activity size={18} />}>Панель администратора</LinkButton>
          {canOpenOwner && <LinkButton href="/reports" icon={<Banknote size={18} />}>Отчёт</LinkButton>}
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
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [redeeming, setRedeeming] = useState(false);
  const [error, setError] = useState('');

  function loadQR() {
    setLoading(true);
    api<QRData>(`/api/qr/${token}`)
      .then((payload) => {
        setData(payload);
        setSelected(payload.tariffs[0]?.id || '');
        const firstReadyProvider = payload.payment_providers.find((provider) => provider.configured)?.provider;
        setPaymentProvider(firstReadyProvider || (IS_DEV ? 'mock' : 'payme'));
      })
      .catch((err) => setError(String(err.message || err)))
      .finally(() => setLoading(false));
  }

  useEffect(loadQR, [token]);

  const selectedTariff = useMemo(() => data?.tariffs.find((tariff) => tariff.id === selected), [data, selected]);
  const providerOptions = useMemo(() => {
    const options = data?.payment_providers || [];
    return [
      ...options,
      ...(IS_DEV ? [{ provider: 'mock' as PaymentProvider, label: 'Тест', configured: true, sandbox: true, message: 'Локальная тестовая оплата без внешнего провайдера' }] : []),
    ];
  }, [data]);
  const selectedProviderOption = providerOptions.find((provider) => provider.provider === paymentProvider);
  const paymentMethodReady = Boolean(selectedProviderOption?.configured);
  const customAmountUZS = parseUZSInput(customAmount);
  const checkoutAmountUZS = customAmountUZS || selectedTariff?.price_uzs || 0;

  async function createCheckout() {
    if (!selected && !customAmountUZS) return;
    if (!paymentMethodReady) {
      setError(selectedProviderOption?.message || 'Способ оплаты не настроен');
      return;
    }
    setCreating(true);
    setError('');
    try {
      const payload = await api<{ checkout_url: string }>('/api/checkouts', {
        method: 'POST',
        body: JSON.stringify({
          qr_token: token,
          payment_provider: paymentProvider,
          ...(!customAmountUZS && selected ? { tariff_block_id: selected } : {}),
          ...(customAmountUZS ? { amount_uzs: customAmountUZS } : {}),
        }),
      });
      window.location.href = payload.checkout_url;
    } catch (err) {
      setError(String((err as Error).message || err));
    } finally {
      setCreating(false);
    }
  }

  async function redeemVoucher() {
    if (!voucherCode.trim()) return;
    setRedeeming(true);
    setVoucherMessage('');
    setError('');
    try {
      const payload = await api<{ grant_id: string; minutes_left: number }>('/api/vouchers/redeem', {
        method: 'POST',
        body: JSON.stringify({ code: voucherCode.trim(), qr_token: token }),
      });
      setVoucherMessage(`Ваучер применён: ${payload.minutes_left} мин`);
      loadQR();
    } catch (err) {
      setError(String((err as Error).message || err));
    } finally {
      setRedeeming(false);
    }
  }

  if (loading) return <Centered text="Загружаем компьютер" />;
  if (error && !data) return <Centered text={error} />;
  if (!data) return <Centered text="Компьютер не найден" />;

  const isBusy = !isPayableStatus(data.pc.status);
  const payLabel = isBusy
    ? 'Компьютер занят'
    : checkoutAmountUZS
      ? `Оплатить ${formatUZS(checkoutAmountUZS)}`
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
            <p>{isBusy ? 'Этот компьютер сейчас нельзя оплатить по QR.' : `Пакет или своя сумма. 1 час: ${formatUZS(data.zone.hourly_price_uzs)}.`}</p>
          </div>
        </section>

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

        <section className="qr-voucher-card">
          <div>
            <p>Есть остаток времени?</p>
            <h2>Примените ваучер</h2>
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
            <Button variant="secondary" disabled={isBusy || redeeming || !voucherCode.trim()} icon={<Ticket size={16} />} onClick={redeemVoucher}>
              {redeeming ? 'Проверяем' : 'Применить'}
            </Button>
          </div>
        </section>

        {voucherMessage && <Notice tone="success">{voucherMessage}</Notice>}
        {error && <Notice tone="danger">{error}</Notice>}
      </section>

      <div className="qr-checkout-bar">
        <div className="qr-checkout-inner">
          <Button full className="qr-pay-button" disabled={isBusy || (!selected && !customAmountUZS) || !checkoutAmountUZS || creating || !paymentMethodReady} icon={creating ? <RefreshCw className="spin" size={18} /> : <CreditCard size={18} />} onClick={createCheckout}>
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
  const [endMinutesByGrant, setEndMinutesByGrant] = useState<Record<string, string>>({});
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
    const remaining = Number.parseInt(endMinutesByGrant[grant.id] || '0', 10) || 0;
    const result = await api<{ voucher?: { code: string; minutes_left: number } }>(`/api/admin/grants/${grant.id}/end`, {
      method: 'POST',
      body: JSON.stringify({ reason: 'admin_request', remaining_minutes: remaining }),
    });
    setMessage(result.voucher ? `Сессия завершена. Ваучер: ${result.voucher.code}` : 'Сессия завершена без ваучера');
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
      <WorkspaceHeader auth={auth} selectedClubID={selectedClubID} currentPath={currentPath} onClubChange={onClubChange} onLogout={onLogout} eyebrow="Панель администратора" title="Зал, оплаты и сессии" />

      {message && <Notice tone="success">{message}</Notice>}

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
                  <strong>{grant.pc_label} · {grant.duration_minutes} мин</strong>
                  <span>Источник: {sourceLabel(grant.source)} · {grantStatusLabel(grant.status)}</span>
                  {grant.planned_ends_at && <small>Плановое окончание: {formatDateTime(grant.planned_ends_at)}</small>}
                  {grant.last_error && <small className="danger-text">{grant.last_error}</small>}
                </div>
                {grant.status === 'accepted' && (
                  <div className="end-session">
                    <input
                      aria-label="Остаток минут"
                      inputMode="numeric"
                      value={endMinutesByGrant[grant.id] || ''}
                      onChange={(event) => setEndMinutesByGrant((current) => ({ ...current, [grant.id]: event.target.value }))}
                      placeholder="0 мин"
                    />
                    <Button size="sm" variant="secondary" icon={<Square size={13} />} onClick={() => endGrant(grant)}>Завершить</Button>
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
      <WorkspaceHeader auth={auth} selectedClubID={selectedClubID} currentPath={currentPath} onClubChange={onClubChange} onLogout={onLogout} eyebrow="Отчёт владельца" title="Финансовый отчёт" />
      {!summary ? (
        <Panel><EmptyState text="Считаем сводку" /></Panel>
      ) : (
        <section className="metrics">
          <Metric icon={<CreditCard />} label="Онлайн-выручка" value={formatUZS(summary.online_revenue_uzs)} />
          <Metric icon={<Banknote />} label="Клубу онлайн" value={formatUZS(summary.club_online_revenue_uzs)} />
          <Metric icon={<ReceiptText />} label="Комиссия платформы" value={formatUZS(summary.platform_fee_uzs)} />
          <Metric icon={<Banknote />} label="Наличные" value={formatUZS(summary.cash_revenue_uzs)} />
          <Metric icon={<Activity />} label="Итого клубу" value={formatUZS(summary.club_total_revenue_uzs)} />
          <Metric icon={<ReceiptText />} label="Онлайн-оплаты" value={String(summary.paid_orders)} />
          <Metric icon={<Monitor />} label="Активные сессии" value={String(summary.active_grants)} />
        </section>
      )}
    </main>
  );
}

function defaultZoneForm(zones: Zone[] = []): Partial<Zone> {
  return { name: '', hourly_price_uzs: 15000, sort_order: (zones.length + 1) * 10, status: 'active' };
}

function defaultTariffForm(zones: Zone[] = [], tariffs: Tariff[] = []): Partial<Tariff> {
  const zoneID = zones[0]?.id || '';
  const zoneTariffs = tariffs.filter((tariff) => tariff.zone_id === zoneID);
  return { zone_id: zoneID, name: '', duration_minutes: 60, price_uzs: 0, sort_order: zoneTariffs.length + 1, status: 'active' };
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
  }

  function resetTariff() {
    setTariffForm(defaultTariffForm(settings?.zones || [], settings?.tariffs || []));
  }

  function resetPC() {
    setPCForm(defaultPCForm(settings?.zones || [], settings?.pcs || []));
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
    pcs: 'Компьютеры и QR',
    users: 'Доступы',
  }[settingsSection];
  const visibleUsers = settings
    ? canManageNetwork
      ? settings.users.filter((user) => user.role === 'owner' || user.role === 'admin')
      : settings.users.filter((user) => user.role === 'admin')
    : [];

  return (
    <main className="shell">
      <WorkspaceHeader auth={auth} selectedClubID={selectedClubID} currentPath={currentPath} onClubChange={onClubChange} onLogout={onLogout} eyebrow="Настройки" title={settingsTitle} />
      {canManageNetwork && (
        <div className="page-actions">
          <Button variant="secondary" icon={<Plus size={16} />} onClick={startNewClub}>Новый клуб</Button>
        </div>
      )}
      {message && <Notice tone="success">{message}</Notice>}
      {error && <Notice tone="danger">{error}</Notice>}
      {!creatingClub && clubForm?.id && (
        <div className="settings-subnav">
          <LinkButton href="/settings" variant={settingsSection === 'club' ? 'secondary' : 'ghost'} icon={<Building2 size={16} />}>Клуб</LinkButton>
          <LinkButton href="/settings/zones" variant={settingsSection === 'zones' ? 'secondary' : 'ghost'} icon={<ReceiptText size={16} />}>Зоны и пакеты</LinkButton>
          <LinkButton href="/settings/pcs" variant={settingsSection === 'pcs' ? 'secondary' : 'ghost'} icon={<Monitor size={16} />}>Компьютеры и QR</LinkButton>
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
              caption={canManageNetwork ? 'Основные данные клуба, прямые подключения Click/Payme и параметры Soliq/OFD.' : 'Основные данные клуба и состояние платежного подключения.'}
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
                  <Field label="Click secret key" type="password" value={clubForm.click_secret_key} onChange={(value) => setClubForm({ ...clubForm, click_secret_key: value })} help="Секрет для проверки Prepare/Complete callback." />
                  <Field label="Payme merchant ID" value={clubForm.payme_merchant_id} onChange={(value) => setClubForm({ ...clubForm, payme_merchant_id: value })} help="ID кассы/мерчанта Payme для checkout-ссылки." />
                  <Field label="Payme secret key" type="password" value={clubForm.payme_secret_key} onChange={(value) => setClubForm({ ...clubForm, payme_secret_key: value })} help="Для sandbox укажите TEST_KEY из Payme Business; для production — боевой secret." />
                  <Field
                    label="Комиссия платформы, %"
                    type="number"
                    step="0.01"
                    min="0"
                    value={bpsToPercentInput(clubForm.platform_fee_bps)}
                    onChange={(value) => setClubForm({ ...clubForm, platform_fee_bps: percentInputToBPS(value) })}
                    help="Введите процент: 2 = 2%. В системе это сохранится как 200 bps."
                  />
                  <SelectField label="Статус клуба" value={clubForm.status} options={statusOptions()} onChange={(value) => setClubForm({ ...clubForm, status: value })} help="Отключенный клуб не должен принимать новые оплаты." />
                  <Field label="OFD MXIK" value={clubForm.ofd_mxik} onChange={(value) => setClubForm({ ...clubForm, ofd_mxik: value })} help="Код услуги для фискального чека." />
                  <Field label="OFD package_code" value={clubForm.ofd_package_code} onChange={(value) => setClubForm({ ...clubForm, ofd_package_code: value })} help="Код единицы/пакета для OFD." />
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
                <button className="editable-row" key={zone.id} onClick={() => setZoneForm(zone)}>
                  <span>
                    <strong>{zone.name}</strong>
                    <small>1 час: {formatUZS(zone.hourly_price_uzs)} · {statusLabel(zone.status)} · порядок {zone.sort_order}</small>
                  </span>
                  <em>Изменить</em>
                </button>
              ))}
            </div>
            <div className="button-row">
              <Button size="sm" variant="secondary" icon={<Plus size={13} />} onClick={resetZone}>Новая зона</Button>
            </div>
            <div className="form-mode">
              <strong>{zoneForm.id ? 'Редактирование зоны' : 'Новая зона'}</strong>
              <span>Клик по зоне выше открывает редактирование. Для создания нажмите “Новая зона”.</span>
            </div>
            <div className="form-grid">
              <Field label="Название зоны" value={zoneForm.name || ''} onChange={(value) => setZoneForm({ ...zoneForm, name: value })} />
              <Field label="Стоимость 1 часа, сум" type="number" value={String(zoneForm.hourly_price_uzs || 0)} onChange={(value) => setZoneForm({ ...zoneForm, hourly_price_uzs: Number(value || 0) })} help="Используется, когда клиент или админ вводит сумму вручную без пакета." />
              <Field label="Порядок в списке" type="number" value={String(zoneForm.sort_order || 0)} onChange={(value) => setZoneForm({ ...zoneForm, sort_order: Number(value || 0) })} help="Меньше число — выше в списке. Можно ставить 10, 20, 30, чтобы потом вставлять между ними." />
              <SelectField label="Статус" value={zoneForm.status || 'active'} options={statusOptions()} onChange={(value) => setZoneForm({ ...zoneForm, status: value })} />
            </div>
            <div className="button-row">
              {zoneForm.id && <Button variant="danger" icon={<Trash2 size={16} />} onClick={deleteZone}>Удалить</Button>}
              <Button icon={<Save size={16} />} onClick={saveZone}>{zoneForm.id ? 'Сохранить зону' : 'Добавить зону'}</Button>
            </div>
          </Panel>

          <Panel className="stack">
            <SectionTitle icon={<ReceiptText size={18} />} title="Пакеты" caption="Пакет выбирает клиент на QR-странице. Цена пакета фиксированная и не зависит от стоимости часа зоны." />
            <div className="compact-list">
              {settings.tariffs.map((tariff) => (
                <button className="editable-row" key={tariff.id} onClick={() => setTariffForm(tariff)}>
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
            <div className="form-mode">
              <strong>{tariffForm.id ? 'Редактирование пакета' : 'Новый пакет'}</strong>
              <span>Если хотите добавить пакет, сначала нажмите “Новый пакет”.</span>
            </div>
            <div className="form-grid two">
              <SelectField label="Зона" value={tariffForm.zone_id || ''} options={settings.zones.map((zone) => ({ value: zone.id, label: zone.name }))} onChange={(value) => setTariffForm({ ...tariffForm, zone_id: value })} />
              <Field label="Название" value={tariffForm.name || ''} onChange={(value) => setTariffForm({ ...tariffForm, name: value })} />
              <Field label="Минуты" type="number" value={String(tariffForm.duration_minutes || 0)} onChange={(value) => setTariffForm({ ...tariffForm, duration_minutes: Number(value || 0) })} />
              <Field label="Цена, сум" type="number" value={String(tariffForm.price_uzs || 0)} onChange={(value) => setTariffForm({ ...tariffForm, price_uzs: Number(value || 0) })} />
              <Field label="Порядок в списке" type="number" value={String(tariffForm.sort_order || 0)} onChange={(value) => setTariffForm({ ...tariffForm, sort_order: Number(value || 0) })} help="Влияет на порядок пакетов на QR-странице." />
              <SelectField label="Статус" value={tariffForm.status || 'active'} options={statusOptions()} onChange={(value) => setTariffForm({ ...tariffForm, status: value })} />
            </div>
            <div className="button-row">
              {tariffForm.id && <Button variant="danger" icon={<Trash2 size={16} />} onClick={deleteTariff}>Удалить</Button>}
              <Button icon={<Save size={16} />} onClick={saveTariff}>{tariffForm.id ? 'Сохранить пакет' : 'Добавить пакет'}</Button>
            </div>
          </Panel>
            </>
          )}

          {!creatingClub && settingsSection === 'pcs' && (
          <Panel className="stack settings-wide">
            <SectionTitle icon={<Monitor size={18} />} title="Компьютеры и QR" caption="Добавьте компьютеры клуба. QR создается автоматически после сохранения ПК." />
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
                  {settings.pcs.map((pc) => (
                    <tr key={pc.id}>
                      <td>{pc.label} · #{pc.number}</td>
                      <td>{pc.zone}</td>
                      <td>{pc.qr_url ? <AppLink className="text-link" href={pc.qr_url}>{pc.qr_token}</AppLink> : '—'}</td>
                      <td><Button size="sm" variant="ghost" onClick={() => setPCForm(pc)}>Изменить</Button></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="button-row">
              <Button size="sm" variant="secondary" icon={<Plus size={13} />} onClick={resetPC}>Новый ПК</Button>
            </div>
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
              {visibleUsers.length === 0 && <EmptyState text="Админы клуба пока не добавлены" />}
            </div>
            <div className="button-row">
              <Button size="sm" variant="secondary" icon={<Plus size={13} />} onClick={resetUser}>{canManageNetwork ? 'Новый пользователь' : 'Новый админ'}</Button>
            </div>
            <div className="form-mode">
              <strong>{userForm.id ? 'Редактирование доступа' : 'Новый пользователь'}</strong>
              <span>{canManageNetwork ? 'Суперадмин может добавить владельца клуба или админа.' : 'Владелец может добавлять только админов. Владельцев добавляет команда Clubpay.'}</span>
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
                  <input value="Админ" readOnly />
                </label>
              )}
              <SelectField label="Статус" value={userForm.status || 'active'} options={statusOptions()} onChange={(value) => setUserForm({ ...userForm, status: value })} />
            </div>
            <div className="button-row">
              {userForm.id && <Button variant="danger" icon={<Trash2 size={16} />} onClick={deleteUser}>Удалить</Button>}
              <Button icon={<Save size={16} />} onClick={saveUser}>{userForm.id ? 'Сохранить доступ' : canManageNetwork ? 'Добавить пользователя' : 'Добавить админа'}</Button>
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

function WorkspaceHeader({ auth, selectedClubID, currentPath, onClubChange, onLogout, eyebrow, title }: WorkspaceProps & { eyebrow: string; title: string }) {
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
        {canOpenOwner && <AppLink className={`btn ghost sm ${currentPath.startsWith('/reports') || currentPath.startsWith('/owner') ? 'active' : ''}`} href="/reports"><Banknote size={13} /><span>Отчёт</span></AppLink>}
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
  const clickReady = club.click_connected ?? Boolean(club.click_service_id);
  const paymeReady = club.payme_connected ?? Boolean(club.payme_merchant_id);
  const paymentReady = club.payment_connected ?? (clickReady || paymeReady);
  const payoutReady = club.payouts_connected ?? club.platform_fee_bps > 0;
  const fiscalReady = club.fiscal_connected ?? Boolean(club.ofd_mxik && club.ofd_package_code);
  const clubActive = club.status === 'active';
  const connectedProviders = [clickReady && 'Click', paymeReady && 'Payme'].filter(Boolean).join(', ');

  return (
    <div className="connection-summary">
      <div className="connection-summary-head">
        <div>
          <strong>Подключение клуба</strong>
          <span>Технические ключи Click/Payme и Soliq заполняет команда Clubpay после подключения клуба.</span>
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
          ready={payoutReady}
          title="Комиссия платформы"
          value={payoutReady ? formatPercentFromBPS(club.platform_fee_bps) : 'Не задана'}
          caption="Коммерческую ставку меняет только команда Clubpay."
        />
        <ConnectionItem
          icon={<ReceiptText size={18} />}
          ready={fiscalReady}
          title="Фискализация"
          value={fiscalReady ? 'Настроена' : 'Ждёт настройки'}
          caption={fiscalReady ? 'Коды OFD сохранены.' : 'Нужны корректные MXIK и package_code.'}
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
  };
  return dictionary[message] || message;
}

function bpsToPercentInput(bps?: number) {
  const percent = Number(bps || 0) / 100;
  return Number.isInteger(percent) ? String(percent) : String(Number(percent.toFixed(4)));
}

function percentInputToBPS(value: string) {
  const percent = Number(value.replace(',', '.') || 0);
  if (!Number.isFinite(percent) || percent < 0) return 0;
  return Math.round(percent * 100);
}

function formatPercentFromBPS(bps?: number) {
  const percent = Number(bps || 0) / 100;
  return `${percent.toLocaleString('ru-RU', { maximumFractionDigits: 2 })}%`;
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
    not_requested: 'Soliq не запрошен',
    pending: 'Soliq в обработке',
    not_confirmed: 'Soliq не подтверждён',
    confirmed: 'Soliq подтверждён',
    failed: 'Ошибка Soliq',
  };
  return labels[status || ''] || 'Soliq не подтверждён';
}

function clubRole(auth: AuthPayload, clubID: string) {
  if (auth.user.global_role === 'super_admin') return 'super_admin';
  return auth.clubs.find((club) => club.id === clubID)?.role || '';
}

function canViewAdmin(auth: AuthPayload, clubID: string) {
  return ['super_admin', 'owner', 'admin'].includes(clubRole(auth, clubID));
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
    admin: 'Админ',
  };
  return labels[role] || role;
}

function roleOptions(canManageNetwork = false) {
  return canManageNetwork
    ? [
        { value: 'owner', label: 'Владелец' },
        { value: 'admin', label: 'Админ' },
      ]
    : [{ value: 'admin', label: 'Админ' }];
}

function statusOptions() {
  return [
    { value: 'active', label: 'Активен' },
    { value: 'inactive', label: 'Отключен' },
  ];
}

createRoot(document.getElementById('root')!).render(<App />);
