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
// Soul Sniper API (Phase 3 → V2)
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
  daily_snipes?: number;
  daily_limit?: number;
  payment_token?: string;
}

export interface SubscribePrice {
  tier: string;
  price_usdt: number;
  treasury: string;
  bnb_price?: number;
  price_bnb?: string;
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
  author_handle: string;
  tag_id: string;
  tweet_url: string;
  used_soul: boolean;
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

// Sniper V2 — Tag & Feed types
export interface SniperTagAccount {
  handle: string;
  display_name: string;
  realtime_priority: boolean;
}

export interface SniperTag {
  id: string;
  name: string;
  name_en: string;
  icon: string;
  category: string;
  description: string;
  is_default: boolean;
  sort_order: number;
  accounts: SniperTagAccount[];
}

export interface TweetCardAuthor {
  handle: string;
  name: string;
  avatar: string;
  verified: boolean;
  followers_count: number;
}

export interface TweetCardStats {
  replies: number;
  retweets: number;
  likes: number;
  views: number;
}

export interface TweetCard {
  id: string;
  text: string;
  author: TweetCardAuthor;
  tags: string[];
  created_at: string;
  stats: TweetCardStats;
  has_media: boolean;
  tweet_url: string;
  has_soul: boolean;
  soul_handle?: string;
}

export interface FeedResult {
  tag_ids: string[];
  tweets: TweetCard[];
  next_cursor: string;
  cached: boolean;
  cache_age_seconds: number;
}

export const sniperApi = {
  // Tags
  getTags: () => apiFetch<{ tags: SniperTag[]; defaults: string[] }>("/api/sniper/tags"),

  // Feed
  getFeed: (tagIds: string[], cursor?: string, count = 20) => {
    const params = new URLSearchParams({ tag_ids: tagIds.join(","), count: String(count) });
    if (cursor) params.set("cursor", cursor);
    return apiFetch<FeedResult>(`/api/sniper/feed?${params.toString()}`);
  },

  feedRefresh: (tagIds: string[]) =>
    apiFetch<{ status: string }>(`/api/sniper/feed/refresh?tag_ids=${tagIds.join(",")}`),

  // User tag preferences
  getUserTags: () => apiFetch<{ tag_ids: string[] }>("/api/sniper/user/tags"),

  updateUserTags: (tagIds: string[]) =>
    apiFetch<{ tag_ids: string[] }>("/api/sniper/user/tags", {
      method: "PUT",
      body: JSON.stringify({ tag_ids: tagIds }),
    }),

  // Mute
  getMuted: () => apiFetch<{ handles: string[] }>("/api/sniper/user/muted"),

  muteAccount: (handle: string) =>
    apiFetch<{ status: string }>("/api/sniper/user/muted", {
      method: "POST",
      body: JSON.stringify({ handle }),
    }),

  unmuteAccount: (handle: string) =>
    apiFetch<{ status: string }>(`/api/sniper/user/muted/${handle}`, { method: "DELETE" }),

  // Snipe
  snipe: (tweetId: string, tweetText: string, authorHandle: string, tagId: string, language = "en") =>
    apiFetch<SniperReply>("/api/sniper/snipe", {
      method: "POST",
      body: JSON.stringify({ tweet_id: tweetId, tweet_text: tweetText, author_handle: authorHandle, tag_id: tagId, language }),
    }),

  // Subscription (kept)
  getSubscribePrice: (tier = "pro") =>
    apiFetch<SubscribePrice>(`/api/sniper/subscribe-price?tier=${tier}`),

  subscribe: (tier: string, paymentTxHash: string, paymentToken = "USDT", paymentAmount = 0) =>
    apiFetch<Subscription>("/api/sniper/subscribe", {
      method: "POST",
      body: JSON.stringify({ tier, payment_tx_hash: paymentTxHash, payment_token: paymentToken, payment_amount: paymentAmount }),
    }),

  getSubscription: () => apiFetch<SubscriptionStatus>("/api/sniper/subscription"),

  getReplies: () => apiFetch<{ replies: SniperReply[] }>("/api/sniper/replies"),

  // Persona (kept)
  setPersona: (bio: string, style: string, materials: string, language: string) =>
    apiFetch<UserPersona>("/api/sniper/persona", {
      method: "POST",
      body: JSON.stringify({ bio, style, materials, language }),
    }),

  getPersona: () => apiFetch<{ configured: boolean; persona?: UserPersona }>("/api/sniper/persona"),

  // Legacy (deprecated)
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
