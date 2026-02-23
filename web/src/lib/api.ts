const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8990";

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
    const error = await res.json().catch(() => ({ error: "Request failed" }));
    throw new Error(error.error || `HTTP ${res.status}`);
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

// --- Claw Key Management API (session-based, no API key in frontend) ---

export interface ClawBindingInfo {
  id: string;
  claw_id: string;
  claw_name: string;
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
      recent_contributions: Fragment[];
    }>(`/api/claw/keys/${bindingId}/dashboard`),
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
  demands: () => apiFetch<FragmentDemand[]>("/api/mining/demands"),
  rewards: (clawId: string) => apiFetch<MiningReward[]>(`/api/mining/rewards/${clawId}`),
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
// Soul Sniper API (Phase 3)
// ═══════════════════════════════════════════════════════════════

export interface Subscription {
  id: string;
  wallet_addr: string;
  tier: string;
  llm_model: string;
  status: string;
  expires_at: string;
  payment_tx_hash: string;
  payment_token: string;
  payment_amount: number;
}

export interface SubscriptionStatus {
  active: boolean;
  tier?: string;
  llm_model?: string;
  expires_at?: string;
  kol_count?: number;
  kol_limit?: number;
  daily_replies?: number;
  daily_limit?: number;
  payment_token?: string;
}

export interface SniperKOL {
  id: string;
  subscription_id: string;
  shell_id: string;
  handle: string;
  shell?: Shell;
}

export interface ReplyVariant {
  style: string;
  content: string;
  model: string;
}

export interface SniperReply {
  id: string;
  shell_id: string;
  wallet_addr: string;
  tweet_id: string;
  tweet_text: string;
  replies: ReplyVariant[];
  created_at: string;
  shell?: Shell;
}

export interface UserPersona {
  id: string;
  wallet_addr: string;
  bio: string;
  style: string;
  materials: string;
  language: string;
}

export const sniperApi = {
  subscribe: (tier: string, paymentTxHash: string, paymentToken = "USDT", paymentAmount = 0) =>
    apiFetch<Subscription>("/api/sniper/subscribe", {
      method: "POST",
      body: JSON.stringify({ tier, payment_tx_hash: paymentTxHash, payment_token: paymentToken, payment_amount: paymentAmount }),
    }),

  getSubscription: () => apiFetch<SubscriptionStatus>("/api/sniper/subscription"),

  addKOL: (handle: string) =>
    apiFetch<SniperKOL>("/api/sniper/kols", {
      method: "POST",
      body: JSON.stringify({ handle }),
    }),

  listKOLs: () => apiFetch<{ kols: SniperKOL[] }>("/api/sniper/kols"),

  removeKOL: (id: string) =>
    apiFetch<{ status: string }>(`/api/sniper/kols/${id}`, { method: "DELETE" }),

  generateReply: (handle: string, tweetId: string, tweetText: string) =>
    apiFetch<SniperReply>("/api/sniper/reply", {
      method: "POST",
      body: JSON.stringify({ handle, tweet_id: tweetId, tweet_text: tweetText }),
    }),

  getReplies: () => apiFetch<{ replies: SniperReply[] }>("/api/sniper/replies"),

  setPersona: (bio: string, style: string, materials: string, language: string) =>
    apiFetch<UserPersona>("/api/sniper/persona", {
      method: "POST",
      body: JSON.stringify({ bio, style, materials, language }),
    }),

  getPersona: () => apiFetch<{ configured: boolean; persona?: UserPersona }>("/api/sniper/persona"),
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
