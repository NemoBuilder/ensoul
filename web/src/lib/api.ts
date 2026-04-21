const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8990";

// ApiError carries the structured error code from the backend (e.g.
// "INSUFFICIENT_CREDITS", "PRO_REQUIRED", "WORKSPACE_LIMIT") so the UI can
// gate behaviour reliably without string-matching the human-readable message.
export class ApiError extends Error {
  status: number;
  code?: string;
  body?: Record<string, unknown>;
  constructor(message: string, status: number, code?: string, body?: Record<string, unknown>) {
    super(message);
    this.status = status;
    this.code = code;
    this.body = body;
  }
}

// Generic fetch wrapper with error handling
async function apiFetch<T>(
  path: string,
  options?: RequestInit
): Promise<T> {
  const url = `${API_BASE}${path}`;
  const res = await fetch(url, {
    credentials: "include", // Send cookies for session auth
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  });

  if (!res.ok) {
    const body = (await res.json().catch(() => ({ error: "Request failed" }))) as Record<string, unknown>;
    const message = (body.error as string) || `HTTP ${res.status}`;
    const code = body.code as string | undefined;
    throw new ApiError(message, res.status, code, body);
  }

  return res.json();
}

// Authenticated fetch (for Claw endpoints)
function authFetch<T>(path: string, apiKey: string, options?: RequestInit) {
  return apiFetch<T>(path, {
    ...options,
    headers: {
      Authorization: `Bearer ${apiKey}`,
      ...options?.headers,
    },
  });
}

// --- Types ---

export interface DimensionData {
  score: number;
  summary: string;
}

export interface TwitterMeta {
  bio?: string;
  followers_count?: number;
  following_count?: number;
  tweet_count?: number;
  location?: string;
  verified?: boolean;
  account_created_at?: string;
  banner_url?: string;
  listed_count?: number;
  favourites_count?: number;
  data_source?: string;
}

export interface Shell {
  id: string;
  handle: string;
  token_id: number | null;
  owner_addr: string;
  stage: "embryo" | "growing" | "mature" | "evolving";
  dna_version: number;
  seed_summary: string;
  soul_prompt: string;
  dimensions: Record<string, DimensionData>;
  total_frags: number;
  accepted_frags: number;
  total_claws: number;
  total_chats: number;
  avatar_url: string;
  display_name: string;
  agent_id: number | null;
  twitter_meta?: TwitterMeta;
  created_at: string;
  updated_at: string;
}

export interface Fragment {
  id: string;
  shell_id: string;
  claw_id: string;
  dimension: string;
  content?: string;
  content_hash?: string;
  status: "pending" | "accepted" | "rejected";
  confidence: number;
  reject_reason?: string;
  tx_hash?: string;
  created_at: string;
  claw?: Claw;
  shell?: Shell;
}

export interface Claw {
  id: string;
  name: string;
  description: string;
  status: "pending_claim" | "claimed";
  twitter_handle?: string;
  wallet_addr: string;
  total_submitted: number;
  total_accepted: number;
  earnings: number;
  created_at: string;
}

export interface SeedPreview {
  handle: string;
  display_name: string;
  avatar_url: string;
  seed_summary: string;
  dimensions: Record<string, DimensionData>;
  twitter_meta?: TwitterMeta;
}

export interface Ensouling {
  id: string;
  shell_id: string;
  version_from: number;
  version_to: number;
  frags_merged: number;
  summary_diff: string;
  created_at: string;
}

export interface ClawRank {
  rank: number;
  id: string;
  name: string;
  description: string;
  total_submitted: number;
  total_accepted: number;
  accept_rate: string;
  earnings: number;
  created_at: string;
}

export interface ClawDimStat {
  Dimension: string;
  Total: number;
  Accepted: number;
}

export interface ClawShellContrib {
  ShellID: string;
  Handle: string;
  AvatarURL: string;
  DisplayName: string;
  FragCount: number;
  AcceptedCount: number;
}

export interface ClawProfile {
  claw: {
    id: string;
    name: string;
    description: string;
    status: string;
    total_submitted: number;
    total_accepted: number;
    accept_rate: string;
    earnings: number;
    created_at: string;
  };
  dimension_stats: ClawDimStat[];
  shell_contributions: ClawShellContrib[];
  recent_accepted: Fragment[];
}

export interface ShellContributor {
  claw_id: string;
  name: string;
  total_frags: number;
  accepted_frags: number;
}

export interface GlobalStats {
  souls: number;
  fragments: number;
  claws: number;
  chats: number;
}

export interface TaskItem {
  handle: string;
  dimension: string;
  score: number;
  priority: string;
  message: string;
}

export interface PaginatedResult<T> {
  total: number;
  page: number;
  limit: number;
  [key: string]: T[] | number;
}

// --- Shell API ---

export interface MintQuota {
  minted: number;
  limit: number;
  can_mint: boolean;
}

export const shellApi = {
  mintQuota: (wallet: string) =>
    apiFetch<MintQuota>(`/api/shell/mint-quota?wallet=${encodeURIComponent(wallet)}`),

  preview: (handle: string) =>
    apiFetch<SeedPreview>("/api/shell/preview", {
      method: "POST",
      body: JSON.stringify({ handle }),
    }),

  mint: (handle: string, ownerAddr: string, signature: string, preview: SeedPreview) =>
    apiFetch<Shell>("/api/shell/mint", {
      method: "POST",
      body: JSON.stringify({ handle, owner_addr: ownerAddr, preview }),
      headers: {
        "X-Wallet-Address": ownerAddr,
        "X-Wallet-Signature": signature,
      },
    }),

  confirm: (handle: string, txHash: string, ownerAddr: string, signature: string, agentId?: number) =>
    apiFetch<{ status: string }>("/api/shell/confirm", {
      method: "POST",
      body: JSON.stringify({ handle, tx_hash: txHash, agent_id: agentId ?? 0 }),
      headers: {
        "X-Wallet-Address": ownerAddr,
        "X-Wallet-Signature": signature,
      },
    }),

  cancel: (handle: string, ownerAddr: string, signature: string) =>
    apiFetch<{ status: string }>("/api/shell/cancel", {
      method: "POST",
      body: JSON.stringify({ handle }),
      headers: {
        "X-Wallet-Address": ownerAddr,
        "X-Wallet-Signature": signature,
      },
    }),

  list: (params?: {
    stage?: string;
    sort?: string;
    search?: string;
    page?: number;
    limit?: number;
  }) => {
    const query = new URLSearchParams();
    if (params?.stage) query.set("stage", params.stage);
    if (params?.sort) query.set("sort", params.sort);
    if (params?.search) query.set("search", params.search);
    if (params?.page) query.set("page", String(params.page));
    if (params?.limit) query.set("limit", String(params.limit));
    return apiFetch<{ shells: Shell[]; total: number; page: number; limit: number }>(
      `/api/shell/list?${query}`
    );
  },

  get: (handle: string) => apiFetch<Shell>(`/api/shell/${handle}`),

  byOwner: (address: string) =>
    apiFetch<{ shells: Shell[]; total: number }>(`/api/shell/by-owner/${address}`),

  getDimensions: (handle: string) =>
    apiFetch<Record<string, DimensionData>>(`/api/shell/${handle}/dimensions`),

  getHistory: (handle: string) =>
    apiFetch<Ensouling[]>(`/api/shell/${handle}/history`),

  getContributors: (handle: string) =>
    apiFetch<{ contributors: ShellContributor[] }>(`/api/shell/${handle}/contributors`),
};

// --- Fragment API ---

export const fragmentApi = {
  submit: (apiKey: string, handle: string, dimension: string, content: string) =>
    authFetch<Fragment>("/api/fragment/submit", apiKey, {
      method: "POST",
      body: JSON.stringify({ handle, dimension, content }),
    }),

  list: (params?: {
    handle?: string;
    status?: string;
    dimension?: string;
    page?: number;
    limit?: number;
  }) => {
    const query = new URLSearchParams();
    if (params?.handle) query.set("handle", params.handle);
    if (params?.status) query.set("status", params.status);
    if (params?.dimension) query.set("dimension", params.dimension);
    if (params?.page) query.set("page", String(params.page));
    if (params?.limit) query.set("limit", String(params.limit));
    return apiFetch<{ fragments: Fragment[]; total: number; page: number; limit: number }>(
      `/api/fragment/list?${query}`
    );
  },

  get: (id: string) => apiFetch<Fragment>(`/api/fragment/${id}`),
};

// --- Claw API ---

export const clawApi = {
  register: (name: string, description: string) =>
    apiFetch<{
      claw: { api_key: string; claim_url: string; verification_code: string };
      important: string;
    }>("/api/claw/register", {
      method: "POST",
      body: JSON.stringify({ name, description }),
    }),

  status: (apiKey: string) =>
    authFetch<{ status: string; claimed: boolean; claim_url: string }>(
      "/api/claw/status",
      apiKey
    ),

  claimInfo: (code: string) =>
    apiFetch<{ name: string; verification_code: string; status: string }>(
      `/api/claw/claim/${code}`
    ),

  claimVerify: (claimCode: string) =>
    apiFetch<{ success: boolean; message: string }>("/api/claw/claim/verify", {
      method: "POST",
      body: JSON.stringify({ claim_code: claimCode }),
    }),

  me: (apiKey: string) => authFetch<Claw>("/api/claw/me", apiKey),

  dashboard: (apiKey: string) =>
    authFetch<{
      overview: {
        total_submitted: number;
        total_accepted: number;
        accept_rate: string;
        earnings: number;
      };
      recent_contributions: Fragment[];
    }>("/api/claw/dashboard", apiKey),

  contributions: (apiKey: string, page?: number, limit?: number) => {
    const query = new URLSearchParams();
    if (page) query.set("page", String(page));
    if (limit) query.set("limit", String(limit));
    return authFetch<{ contributions: Fragment[]; total: number }>(
      `/api/claw/contributions?${query}`,
      apiKey
    );
  },

  // Public endpoints
  profile: (id: string) =>
    apiFetch<ClawProfile>(`/api/claw/profile/${id}`),

  leaderboard: (page?: number, limit?: number) => {
    const query = new URLSearchParams();
    if (page) query.set("page", String(page));
    if (limit) query.set("limit", String(limit));
    return apiFetch<{ claws: ClawRank[]; total: number; page: number; limit: number }>(
      `/api/claw/leaderboard?${query}`
    );
  },
};

// --- Chat API ---

export interface ChatSession {
  id: string;
  shell_id: string;
  wallet_addr?: string;
  tier: "guest" | "free" | "paid";
  rounds: number;
  title?: string;
  created_at: string;
  updated_at: string;
  shell?: Shell;
  messages?: ChatSessionMessage[];
}

export interface ChatSessionMessage {
  id: string;
  session_id: string;
  role: "user" | "assistant";
  content: string;
  created_at: string;
}

export const chatApi = {
  // Create a new chat session for a soul
  createSession: (handle: string) =>
    apiFetch<{ session_id: string; tier: string }>(`/api/chat/${handle}/session`, {
      method: "POST",
    }),

  // Send a message in a chat session (returns raw Response for SSE streaming)
  sendMessage: (sessionId: string, message: string) => {
    const url = `${API_BASE}/api/chat/sessions/${sessionId}/message`;
    return fetch(url, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ message }),
    });
  },

  // Get a chat session with its messages
  getSession: (sessionId: string) =>
    apiFetch<ChatSession>(`/api/chat/sessions/${sessionId}`),

  // List current user's chat sessions (requires login)
  listSessions: (handle?: string) => {
    const query = handle ? `?handle=${handle}` : "";
    return apiFetch<{ sessions: ChatSession[] }>(`/api/chat/sessions${query}`);
  },

  // Delete a chat session (requires login + ownership)
  deleteSession: (sessionId: string) =>
    apiFetch<{ status: string }>(`/api/chat/sessions/${sessionId}`, {
      method: "DELETE",
    }),
};

// --- Share API ---

export interface ChatShareMessage {
  role: "user" | "assistant";
  content: string;
}

export interface ChatShareData {
  id: string;
  code: string;
  session_id: string;
  shell_id: string;
  handle: string;
  avatar_url: string;
  stage: string;
  dna_version: number;
  messages: string; // JSON string of ChatShareMessage[]
  created_at: string;
}

export const shareApi = {
  // Create a share link for a chat session
  create: (sessionId: string, messageIndex: number = -1) =>
    apiFetch<{ code: string; share_url: string }>("/api/chat/share", {
      method: "POST",
      body: JSON.stringify({ session_id: sessionId, message_index: messageIndex }),
    }),

  // Get a share by its short code (public, no auth)
  get: (code: string) =>
    apiFetch<ChatShareData>(`/api/chat/share/${code}`),
};

// --- Stats API ---

export const statsApi = {
  global: () => apiFetch<GlobalStats>("/api/stats"),
};

// --- Tasks API ---

export const tasksApi = {
  list: () => apiFetch<TaskItem[]>("/api/tasks"),
};

// --- Session Auth API (wallet signature login, HttpOnly cookie) ---

export const sessionApi = {
  login: (address: string, signature: string, message: string) =>
    apiFetch<{ address: string; message: string }>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ address, signature, message }),
    }),

  logout: () =>
    apiFetch<{ message: string }>("/api/auth/logout", {
      method: "POST",
    }),

  session: () =>
    apiFetch<{ address: string }>("/api/auth/session"),
};

// --- Email Auth API (email + verification code, HttpOnly cookie) ---

export interface EmailSessionInfo {
  user_id: string;
  email: string;
  twitter_handle?: string;
  wallet_addr?: string;
  is_pro: boolean;
  credits: number;
  has_password: boolean;
}

export const emailAuthApi = {
  sendCode: (email: string) =>
    apiFetch<{ message: string }>("/api/auth/email/send-code", {
      method: "POST",
      body: JSON.stringify({ email }),
    }),

  verify: (email: string, code: string) =>
    apiFetch<{ email: string; user_id: string; is_new: boolean; message: string }>("/api/auth/email/verify", {
      method: "POST",
      body: JSON.stringify({ email, code }),
    }),

  logout: () =>
    apiFetch<{ message: string }>("/api/auth/email/logout", {
      method: "POST",
    }),

  session: () =>
    apiFetch<EmailSessionInfo>("/api/auth/email/session"),

  passwordLogin: (email: string, password: string) =>
    apiFetch<{ email: string; user_id: string; message: string }>("/api/auth/email/password-login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }),

  setPassword: (password: string) =>
    apiFetch<{ message: string }>("/api/auth/email/set-password", {
      method: "POST",
      body: JSON.stringify({ password }),
    }),

  hasPassword: (email: string) =>
    apiFetch<{ has_password: boolean }>(`/api/auth/email/has-password?email=${encodeURIComponent(email)}`),
};

// --- Account Binding API (cross-link email ↔ wallet on the same User) ---

export const bindApi = {
  // Email user → bind a wallet (requires wallet signature on `ensoul:bind:<ts>`)
  wallet: (address: string, signature: string, message: string) =>
    apiFetch<{ wallet_addr: string; bound?: boolean; already_bound?: boolean }>(
      "/api/auth/bind/wallet",
      { method: "POST", body: JSON.stringify({ address, signature, message }) },
    ),

  // Wallet user → request email verification code for binding
  emailSend: (email: string) =>
    apiFetch<{ message: string }>("/api/auth/bind/email/send", {
      method: "POST",
      body: JSON.stringify({ email }),
    }),

  // Wallet user → verify code and bind email
  email: (email: string, code: string) =>
    apiFetch<{ email: string; bound?: boolean; already_bound?: boolean }>(
      "/api/auth/bind/email",
      { method: "POST", body: JSON.stringify({ email, code }) },
    ),
};

// --- Claw Key Management API (session-based, no API key in frontend) ---

export interface ClawBindingInfo {
  id: string;
  claw_id: string;
  claw_name: string;
  wallet_addr: string;
  mining_approved: boolean;
}

export const clawKeyApi = {
  // Bind a Claw API key to the current wallet session
  bind: (apiKey: string) =>
    apiFetch<{ id: string; name: string }>("/api/claw/keys", {
      method: "POST",
      body: JSON.stringify({ api_key: apiKey }),
    }),

  // List all Claws bound to the current wallet
  list: () =>
    apiFetch<{ claws: ClawBindingInfo[] }>("/api/claw/keys"),

  // Unbind a Claw from the current wallet
  unbind: (bindingId: string) =>
    apiFetch<{ message: string }>(`/api/claw/keys/${bindingId}`, {
      method: "DELETE",
    }),

  // Get dashboard data for a bound Claw
  dashboard: (bindingId: string) =>
    apiFetch<{
      overview: {
        total_submitted: number;
        total_accepted: number;
        accept_rate: string;
        earnings: number;
      };
      claw_id: string;
      wallet_addr: string;
      recent_contributions: Fragment[];
      mining_rewards: MiningReward[];
      total_earned: number;
      total_pending: number;
    }>(`/api/claw/keys/${bindingId}/dashboard`),
};

// ═══════════════════════════════════════════════════════════════
// Withdraw API
// ═══════════════════════════════════════════════════════════════

export interface WithdrawStatus {
  claw_wallet: string;
  user_wallet: string;
  token_balance: number;
  bnb_balance: number;
  withdrawable: number;
  has_gas: boolean;
  min_gas: number;
  min_amount: number;
  can_withdraw: boolean;
  reason?: string;
}

export interface WithdrawRecord {
  id: string;
  claw_id: string;
  from_addr: string;
  to_addr: string;
  amount: number;
  tx_hash?: string;
  status: string;
  last_error?: string;
  created_at: string;
}

export const withdrawApi = {
  // Pre-flight check: gas, balance, cooldown
  check: (clawId: string) =>
    apiFetch<WithdrawStatus>(`/api/claw/withdraw/check?claw_id=${clawId}`),

  // Execute withdrawal
  withdraw: (clawId: string, amount: number) =>
    apiFetch<{ message: string; withdraw_id: string; amount: number; status: string }>(
      "/api/claw/withdraw",
      {
        method: "POST",
        body: JSON.stringify({ claw_id: clawId, amount }),
      }
    ),

  // Get withdrawal history
  history: (clawId: string) =>
    apiFetch<{ withdrawals: WithdrawRecord[] }>(
      `/api/claw/withdraw/history?claw_id=${clawId}`
    ),
};

// ═══════════════════════════════════════════════════════════════
// Mining API (Phase 1)
// ═══════════════════════════════════════════════════════════════

export interface MiningPoolStatus {
  balance: number;
  total_deposited: number;
  total_released: number;
  daily_limit: number;
  daily_released: number;
  daily_remaining: number;
  daily_start_balance: number;
  paused: boolean;
  last_reset_at: string;
}

export interface FragmentDemand {
  id: string;
  shell_id: string;
  dimension: string;
  description: string;
  bounty: number;
  status: string;
  created_at: string;
  expires_at: string;
  shell?: Shell;
}

export interface MiningReward {
  id: string;
  claw_id: string;
  fragment_id: string;
  demand_id?: string;
  amount: number;
  tx_hash?: string;
  status: string;
  created_at: string;
}

export const miningApi = {
  pool: () => apiFetch<MiningPoolStatus>("/api/mining/pool"),
  demands: () =>
    apiFetch<{ demands: FragmentDemand[]; total: number }>("/api/mining/demands").then(
      (r) => r.demands ?? []
    ),
  rewards: (clawId: string) =>
    apiFetch<{ rewards: MiningReward[]; total_earned: number; total_pending: number }>(
      `/api/mining/rewards/${clawId}`
    ),
};

// ═══════════════════════════════════════════════════════════════
// Mint V2 API (Phase 2) — Tiered pricing + Permit
// ═══════════════════════════════════════════════════════════════

export interface MintPriceInfo {
  handle: string;
  followers: number;
  tier: string;
  price_wei: string;
  price_bnb: number;
  already_minted: boolean;
}

export interface MintPermit {
  handle_hash: string;
  price: string;
  deadline: number;
  nonce: string;
  signature: string;
}

export interface MintPermitResponse {
  permit: MintPermit;
  handle: string;
  followers: number;
  tier: string;
  price_wei: string;
  price_bnb: number;
}

export const mintV2Api = {
  getPrice: (handle: string) =>
    apiFetch<MintPriceInfo>(`/api/shell/mint-price?handle=${encodeURIComponent(handle)}`),

  getPermit: (handle: string, walletAddr: string, signature: string) =>
    apiFetch<MintPermitResponse>("/api/shell/mint-permit", {
      method: "POST",
      body: JSON.stringify({ handle }),
      headers: {
        "X-Wallet-Address": walletAddr,
        "X-Wallet-Signature": signature,
      },
    }),
};

// ═══════════════════════════════════════════════════════════════
// Holder Revenue API (Phase 4)
// ═══════════════════════════════════════════════════════════════

export interface HolderRevenue {
  id: string;
  shell_id: string;
  wallet_addr: string;
  period: string;
  usage_count: number;
  weight: number;
  amount: number;
  tx_hash?: string;
  status: string;
  shell?: Shell;
}

export interface HolderDashboard {
  total_earned: number;
  total_pending: number;
  shells: { handle: string; stage: string; avatar_url: string; current_usage: number }[];
  recent_revenue: HolderRevenue[];
}

export const holderApi = {
  dashboard: () => apiFetch<HolderDashboard>("/api/holder/dashboard"),
  revenue: (period: string) => apiFetch<{ revenues: HolderRevenue[] }>(`/api/holder/revenue/${period}`),
  claim: () => apiFetch<{ amount: number; tx_hash: string; status: string }>("/api/holder/claim", { method: "POST" }),
};

// ═══════════════════════════════════════════════════════════════
// KOL Claim API (Phase 4)
// ═══════════════════════════════════════════════════════════════

export interface ClaimStatusData {
  claimed: boolean;
  claimable: boolean;
  status?: string;
  handle: string;
  kol_wallet?: string;
  verify_code?: string;
  claimed_at?: string;
  transition_end?: string;
  kol_share?: number;
  holder_share?: number;
  in_transition?: boolean;
}

export const kolClaimApi = {
  initiate: (handle: string) =>
    apiFetch<{ claim_id: string; verify_code: string; instruction: string }>(
      "/api/claim/initiate",
      { method: "POST", body: JSON.stringify({ handle }) },
    ),

  verify: (handle: string, tweetId: string) =>
    apiFetch<{ status: string }>("/api/claim/verify", {
      method: "POST",
      body: JSON.stringify({ handle, tweet_id: tweetId }),
    }),

  status: (handle: string) => apiFetch<ClaimStatusData>(`/api/claim/${handle}`),
};

// ═══════════════════════════════════════════════════════════════
// Economy Dashboard API
// ═══════════════════════════════════════════════════════════════

export interface EconomyMiningPool {
  balance: number;
  total_deposited: number;
  total_released: number;
  daily_limit: number;
  daily_released: number;
  daily_remaining: number;
  daily_start_balance: number;
  paused: boolean;
}

export interface EconomyBuybackSummary {
  total_bnb_spent: number;
  total_token_bought: number;
  total_operations: number;
  mint_revenue_bnb: number;
  mint_revenue_token: number;
  sub_revenue_bnb: number;
  sub_revenue_token: number;
}

export interface EconomySplitConfig {
  mint_buyback_pct: number;
  mint_treasury_pct: number;
  mint_revenue_pool_pct: number;
  sub_buyback_pct: number;
  sub_treasury_pct: number;
  sub_revenue_pool_pct: number;
}

export interface EconomyRevenuePool {
  id: string;
  period: string;
  total_revenue: number;
  pool_amount: number;
  distributed: boolean;
  created_at: string;
}

export interface EconomyBuybackRecord {
  id: string;
  source: string;
  bnb_amount: number;
  token_amount: number;
  swap_tx_hash: string;
  created_at: string;
}

export interface EconomyOverview {
  total_souls: number;
  total_fragments: number;
  total_subscribers: number;
  total_mining_payout: number;
  token_info: {
    total_supply: number;
    token_address: string;
  };
  mining_pool: EconomyMiningPool;
  buyback: EconomyBuybackSummary;
  revenue_pools: EconomyRevenuePool[];
  buyback_history: EconomyBuybackRecord[];
  split_config: EconomySplitConfig;
  last_buyback: {
    source: string;
    bnb_amount: number;
    token_amount: number;
    created_at: string;
  } | null;
  wallets: {
    buyback_bnb: number;
    buyback_token: number;
    buyback_addr: string;
    mining_pool_token: number;
    mining_pool_addr: string;
    revenue_pool_token: number;
    revenue_pool_addr: string;
    treasury_addr: string;
  };
}

export const economyApi = {
  overview: () => apiFetch<EconomyOverview>("/api/economy/overview"),
};

// --- Billing API (LemonSqueezy) ---

export interface BillingStatus {
  is_pro: boolean;
  credits: number;
  credits_reset: string;
  plan: "free" | "pro";
  pro_expires_at?: string;
}

export const billingApi = {
  checkout: () =>
    apiFetch<{ url: string }>("/api/billing/checkout", { method: "POST" }),
  status: () =>
    apiFetch<BillingStatus>("/api/billing/status"),
};

// --- Vibe Write 2.0 Workspace API ---

export interface Workspace {
  id: string;
  name: string;
  twitter_handle?: string;
  created_at: string;
  updated_at: string;
}

export interface VibeMemory {
  id: string;
  workspace_id: string;
  category: "profile" | "knowledge" | "network" | "archive" | "rules";
  content: string;
  source: "user" | "ai" | "import";
  status: "pending" | "accepted" | "rejected";
  reason?: string;
  /**
   * Set when a Free user tried to save into a Pro-only category and the
   * backend silently downgraded the entry to `profile`. UI should surface
   * a soft inline hint instead of an upgrade modal.
   */
  downgraded_from?: "knowledge" | "network" | "archive";
  created_at: string;
  updated_at: string;
}

export interface VibeChatItem {
  id: string;
  workspace_id: string;
  title: string;
  created_at: string;
  updated_at: string;
}

export interface VibeChatMsg {
  id: string;
  chat_id: string;
  role: "user" | "assistant";
  content: string;
  credits_cost: number;
  soul_handles?: string[];
  memory_cats?: string[];
  scenario?: string;
  feedback?: number; // -1, 0, 1
  created_at: string;
}

export const workspaceApi = {
  list: () =>
    apiFetch<{ workspaces: Workspace[] }>("/api/vibe-write/workspaces"),

  create: (name: string, twitterHandle?: string) =>
    apiFetch<Workspace>("/api/vibe-write/workspaces", {
      method: "POST",
      body: JSON.stringify({ name, twitter_handle: twitterHandle }),
    }),

  update: (id: string, data: { name?: string; twitter_handle?: string }) =>
    apiFetch<Workspace>(`/api/vibe-write/workspaces/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    }),

  delete: (id: string) =>
    apiFetch<{ message: string }>(`/api/vibe-write/workspaces/${id}`, {
      method: "DELETE",
    }),

  // Twitter-handle setup wizard: fetches recent tweets and distills seed memories.
  setup: (id: string, twitterHandle: string, autoAccept = false) =>
    apiFetch<{
      workspace: Workspace;
      profile_source: string;
      tweets_analyzed: number;
      pending_memories: VibeMemory[];
      status: string;
    }>(`/api/vibe-write/workspaces/${id}/setup`, {
      method: "POST",
      body: JSON.stringify({ twitter_handle: twitterHandle, auto_accept: autoAccept }),
    }),

  // Memories
  listMemories: (wsId: string, status?: "pending" | "accepted" | "rejected") => {
    const qs = status ? `?status=${status}` : "";
    return apiFetch<{ memories: VibeMemory[] }>(`/api/vibe-write/workspaces/${wsId}/memories${qs}`);
  },

  createMemory: (wsId: string, category: string, content: string) =>
    apiFetch<VibeMemory>(`/api/vibe-write/workspaces/${wsId}/memories`, {
      method: "POST",
      body: JSON.stringify({ category, content }),
    }),

  updateMemory: (memId: string, content: string) =>
    apiFetch<VibeMemory>(`/api/vibe-write/memories/${memId}`, {
      method: "PUT",
      body: JSON.stringify({ content }),
    }),

  deleteMemory: (memId: string) =>
    apiFetch<{ message: string }>(`/api/vibe-write/memories/${memId}`, {
      method: "DELETE",
    }),

  reviewMemory: (
    memId: string,
    action: "accept" | "reject",
    content?: string
  ) =>
    apiFetch<VibeMemory>(`/api/vibe-write/memories/${memId}/review`, {
      method: "POST",
      body: JSON.stringify({ action, content }),
    }),

  feedbackMessage: (msgId: string, value: -1 | 0 | 1) =>
    apiFetch<{ ok: boolean; feedback: number }>(
      `/api/vibe-write/messages/${msgId}/feedback`,
      {
        method: "POST",
        body: JSON.stringify({ value }),
      }
    ),

  // Chats
  listChats: (wsId: string) =>
    apiFetch<{ chats: VibeChatItem[] }>(`/api/vibe-write/workspaces/${wsId}/chats`),

  createChat: (wsId: string) =>
    apiFetch<VibeChatItem>(`/api/vibe-write/workspaces/${wsId}/chats`, {
      method: "POST",
    }),

  deleteChat: (chatId: string) =>
    apiFetch<{ message: string }>(`/api/vibe-write/chats/${chatId}`, {
      method: "DELETE",
    }),

  getMessages: (chatId: string, params?: { limit?: number; before?: string }) => {
    const query = new URLSearchParams();
    if (params?.limit) query.set("limit", String(params.limit));
    if (params?.before) query.set("before", params.before);
    const qs = query.toString();
    return apiFetch<{ messages: VibeChatMsg[]; has_more?: boolean }>(
      `/api/vibe-write/chats/${chatId}/messages${qs ? `?${qs}` : ""}`
    );
  },

  sendMessage: (chatId: string, content: string) =>
    apiFetch<{
      user_message: VibeChatMsg;
      assistant_message: VibeChatMsg;
      credits_used: number;
      soul_enhanced?: boolean;
      soul_handles?: string[];
      memory_cats?: string[];
    }>(`/api/vibe-write/chats/${chatId}/messages`, {
      method: "POST",
      body: JSON.stringify({ content }),
    }),

  /**
   * Stream a chat reply via Server-Sent Events.
   * Calls handlers as events arrive. Returns a promise that resolves when the
   * stream ends (done) or rejects on error event / network failure.
   *
   * Events:
   *  - meta:  { user_message_id, scenario, used_memory_cats, soul_handles }
   *  - chunk: token text (string)
   *  - done:  { assistant_message_id, credits_used, total_chars, model }
   *  - error: error message string (terminal)
   */
  sendMessageStream: async (
    chatId: string,
    payload: {
      content: string;
      attached_tweet?: { url?: string; author_handle?: string; text?: string };
      variant_count?: number;
      output_langs?: string[];
      /** Explicit opt-in to use the attached tweet author's Soul. Without
       * this flag (or an `@handle` mention in content), Soul is NOT applied
       * even if the author owns one. */
      use_soul?: boolean;
    } | string,
    handlers: {
      onMeta?: (meta: {
        user_message_id: string;
        scenario: string;
        mode?: "chat" | "reply" | "translate";
      }) => void;
      onContext?: (ctx: {
        used_memory_cats?: string[];
        soul_handles?: string[];
        soul_enhanced?: boolean;
        methodology_slugs?: string[];
        output_langs?: string[];
        variant_count?: number;
      }) => void;
      onChunk?: (text: string) => void;
      onVariant?: (v: {
        idx: number;
        content: string;
        recommended?: boolean;
        reason?: string;
        lang?: string;
      }) => void;
      onMemorySuggest?: (m: {
        id: string;
        category: string;
        content: string;
        reason?: string;
      }) => void;
      onSoulLock?: (info: { handle: string; upgrade?: boolean }) => void;
      onDone?: (info: {
        assistant_message_id: string;
        credits_used: number;
        total_chars: number;
        model: string;
        cleaned_content?: string;
        pending_memories?: VibeMemory[];
        mode?: "chat" | "reply" | "translate";
      }) => void;
      signal?: AbortSignal;
    }
  ): Promise<void> => {
    const body = typeof payload === "string" ? { content: payload } : payload;
    const res = await fetch(
      `${API_BASE}/api/vibe-write/chats/${chatId}/messages/stream`,
      {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
        signal: handlers.signal,
      }
    );
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
      throw new Error(err.error || `HTTP ${res.status}`);
    }
    if (!res.body) throw new Error("response body missing");

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";

    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });

      // SSE messages are separated by \n\n
      let idx;
      while ((idx = buffer.indexOf("\n\n")) >= 0) {
        const raw = buffer.slice(0, idx);
        buffer = buffer.slice(idx + 2);

        let event = "message";
        let dataStr = "";
        for (const line of raw.split("\n")) {
          if (line.startsWith("event:")) event = line.slice(6).trim();
          else if (line.startsWith("data:")) dataStr += line.slice(5).trim();
        }
        if (!dataStr) continue;
        let data: unknown;
        try {
          data = JSON.parse(dataStr);
        } catch {
          data = dataStr;
        }

        if (event === "meta" && handlers.onMeta) {
          handlers.onMeta(data as Parameters<NonNullable<typeof handlers.onMeta>>[0]);
        } else if (event === "context" && handlers.onContext) {
          handlers.onContext(data as Parameters<NonNullable<typeof handlers.onContext>>[0]);
        } else if (event === "chunk" && handlers.onChunk) {
          handlers.onChunk(typeof data === "string" ? data : String(data));
        } else if (event === "variant" && handlers.onVariant) {
          handlers.onVariant(data as Parameters<NonNullable<typeof handlers.onVariant>>[0]);
        } else if (event === "memory_suggest" && handlers.onMemorySuggest) {
          handlers.onMemorySuggest(data as Parameters<NonNullable<typeof handlers.onMemorySuggest>>[0]);
        } else if (event === "soul_lock" && handlers.onSoulLock) {
          handlers.onSoulLock(data as Parameters<NonNullable<typeof handlers.onSoulLock>>[0]);
        } else if (event === "done" && handlers.onDone) {
          handlers.onDone(data as Parameters<NonNullable<typeof handlers.onDone>>[0]);
          return;
        } else if (event === "error") {
          const msg = typeof data === "string" ? data : "stream error";
          throw new Error(msg);
        }
      }
    }
  },
};
