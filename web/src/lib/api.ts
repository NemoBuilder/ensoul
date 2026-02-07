const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

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
  created_at: string;
  updated_at: string;
}

export interface Fragment {
  id: string;
  shell_id: string;
  claw_id: string;
  dimension: string;
  content: string;
  status: "pending" | "accepted" | "rejected";
  confidence: number;
  reject_reason?: string;
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

export const shellApi = {
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

  confirm: (handle: string, txHash: string, agentId?: number) =>
    apiFetch<{ status: string }>("/api/shell/confirm", {
      method: "POST",
      body: JSON.stringify({ handle, tx_hash: txHash, agent_id: agentId ?? 0 }),
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
};

// --- Chat API ---

export const chatApi = {
  // Returns an EventSource for streaming chat responses
  sendMessage: (handle: string, message: string) => {
    const url = `${API_BASE}/api/chat/${handle}`;
    return fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ message }),
    });
  },
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
