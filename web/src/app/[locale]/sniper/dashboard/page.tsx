import { redirect } from "next/navigation";

/**
 * Old Sniper Dashboard — redirects to the new Sniper feed page.
 * Kept for backwards compatibility with bookmarks / shared links.
 */
export default function SniperDashboardRedirect() {
  redirect("/sniper");
}
