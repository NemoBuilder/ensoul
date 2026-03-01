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
  status: "pending" | "minted" | "skipped" | "failed";
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
