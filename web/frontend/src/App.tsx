import { useEffect, useRef, useState } from "react";
import { request, money, type Row } from "./api";
type IconName =
  | "store"
  | "orders"
  | "box"
  | "headset"
  | "chart"
  | "search"
  | "bell"
  | "chevron"
  | "refresh"
  | "cart"
  | "clock"
  | "shield"
  | "truck"
  | "close"
  | "check"
  | "more"
  | "logout"
  | "warning";

const paths: Record<IconName, React.ReactNode> = {
  store: (
    <>
      <path d="M4 10v10h16V10" />
      <path d="M3 10l2-6h14l2 6M8 20v-6h8v6" />
      <path d="M3 10c0 2 3 2 3 0 0 2 3 2 3 0 0 2 3 2 3 0 0 2 3 2 3 0 0 2 3 2 3 0" />
    </>
  ),
  orders: (
    <>
      <path d="M6 3h12v18l-3-2-3 2-3-2-3 2V3Z" />
      <path d="M9 8h6M9 12h6" />
    </>
  ),
  box: (
    <>
      <path d="m4 7 8-4 8 4-8 4-8-4Z" />
      <path d="m4 7 8 4v10l-8-4V7Zm16 0-8 4v10l8-4V7Z" />
    </>
  ),
  headset: (
    <>
      <path d="M4 14v-3a8 8 0 0 1 16 0v3" />
      <path d="M4 14a2 2 0 0 1 2-2h2v7H6a2 2 0 0 1-2-2v-3Zm16 0a2 2 0 0 0-2-2h-2v7h2a2 2 0 0 0 2-2v-3ZM16 19c0 2-2 2-4 2" />
    </>
  ),
  chart: (
    <>
      <path d="M4 20V10M10 20V4M16 20v-7M22 20H2" />
    </>
  ),
  search: (
    <>
      <circle cx="11" cy="11" r="7" />
      <path d="m20 20-4-4" />
    </>
  ),
  bell: (
    <>
      <path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9ZM10 21h4" />
    </>
  ),
  chevron: <path d="m9 18 6-6-6-6" />,
  refresh: (
    <>
      <path d="M20 7h-5V2" />
      <path d="M20 7a9 9 0 1 0 1 8" />
    </>
  ),
  cart: (
    <>
      <path d="M3 4h2l2 11h10l3-8H6" />
      <circle cx="9" cy="20" r="1" />
      <circle cx="17" cy="20" r="1" />
    </>
  ),
  clock: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7v5l3 2" />
    </>
  ),
  shield: (
    <>
      <path d="M12 3 4 6v5c0 5 3 8 8 10 5-2 8-5 8-10V6l-8-3Z" />
      <path d="m9 12 2 2 4-5" />
    </>
  ),
  truck: (
    <>
      <path d="M3 6h11v11H3V6Zm11 4h4l3 4v3h-7v-7Z" />
      <circle cx="7" cy="18" r="2" />
      <circle cx="18" cy="18" r="2" />
    </>
  ),
  close: <path d="m6 6 12 12M18 6 6 18" />,
  check: <path d="m5 12 4 4L19 6" />,
  more: (
    <>
      <circle cx="5" cy="12" r="1" />
      <circle cx="12" cy="12" r="1" />
      <circle cx="19" cy="12" r="1" />
    </>
  ),
  logout: (
    <>
      <path d="M10 5H5v14h5M14 8l4 4-4 4M8 12h10" />
    </>
  ),
  warning: (
    <>
      <path d="M12 3 2 21h20L12 3Z" />
      <path d="M12 9v5M12 18h.01" />
    </>
  ),
};

function Icon({ name, size = 18 }: { name: IconName; size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {paths[name]}
    </svg>
  );
}

function ProductArt({ kind }: { kind: string }) {
  return (
    <div className={`product-art ${kind}`}>
      {kind === "coffee" && (
        <>
          <div className="coffee-filter" />
          <div className="coffee-pot" />
          <span />
        </>
      )}
      {kind === "cup" && (
        <>
          <div className="cup-body" />
          <div className="cup-lid" />
          <span />
        </>
      )}
      {kind === "tray" && (
        <>
          <div className="tray-body" />
          <div className="tray-item one" />
          <div className="tray-item two" />
        </>
      )}
    </div>
  );
}

type Role = "buyer" | "warehouse" | "cs" | "ops";
type User = { account: string; display_name: string; role: Role };
const menus: Record<Role, string[]> = {
  buyer: ["商品与订单", "我的订单"],
  warehouse: ["待发货订单"],
  cs: ["退款审核", "退款执行", "异常支付"],
  ops: ["运营概览", "商品管理", "业务核查", "渠道演示"],
};
const roles = { buyer: "买家", warehouse: "仓库", cs: "客服", ops: "运营" };
const labels: Record<string, string> = {
  PENDING_PAYMENT: "待付款",
  PAID: "已付款",
  SHIPPED: "已发货",
  CANCELLED: "已取消",
  CLOSED_TIMEOUT: "超时关闭",
  REFUND_PENDING: "退款处理中",
  REFUND_REQUESTED: "退款待审核",
  REFUND_FAILED: "退款失败",
  REFUNDED: "已退款",
  PENDING: "待审核",
  APPROVED: "已批准",
  REJECTED: "已拒绝",
};
const show = (v: unknown) => String(v ?? "—");
export default function App() {
  const [user, setUser] = useState<User | null>(null),
    [token, setToken] = useState(() => sessionStorage.getItem("commerce-token") || "");
  const [active, setActive] = useState(""),
    [rows, setRows] = useState<Row[]>([]),
    [products, setProducts] = useState<Row[]>([]),
    [summary, setSummary] = useState<Row>({});
  const [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [busy, setBusy] = useState(false),
    [loading, setLoading] = useState(false),
    [page, setPage] = useState(1),
    [total, setTotal] = useState(0);
  const [purchase, setPurchase] = useState<Row | null>(null),
    [quantity, setQuantity] = useState(1),
    [orderNo, setOrderNo] = useState("");
  const [config, setConfig] = useState<Row>({}),
    [now, setNow] = useState(Date.now());
  const generation = useRef(0);
  const locked = useRef(false);
  function logout() {
    generation.current++;
    sessionStorage.removeItem("commerce-token");
    setToken("");
    setUser(null);
    setRows([]);
    setProducts([]);
    setPurchase(null);
    setNotice("");
    setError("");
  }
  async function api(path: string, method = "GET", body?: unknown) {
    try {
      return await request(path, token, method, body);
    } catch (e) {
      if (e instanceof Error && e.message.startsWith("401:")) logout();
      throw e;
    }
  }
  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    request("/users/me", token)
      .then((d) => {
        if (cancelled) return;
        const u = (d.user || d) as User;
        setUser(u);
        setActive(menus[u.role]?.[0] || "");
      })
      .catch((e) => {
        if (!cancelled) {
          logout();
          setError(String(e));
        }
      });
    return () => {
      cancelled = true;
    };
  }, [token]);
  async function refresh(silent = false) {
    if (!user) return;
    const id = ++generation.current;
    if (!silent) setLoading(true);
    try {
      let list: Row[] = [],
        ps: Row[] | null = null,
        sm: Row | null = null,
        cfg: Row | null = null,
        count = 0;
      if (user.role === "buyer") {
        const [p, o] = await Promise.all([
          api("/products"),
          api(`/orders/mine?page=${page}&page_size=20`),
        ]);
        ps = p.items ?? [];
        list = o.items ?? [];
        count = Number(o.total ?? 0);
      } else if (user.role === "warehouse") list = (await api("/warehouse/orders")).items ?? [];
      else if (user.role === "cs")
        list =
          (await api(active === "异常支付" ? "/cs/pay-exceptions" : "/cs/refunds")).items ?? [];
      else if (active === "运营概览") sm = await api("/ops/summary");
      else if (active === "商品管理")
        ps = (await api("/products?include_off_sale=true")).items ?? [];
      else if (active === "渠道演示") {
        const [c, r] = await Promise.all([api("/channel/config"), api("/channel/records")]);
        cfg = c;
        list = r.items ?? [];
      }
      if (id !== generation.current) return;
      setRows(list);
      setTotal(count);
      if (ps) setProducts(ps);
      if (sm) setSummary(sm);
      if (cfg)
        setConfig({
          pay_behavior: "success",
          notify_count: 1,
          notify_delay_seconds: 0,
          pay_late_seconds: 0,
          refund_success_from_attempt: 1,
          ...cfg,
        });
    } catch (e) {
      if (id === generation.current) setError(String(e));
    } finally {
      if (id === generation.current) setLoading(false);
    }
  }
  useEffect(() => {
    setRows([]);
    setError("");
    void refresh();
    const t = setInterval(() => {
      if (!locked.current && (user?.role !== "ops" || active === "运营概览")) void refresh(true);
    }, 5000);
    return () => {
      generation.current++;
      clearInterval(t);
    };
  }, [user, active, page]);
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, []);
  async function action(work: () => Promise<Row>, message = "操作已受理，状态以服务端结果为准") {
    if (locked.current) return;
    locked.current = true;
    generation.current++;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const result = await work();
      setNotice(message);
      if (!(user?.role === "ops" && active === "业务核查")) await refresh();
      return result;
    } catch (e) {
      setError(String(e));
    } finally {
      locked.current = false;
      setBusy(false);
    }
  }
  function reason(title: string) {
    const value = window.prompt(title);
    return value?.trim() || null;
  }
  function orderActions(r: Row) {
    const base = `/orders/${encodeURIComponent(show(r.order_no))}`;
    return (
      <>
        {r.status === "PENDING_PAYMENT" && (
          <>
            <button
              disabled={busy}
              onClick={() =>
                void action(() => api(base + "/pay", "POST"), "支付已发起，请等待渠道通知")
              }
            >
              付款
            </button>
            <button
              disabled={busy}
              onClick={() => void action(() => api(base + "/cancel", "POST"))}
            >
              取消订单
            </button>
          </>
        )}
        {r.status === "PAID" && (
          <button
            disabled={busy}
            onClick={() => {
              const v = reason("请输入售前退款原因");
              if (v) void action(() => api(base + "/refund", "POST", { reason: v }));
            }}
          >
            申请退款
          </button>
        )}
      </>
    );
  }
  function recordsTable(items: Row[], kind: string) {
    return (
      <div className="live-table">
        <table>
          <thead>
            <tr>
              {(kind === "order"
                ? ["订单 / 商品", "数量 / 金额", "状态 / 时间", "操作"]
                : kind === "refund"
                  ? ["订单 / 申请编号", "申请原因", "审核状态", "操作"]
                  : kind === "exception"
                    ? ["订单 / 交易号", "金额 / 异常原因", "处理状态", "操作"]
                    : ["时间 / 订单", "类型", "详情 / 结果", "操作人"]
              ).map((h) => (
                <th key={h}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {items.map((r, i) => (
              <tr key={show(r.id || r.order_no || i)}>
                {kind === "order" ? (
                  <>
                    <td>
                      <strong>{show(r.product_name)}</strong>
                      <small>{show(r.order_no)}</small>
                    </td>
                    <td>
                      {show(r.quantity)} 件<small>¥{money(r.amount_cents)}</small>
                    </td>
                    <td>
                      {user?.role === "warehouse"
                        ? "待发货"
                        : labels[show(r.status)] || show(r.status)}
                      <small>
                        {r.status === "PENDING_PAYMENT"
                          ? `剩余 ${Math.max(0, Math.ceil((Date.parse(show(r.pay_expire_at)) - now) / 1000))} 秒`
                          : show(r.carrier || r.created_at || r.paid_at)}{" "}
                        {show(r.tracking_no || "")}
                      </small>
                      {r.refund_fail_reason && <small>{show(r.refund_fail_reason)}</small>}
                    </td>
                    <td>
                      {user?.role === "buyer" ? (
                        orderActions(r)
                      ) : (
                        <button
                          disabled={busy}
                          onClick={() => {
                            const carrier = reason("请输入物流公司");
                            if (!carrier) return;
                            const tracking_no = reason("请输入物流单号");
                            if (tracking_no)
                              void action(
                                () =>
                                  api(
                                    `/warehouse/orders/${encodeURIComponent(show(r.order_no))}/ship`,
                                    "POST",
                                    { carrier, tracking_no },
                                  ),
                                "发货登记成功",
                              );
                          }}
                        >
                          登记发货
                        </button>
                      )}
                    </td>
                  </>
                ) : kind === "refund" ? (
                  <>
                    <td>
                      {show(r.order_no)}
                      <small>{show(r.id)}</small>
                    </td>
                    <td>
                      {show(r.reason)}
                      <small>{show(r.review_reason || "")}</small>
                    </td>
                    <td>{labels[show(r.status)] || show(r.status)}</td>
                    <td>
                      {r.status === "PENDING" &&
                        [true, false].map((approve) => (
                          <button
                            key={String(approve)}
                            disabled={busy}
                            onClick={() => {
                              const v = reason("请输入审核理由");
                              if (v)
                                void action(() =>
                                  api(
                                    `/cs/refunds/${encodeURIComponent(show(r.id))}/review`,
                                    "POST",
                                    { approve, reason: v },
                                  ),
                                );
                            }}
                          >
                            {approve ? "批准" : "拒绝"}
                          </button>
                        ))}
                    </td>
                  </>
                ) : kind === "exception" ? (
                  <>
                    <td>
                      {show(r.order_no)}
                      <small>{show(r.channel_trade_no)}</small>
                    </td>
                    <td>
                      ¥{money(r.amount_cents)}
                      <small>
                        {show(r.reason)} {show(r.detail)}
                      </small>
                    </td>
                    <td>{show(r.status)}</td>
                    <td>
                      {r.reason === "late_payment" && r.status !== "REFUNDED" && (
                        <button
                          disabled={busy}
                          onClick={() =>
                            void action(() =>
                              api(
                                `/cs/pay-exceptions/${encodeURIComponent(show(r.id))}/refund`,
                                "POST",
                              ),
                            )
                          }
                        >
                          退回迟到支付
                        </button>
                      )}
                    </td>
                  </>
                ) : (
                  <>
                    <td>
                      {show(r.created_at)}
                      <small>{show(r.order_no)}</small>
                    </td>
                    <td>{show(r.type)}</td>
                    <td>
                      {show(r.detail)}
                      <small>{show(r.result || "")}</small>
                    </td>
                    <td>{show(r.operator || "渠道")}</td>
                  </>
                )}
              </tr>
            ))}
          </tbody>
        </table>
        {!items.length && <p className="empty-state">暂无记录</p>}
      </div>
    );
  }
  if (!user)
    return (
      <div className="login-screen">
        <form
          className="login-card"
          onSubmit={(e) => {
            e.preventDefault();
            const data = new FormData(e.currentTarget);
            void action(
              () =>
                request("/users/login", "", "POST", {
                  account: data.get("account"),
                  password: data.get("password"),
                }).then((d) => {
                  sessionStorage.setItem("commerce-token", show(d.token));
                  setToken(show(d.token));
                  return d;
                }),
              "登录成功",
            );
          }}
        >
          <div className="brand">
            <div className="brand-mark">
              <span />
              <span />
              <span />
            </div>
            <h2>Commerce Lab</h2>
          </div>
          <h1>欢迎回来</h1>
          <p>登录后进入你的交易工作台</p>
          <label>
            账号
            <input name="account" required autoComplete="username" defaultValue="buyer1" />
          </label>
          <label>
            密码
            <input name="password" type="password" required autoComplete="current-password" />
          </label>
          <button className="live-primary" disabled={busy}>
            {busy ? "正在登录…" : "登录"}
          </button>
          <small>
            教学账号：buyer1 / buyer2 / buyer3、warehouse01、cs01、ops01；初始密码 123456
          </small>
          {error && (
            <p role="alert" className="live-error">
              {error}
            </p>
          )}
        </form>
      </div>
    );
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark">
            <span />
            <span />
            <span />
          </div>
          <strong>Commerce Lab</strong>
        </div>
        <div className="profile-card">
          <span className="avatar">{user.display_name?.slice(0, 1) || "用"}</span>
          <span>
            <strong>{user.display_name || user.account}</strong>
            <small>
              {roles[user.role]} · {user.account}
            </small>
          </span>
        </div>
        <nav>
          {menus[user.role].map((n) => (
            <button
              key={n}
              className={`live-nav ${active === n ? "active" : ""}`}
              onClick={() => {
                setActive(n);
                setPage(1);
                setNotice("");
              }}
            >
              <Icon name={n === "商品与订单" ? "store" : "orders"} />
              <span>{n}</span>
            </button>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <p>教学交易环境</p>
          <button disabled={busy} onClick={logout}>
            <Icon name="logout" /> 退出 / 切换账号
          </button>
        </div>
      </aside>
      <main>
        <header>
          <div className="date">
            <Icon name="clock" />
            {new Date(now).toLocaleString("zh-CN")}
          </div>
          <span>{roles[user.role]}工作台</span>
        </header>
        <div className="content">
          <div className="live-heading">
            <div>
              <small>COMMERCE LAB / WORKSPACE</small>
              <h1>{active}</h1>
            </div>
            <button disabled={busy || loading} onClick={() => void refresh()}>
              <Icon name="refresh" /> {loading ? "加载中…" : "刷新"}
            </button>
          </div>
          {error && (
            <div role="alert" className="live-error">
              {error}
            </div>
          )}
          {notice && (
            <div role="status" className="live-notice">
              {notice}
            </div>
          )}
          {user.role === "buyer" && active === "商品与订单" && (
            <>
              <section className="hero-card">
                <div>
                  <span>让每一次交易，都安心有序</span>
                  <h1>把日常，过成喜欢的样子</h1>
                  <p>每单购买一种商品，数量为 1 至 2 件。付款期限以订单显示为准。</p>
                </div>
                <div className="hero-stat">
                  <strong>{products.length}</strong>
                  <span>在售商品</span>
                </div>
              </section>
              <div className="product-grid">
                {products.map((p, i) => (
                  <article className="product-card" key={show(p.id)}>
                    <div
                      className="product-visual"
                      style={{ background: i % 2 ? "#dce2e8" : "#d9e6d5" }}
                    >
                      <ProductArt kind={i % 2 ? "cup" : "coffee"} />
                    </div>
                    <div className="product-info">
                      <h3>{show(p.name)}</h3>
                      <p>可售 {Number(p.available ?? 0)} 件</p>
                      <div className="live-heading">
                        <strong>¥{money(p.price_cents)}</strong>
                        <button
                          className="live-primary"
                          disabled={busy || Number(p.available ?? 0) < 1}
                          onClick={() => {
                            setQuantity(1);
                            setPurchase(p);
                          }}
                        >
                          立即购买
                        </button>
                      </div>
                    </div>
                  </article>
                ))}
              </div>
              {!products.length && <p className="empty-state">暂无在售商品</p>}
            </>
          )}
          {user.role === "buyer" && (
            <section className="live-panel">
              <h2>{active === "我的订单" ? "我的订单" : "最近订单"}</h2>
              {recordsTable(rows, "order")}
              <div className="live-heading">
                <button disabled={page <= 1 || loading} onClick={() => setPage(page - 1)}>
                  上一页
                </button>
                <span>
                  第 {page} 页 · 共 {total} 单
                </span>
                <button disabled={page * 20 >= total || loading} onClick={() => setPage(page + 1)}>
                  下一页
                </button>
              </div>
            </section>
          )}
          {user.role === "warehouse" && (
            <section className="live-panel">
              <p>仅显示当前可发货订单；已发货订单可由运营按订单号核查流水。</p>
              {recordsTable(rows, "order")}
            </section>
          )}
          {user.role === "cs" && (
            <section className="live-panel">
              {active === "退款执行" && (
                <form
                  className="live-form"
                  onSubmit={(e) => {
                    e.preventDefault();
                    void action(() =>
                      api(`/cs/orders/${encodeURIComponent(orderNo.trim())}/refund-retry`, "POST"),
                    );
                  }}
                >
                  <label>
                    退款失败的订单号
                    <input required value={orderNo} onChange={(e) => setOrderNo(e.target.value)} />
                  </label>
                  <button disabled={busy}>核查并重试退款</button>
                  <p>仅服务端判定允许重试的订单可执行；审核通过不等于渠道退款成功。</p>
                </form>
              )}
              {recordsTable(rows, active === "异常支付" ? "exception" : "refund")}
            </section>
          )}
          {user.role === "ops" && active === "运营概览" && (
            <div className="live-stats">
              {Object.entries({
                total_paid_cents: "累计收款",
                total_refunded_cents: "累计退款",
                pending_payment: "待付款",
                paid: "待发货",
                shipped: "已发货",
                cancelled: "已取消",
                closed_timeout: "超时关闭",
                refund_pending: "退款中",
                refund_failed: "退款失败",
                refunded: "已退款",
                pay_exceptions: "支付异常",
              }).map(([k, v]) => (
                <article key={k}>
                  <small>{v}</small>
                  <strong>
                    {k.endsWith("cents") ? "¥" + money(summary[k]) : Number(summary[k] ?? 0)}
                  </strong>
                </article>
              ))}
            </div>
          )}
          {user.role === "ops" && active === "商品管理" && (
            <section className="live-panel">
              {products.map((p) => (
                <div className="live-product" key={show(p.id)}>
                  <strong>{show(p.name)}</strong>
                  <span>
                    ¥{money(p.price_cents)} · 可售 {Number(p.available ?? 0)} · 占用{" "}
                    {Number(p.reserved ?? 0)} · {p.on_sale ? "在售" : "已下架"}
                  </span>
                  <button
                    disabled={busy}
                    onClick={() =>
                      void action(() =>
                        api(`/products/${encodeURIComponent(show(p.id))}`, "PUT", {
                          set_on_sale: true,
                          on_sale: !p.on_sale,
                        }),
                      )
                    }
                  >
                    {p.on_sale ? "下架" : "上架"}
                  </button>
                  <button
                    disabled={busy}
                    onClick={() => {
                      const v = reason("输入新价格（整数分）");
                      if (v && /^\d+$/.test(v) && Number.isSafeInteger(Number(v)) && Number(v) > 0)
                        void action(() =>
                          api(`/products/${encodeURIComponent(show(p.id))}`, "PUT", {
                            set_price: true,
                            price_cents: Number(v),
                          }),
                        );
                    }}
                  >
                    修改价格
                  </button>
                  <button
                    disabled={busy}
                    onClick={() => {
                      const v = reason("输入补货数量（正整数）");
                      if (v && /^\d+$/.test(v) && Number.isSafeInteger(Number(v)) && Number(v) > 0)
                        void action(() =>
                          api(`/products/${encodeURIComponent(show(p.id))}`, "PUT", {
                            stock_delta: Number(v),
                          }),
                        );
                    }}
                  >
                    调整数量
                  </button>
                </div>
              ))}
            </section>
          )}
          {user.role === "ops" && active === "业务核查" && (
            <section className="live-panel">
              <form
                className="live-form"
                onSubmit={(e) => {
                  e.preventDefault();
                  void action(async () => {
                    const d = await api(
                      `/ops/orders/${encodeURIComponent(orderNo.trim())}/records`,
                    );
                    setRows(d.items ?? []);
                    return d;
                  }, "核查完成");
                }}
              >
                <label>
                  订单号
                  <input required value={orderNo} onChange={(e) => setOrderNo(e.target.value)} />
                </label>
                <button disabled={busy}>查询流水</button>
              </form>
              {recordsTable(rows, "records")}
            </section>
          )}
          {user.role === "ops" && active === "渠道演示" && (
            <section className="live-panel">
              <h2>模拟支付渠道</h2>
              <form
                className="live-form"
                onSubmit={(e) => {
                  e.preventDefault();
                  void action(() => api("/channel/config", "PUT", config), "渠道配置已保存");
                }}
              >
                <label>
                  支付结果
                  <select
                    value={show(config.pay_behavior || "success")}
                    onChange={(e) => setConfig({ ...config, pay_behavior: e.target.value })}
                  >
                    <option value="success">成功</option>
                    <option value="fail">失败</option>
                    <option value="no_notify">不通知</option>
                  </select>
                </label>
                {[
                  ["notify_count", "通知次数", 1, 10],
                  ["notify_delay_seconds", "通知延迟（秒）", 0, 3600],
                  ["pay_late_seconds", "迟到支付（秒）", 0, 3600],
                  ["refund_success_from_attempt", "退款第几次起成功", 1, 100],
                ].map(([k, l, min, max]) => (
                  <label key={k}>
                    {l}
                    <input
                      required
                      type="number"
                      min={min}
                      max={max}
                      value={Number(config[k] ?? min)}
                      onChange={(e) => setConfig({ ...config, [k]: Number(e.target.value) })}
                    />
                  </label>
                ))}
                <button disabled={busy || loading}>保存配置</button>
              </form>
              {recordsTable(rows, "records")}
            </section>
          )}
        </div>
      </main>
      {purchase && (
        <div className="live-overlay">
          <form
            className="login-card"
            onSubmit={(e) => {
              e.preventDefault();
              void action(async () => {
                const d = await api("/orders", "POST", { product_id: purchase.id, quantity });
                setPurchase(null);
                setActive("我的订单");
                return d;
              }, "订单已创建，请在订单付款期限内支付");
            }}
          >
            <h2>确认购买</h2>
            <p>{show(purchase.name)}</p>
            <label>
              购买数量
              <select value={quantity} onChange={(e) => setQuantity(Number(e.target.value))}>
                <option value={1}>1 件</option>
                {Number(purchase.available) >= 2 && <option value={2}>2 件</option>}
              </select>
            </label>
            <p>合计 ¥{money(Number(purchase.price_cents) * quantity)}</p>
            <button className="live-primary" disabled={busy}>
              确认下单
            </button>
            <button type="button" disabled={busy} onClick={() => setPurchase(null)}>
              取消
            </button>
            {error && (
              <p role="alert" className="live-error">
                {error}
              </p>
            )}
          </form>
        </div>
      )}
    </div>
  );
}
