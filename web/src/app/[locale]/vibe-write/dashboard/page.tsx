import { redirect } from "next/navigation";

/**
 * Old Vibe Write Dashboard — redirects to the new Vibe Write feed page.
 * Kept for backwards compatibility with bookmarks / shared links.
 */
export default function VibeWriteDashboardRedirect() {
  redirect("/vibe-write");
}
