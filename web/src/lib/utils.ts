// Stage color mapping utility
export const stageConfig = {
  embryo: {
    color: "#6b7280",
    bgClass: "bg-gray-500/10",
    borderClass: "border-gray-500 border-dashed",
    textClass: "text-gray-400",
    label: "Embryo",
  },
  growing: {
    color: "#3b82f6",
    bgClass: "bg-blue-500/10",
    borderClass: "border-blue-500",
    textClass: "text-blue-400",
    label: "Growing",
  },
  mature: {
    color: "#8b5cf6",
    bgClass: "bg-purple-500/10",
    borderClass: "border-purple-500",
    textClass: "text-purple-400",
    label: "Mature",
  },
  evolving: {
    color: "#f59e0b",
    bgClass: "bg-amber-500/10",
    borderClass: "border-amber-500",
    textClass: "text-amber-400",
    label: "Evolving",
  },
} as const;

export type Stage = keyof typeof stageConfig;

// Dimension labels
export const dimensionLabels: Record<string, string> = {
  personality: "Personality",
  knowledge: "Knowledge",
  stance: "Stance",
  style: "Style",
  relationship: "Relationship",
  timeline: "Timeline",
};

// Format relative time (e.g., "2 minutes ago")
export function timeAgo(dateString: string): string {
  const now = new Date();
  const date = new Date(dateString);
  const seconds = Math.floor((now.getTime() - date.getTime()) / 1000);

  if (seconds < 60) return "just now";
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
  if (seconds < 604800) return `${Math.floor(seconds / 86400)}d ago`;
  return date.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

// Truncate wallet address for display
export function truncateAddr(addr: string): string {
  if (!addr || addr.length < 10) return addr;
  return `${addr.slice(0, 6)}...${addr.slice(-4)}`;
}

// Calculate overall completion percentage from dimensions
export function calcCompletion(dimensions: Record<string, { score: number }>): number {
  const values = Object.values(dimensions);
  if (values.length === 0) return 0;
  const total = values.reduce((sum, d) => sum + (d.score || 0), 0);
  return Math.round(total / values.length);
}

// Format account age from creation date string (e.g., "Since Jun 2009")
export function accountAge(createdAt: string): string {
  if (!createdAt) return "";
  try {
    const d = new Date(createdAt);
    if (isNaN(d.getTime())) return "";
    const months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
    return `Since ${months[d.getMonth()]} ${d.getFullYear()}`;
  } catch {
    return "";
  }
}

// Format large numbers (e.g., 1234567 → "1.2M")
export function formatCount(n: number | undefined): string {
  if (n == null || n === 0) return "0";
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return n.toString();
}
