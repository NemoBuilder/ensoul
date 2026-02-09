"use client";

import "@rainbow-me/rainbowkit/styles.css";

import { type ReactNode } from "react";
import {
  getDefaultConfig,
  RainbowKitProvider,
  darkTheme,
} from "@rainbow-me/rainbowkit";
import { WagmiProvider } from "wagmi";
import { bsc } from "wagmi/chains";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

// Polyfill indexedDB for SSR — WalletConnect accesses it at module scope
if (typeof globalThis.indexedDB === "undefined") {
  // Provide a minimal stub that prevents crashes during build/SSR
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (globalThis as any).indexedDB = {
    open: () => ({
      onupgradeneeded: null,
      onsuccess: null,
      onerror: null,
      result: {
        transaction: () => ({ objectStore: () => ({ get: () => ({}), put: () => ({}) }) }),
        createObjectStore: () => ({}),
      },
    }),
    deleteDatabase: () => ({ onsuccess: null, onerror: null }),
  };
}

const config = getDefaultConfig({
  appName: "Ensoul",
  projectId:
    process.env.NEXT_PUBLIC_WC_PROJECT_ID || "ensoul-dev-placeholder",
  chains: [bsc],
  ssr: true,
});

const queryClient = new QueryClient();

export function Web3Provider({ children }: { children: ReactNode }) {
  return (
    <WagmiProvider config={config}>
      <QueryClientProvider client={queryClient}>
        <RainbowKitProvider
          theme={darkTheme({
            accentColor: "#8b5cf6",
            accentColorForeground: "white",
            borderRadius: "medium",
            fontStack: "system",
            overlayBlur: "small",
          })}
        >
          {children}
        </RainbowKitProvider>
      </QueryClientProvider>
    </WagmiProvider>
  );
}
