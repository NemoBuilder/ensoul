// Single source of truth for pricing & quota constants.
// Mirrors server/services/credits.go and server/handlers/vibe_workspace.go.
// When changing values here, update the server constants too.

export const PRICING = {
  pro: {
    priceUSD: 49,
    period: "month" as const,
    credits: 5000,
    workspaces: 10,
    variants: 3,
    autoAccept: true,
    soulBoost: true,
    batchGenerate: true,
  },
  free: {
    priceUSD: 0,
    credits: 50,
    workspaces: 1,
    variants: 1,
    autoAccept: false,
    soulBoost: false,
    batchGenerate: false,
  },
  costs: {
    message: 1,
    longTweet: 3,
    variants3: 3,
    variants5: 5,
    soulContext: 1,
    batchGenerate: 15,
    twitterImport: 5,
    textImport: 5,
  },
} as const;

export type Plan = "free" | "pro";
