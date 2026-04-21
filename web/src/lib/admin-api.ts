const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8990";

// Admin-specific fetch wrapper — always sends cookies
async function adminFetch<T>(
  path: string,
  options?: RequestInit
): Promise<T> {
  const url = `${API_BASE}${path}`;
  const res = await fetch(url, {
    credentials: "include",
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: "Request failed" }));
    throw new Error(body.error || `HTTP ${res.status}`);
  }

  return res.json();
}

// ── Types ──────────────────────────────────────────────────────

export interface AdminUser {
  id: string;
  username: string;
  role: string;
  last_login_at: string | null;
  created_at: string;
}

export interface MintCandidate {
  id: string;
  handle: string;
  followers: number;
  price_wei: string;
  tier: string;
  priority: number;
  reason: string;
  status: "pending" | "queued" | "minted" | "skipped" | "failed";
  error_msg?: string;
  added_by: string;
  created_at: string;
  updated_at: string;
}

export interface TaxWalletStatus {
  balance_wei: string;
  candidates: {
    pending: number;
    minted: number;
    failed: number;
  };
}

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

export interface FailedMiningReward {
  id: string;
  claw_id: string;
  fragment_id: string;
  amount: number;
  tx_hash: string;
  status: string;
  retry_count: number;
  last_error: string;
  last_attempt_at: string | null;
  created_at: string;
  claw?: { id: string; name: string; wallet_addr: string };
}

// ── Auth API ───────────────────────────────────────────────────

export const adminAuthApi = {
  login: (username: string, password: string) =>
    adminFetch<{ username: string; role: string; message: string }>(
      "/api/admin/auth/login",
      {
        method: "POST",
        body: JSON.stringify({ username, password }),
      }
    ),

  logout: () =>
    adminFetch<{ message: string }>("/api/admin/auth/logout", {
      method: "POST",
    }),

  me: () => adminFetch<AdminUser>("/api/admin/auth/me"),

  changePassword: (oldPassword: string, newPassword: string) =>
    adminFetch<{ message: string }>("/api/admin/auth/password", {
      method: "POST",
      body: JSON.stringify({
        old_password: oldPassword,
        new_password: newPassword,
      }),
    }),
};

// ── Candidates API ─────────────────────────────────────────────

export const adminCandidatesApi = {
  list: (status?: string) => {
    const query = status ? `?status=${status}` : "";
    return adminFetch<{ candidates: MintCandidate[]; total: number }>(
      `/api/admin/candidates${query}`
    );
  },

  add: (handle: string, priority = 0, reason = "") =>
    adminFetch<MintCandidate>("/api/admin/candidates", {
      method: "POST",
      body: JSON.stringify({ handle, priority, reason }),
    }),

  addBatch: (handles: string[], priority = 0, reason = "") =>
    adminFetch<{ added: number; skipped: number; errors: string[] }>(
      "/api/admin/candidates/batch",
      {
        method: "POST",
        body: JSON.stringify({ handles, priority, reason }),
      }
    ),

  remove: (handle: string) =>
    adminFetch<{ status: string; handle: string }>(
      `/api/admin/candidates/${handle}`,
      { method: "DELETE" }
    ),

  refreshFollowers: (handle: string) =>
    adminFetch<MintCandidate>(
      `/api/admin/candidates/${handle}/refresh`,
      { method: "POST" }
    ),

  refreshAll: () =>
    adminFetch<{ updated: number; errors: string[] }>(
      "/api/admin/candidates/refresh-all",
      { method: "POST" }
    ),

  importFollowing: (handle: string, maxUsers = 500, minFollowers = 10000, priority = 0, reason = "") =>
    adminFetch<{
      source_handle: string;
      total_following: number;
      fetched: number;
      added: number;
      skipped: number;
      filtered_out: number;
      errors: string[];
      api_calls_used: number;
    }>("/api/admin/candidates/import-following", {
      method: "POST",
      body: JSON.stringify({ handle, max_users: maxUsers, min_followers: minFollowers, priority, reason }),
    }),
};

// ── Tax Wallet API ─────────────────────────────────────────────

export const adminTaxWalletApi = {
  status: () => adminFetch<TaxWalletStatus>("/api/admin/tax-wallet/status"),

  triggerMint: () =>
    adminFetch<{ status: string; message: string }>(
      "/api/admin/tax-wallet/mint",
      { method: "POST" }
    ),

  mintSingle: (handle: string) =>
    adminFetch<{ status: string; handle: string; message: string }>(
      `/api/admin/tax-wallet/mint/${handle}`,
      { method: "POST" }
    ),
};

// ── Mining API (uses public endpoint for status) ───────────────

export const adminMiningApi = {
  pool: () => adminFetch<MiningPoolStatus>("/api/mining/pool"),

  deposit: (amount: number) =>
    adminFetch<{ message: string }>("/api/admin/mining/deposit", {
      method: "POST",
      body: JSON.stringify({ amount }),
    }),

  failedRewards: () =>
    adminFetch<{ rewards: FailedMiningReward[]; total: number; max_retries: number }>(
      "/api/admin/mining/rewards/failed"
    ),

  retryReward: (rewardId: string) =>
    adminFetch<{ message: string; reward_id: string }>(
      `/api/admin/mining/rewards/${rewardId}/retry`,
      { method: "POST" }
    ),

  retryAll: () =>
    adminFetch<{ message: string; retried: number }>(
      "/api/admin/mining/rewards/retry-all",
      { method: "POST" }
    ),
};

// ── User Management Types ──────────────────────────────────────

export interface AdminUserListItem {
  wallet_addr: string;
  status: "active" | "banned";
  first_seen_at: string;
  last_seen_at: string;
  login_count: number;
  note: string;
  ban_reason?: string;
  banned_at?: string;
  sub_tier: string | null;
  sub_status: string | null;
  sub_expires_at: string | null;
  snipe_count: number;
}

export interface AdminUserDetailResponse {
  user: {
    id: string;
    wallet_addr: string;
    status: "active" | "banned";
    ban_reason: string;
    banned_at: string | null;
    banned_by: string;
    note: string;
    first_seen_at: string;
    last_seen_at: string;
    login_count: number;
  };
  subscription: {
    id: string;
    wallet_addr: string;
    tier: string;
    llm_model: string;
    status: string;
    expires_at: string;
    payment_tx_hash: string;
    payment_token: string;
    payment_amount: number;
    created_at: string;
  } | null;
  subscription_history: Array<{
    id: string;
    tier: string;
    status: string;
    expires_at: string;
    payment_tx_hash: string;
    payment_token: string;
    payment_amount: number;
    created_at: string;
  }>;
  persona: {
    bio: string;
    style: string;
    materials: string;
    language: string;
  } | null;
  selected_tags: string[];
  muted_accounts: string[];
  stats: {
    total_snipes: number;
    today_snipes: number;
    total_chats: number;
    shells_owned: number;
    claws_bound: number;
    total_withdrawals: number;
  };
}

export interface AdminUserOverviewStats {
  total_users: number;
  active_users: number;
  banned_users: number;
  pro_subscribers: number;
  free_users: number;
  today_new_users: number;
  today_active_users: number;
  weekly_active_users: number;
}

// ── User Management API ────────────────────────────────────────

export const adminUserApi = {
  list: (params: {
    page?: number;
    page_size?: number;
    search?: string;
    status?: string;
    subscription?: string;
    sort?: string;
    order?: string;
  }) => {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([k, v]) => {
      if (v !== undefined && v !== "") qs.set(k, String(v));
    });
    return adminFetch<{ items: AdminUserListItem[]; total: number; page: number; page_size: number }>(
      `/api/admin/users?${qs}`
    );
  },

  detail: (wallet: string) =>
    adminFetch<AdminUserDetailResponse>(`/api/admin/users/${wallet}`),

  ban: (wallet: string, reason: string) =>
    adminFetch<{ status: string; wallet_addr: string }>(
      `/api/admin/users/${wallet}/ban`,
      { method: "POST", body: JSON.stringify({ reason }) }
    ),

  unban: (wallet: string) =>
    adminFetch<{ status: string; wallet_addr: string }>(
      `/api/admin/users/${wallet}/unban`,
      { method: "POST" }
    ),

  updateNote: (wallet: string, note: string) =>
    adminFetch<{ status: string; wallet_addr: string }>(
      `/api/admin/users/${wallet}/note`,
      { method: "PUT", body: JSON.stringify({ note }) }
    ),

  grantSubscription: (wallet: string, tier: string, days: number, reason: string) =>
    adminFetch<{ status: string; wallet_addr: string; tier: string; days: number }>(
      `/api/admin/users/${wallet}/subscription/grant`,
      { method: "POST", body: JSON.stringify({ tier, days, reason }) }
    ),

  extendSubscription: (wallet: string, days: number, reason: string) =>
    adminFetch<{ status: string; wallet_addr: string; days: number }>(
      `/api/admin/users/${wallet}/subscription/extend`,
      { method: "POST", body: JSON.stringify({ days, reason }) }
    ),

  revokeSubscription: (wallet: string, reason: string) =>
    adminFetch<{ status: string; wallet_addr: string }>(
      `/api/admin/users/${wallet}/subscription/revoke`,
      { method: "POST", body: JSON.stringify({ reason }) }
    ),

  stats: () =>
    adminFetch<AdminUserOverviewStats>(`/api/admin/users/stats`),
};

// ── Claw Management Types ──────────────────────────────────────

export interface AdminClawListItem {
  id: string;
  name: string;
  description: string;
  status: "pending_claim" | "claimed";
  mining_approved: boolean;
  wallet_addr: string;
  twitter_handle: string;
  total_submitted: number;
  total_accepted: number;
  earnings: number;
  withdrawn: number;
  created_at: string;
}

export interface AdminClawStats {
  total_claws: number;
  claimed_claws: number;
  approved_claws: number;
  pending_approval: number;
  total_submitted: number;
  total_accepted: number;
}

// ── Claw Management API ────────────────────────────────────────

export const adminClawApi = {
  stats: () =>
    adminFetch<AdminClawStats>("/api/admin/claws/stats"),

  list: (params: {
    page?: number;
    page_size?: number;
    search?: string;
    status?: string;
    mining_approved?: string;
    sort?: string;
    order?: string;
  }) => {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([k, v]) => {
      if (v !== undefined && v !== "") qs.set(k, String(v));
    });
    return adminFetch<{ items: AdminClawListItem[]; total: number; page: number; page_size: number }>(
      `/api/admin/claws?${qs}`
    );
  },

  approve: (clawId: string) =>
    adminFetch<{ status: string; claw_id: string; name: string }>(
      `/api/admin/claws/${clawId}/approve`,
      { method: "POST" }
    ),

  reject: (clawId: string) =>
    adminFetch<{ status: string; claw_id: string; name: string }>(
      `/api/admin/claws/${clawId}/reject`,
      { method: "POST" }
    ),

  batchApprove: (clawIds: string[]) =>
    adminFetch<{ approved: number; errors: string[] }>(
      "/api/admin/claws/batch-approve",
      {
        method: "POST",
        body: JSON.stringify({ claw_ids: clawIds }),
      }
    ),
};

// ── Audit Log API ──────────────────────────────────────────────

export interface AdminAuditLogItem {
  id: string;
  admin_user_id: string;
  admin_name: string;
  action: string;
  target_type: string;
  target_id: string;
  detail: Record<string, unknown>;
  ip: string;
  created_at: string;
}

export const adminAuditApi = {
  list: (params: {
    page?: number;
    page_size?: number;
    action?: string;
    target_id?: string;
  }) => {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([k, v]) => {
      if (v !== undefined && v !== "") qs.set(k, String(v));
    });
    return adminFetch<{ items: AdminAuditLogItem[]; total: number; page: number }>(
      `/api/admin/audit-log?${qs}`
    );
  },
};

// ── Methodology API ────────────────────────────────────────────

export interface MentorMethodology {
  id: string;
  category: "reference" | "mental_model" | "heuristic" | "routing";
  slug: string;
  locale: string;
  title: string;
  summary: string;
  body_md: string;
  tags: string;
  source: string;
  version: string;
  enabled: boolean;
  priority: number;
  created_at: string;
  updated_at: string;
}

export interface MethodologyStat {
  category: string;
  source: string;
  n: number;
}

export interface MethodologyListResponse {
  records: MentorMethodology[];
  total: number;
  stats: MethodologyStat[];
}

export interface MethodologyWriteReq {
  category: string;
  slug: string;
  locale?: string;
  title: string;
  summary?: string;
  body_md: string;
  tags?: string;
  priority?: number;
  enabled?: boolean;
}

export interface MethodologyPreviewResponse {
  scenario: string;
  used_slugs: string[];
  heuristics: number;
  references: number;
  mental_models: number;
  prompt_chars: number;
  prompt: string;
}

export const adminMethodologyApi = {
  list: (params: {
    category?: string;
    source?: string;
    locale?: string;
    enabled?: string;
    q?: string;
  } = {}) => {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([k, v]) => {
      if (v !== undefined && v !== "") qs.set(k, String(v));
    });
    return adminFetch<MethodologyListResponse>(`/api/admin/methodology?${qs}`);
  },
  get: (id: string) =>
    adminFetch<MentorMethodology>(`/api/admin/methodology/${id}`),
  create: (data: MethodologyWriteReq) =>
    adminFetch<MentorMethodology>(`/api/admin/methodology`, {
      method: "POST",
      body: JSON.stringify(data),
    }),
  update: (id: string, data: MethodologyWriteReq, force = false) =>
    adminFetch<MentorMethodology>(
      `/api/admin/methodology/${id}${force ? "?force=true" : ""}`,
      {
        method: "PUT",
        body: JSON.stringify(data),
      }
    ),
  delete: (id: string, hard = false) =>
    adminFetch<{ ok: boolean; hard: boolean; id: string }>(
      `/api/admin/methodology/${id}${hard ? "?hard=true" : ""}`,
      { method: "DELETE" }
    ),
  preview: (message: string) =>
    adminFetch<MethodologyPreviewResponse>(`/api/admin/methodology/preview`, {
      method: "POST",
      body: JSON.stringify({ message }),
    }),
  feedback: () =>
    adminFetch<{
      window_days: number;
      rows: { scenario: string; up: number; down: number; total: number }[];
    }>(`/api/admin/methodology/feedback`),
};
