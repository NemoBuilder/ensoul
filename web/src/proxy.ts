import createMiddleware from "next-intl/middleware";
import { routing } from "./i18n/routing";

export default createMiddleware(routing);

export const config = {
  // Match all pathnames except:
  // - /api, /trpc, /_next, /_vercel
  // - files with dots (e.g. favicon.ico, logo.png)
  matcher: "/((?!api|trpc|_next|_vercel|.*\\..*).*)",
};
