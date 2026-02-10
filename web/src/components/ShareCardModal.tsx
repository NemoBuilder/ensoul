"use client";

import { useRef, useState, useCallback, useEffect } from "react";
import html2canvas from "html2canvas-pro";

interface ShareCardMessage {
  role: "user" | "assistant";
  content: string;
}

interface ShareCardModalProps {
  handle: string;
  avatarUrl?: string;
  stage: string;
  stageColor: string;
  dnaVersion: number;
  messages: ShareCardMessage[]; // The Q&A pair(s) to show
  shareUrl?: string; // e.g. ensoul.ac/s/Xk9m
  onClose: () => void;
  // i18n labels
  labels: {
    title: string;
    download: string;
    shareTwitter: string;
    copied: string;
    generating: string;
  };
}

// Truncate long text for card display
function truncateText(text: string, maxLen: number): string {
  if (text.length <= maxLen) return text;
  return text.slice(0, maxLen - 1).trim() + "…";
}

export default function ShareCardModal({
  handle,
  avatarUrl,
  stage,
  stageColor,
  dnaVersion,
  messages,
  shareUrl,
  onClose,
  labels,
}: ShareCardModalProps) {
  const cardRef = useRef<HTMLDivElement>(null);
  const [generating, setGenerating] = useState(false);
  const [imageUrl, setImageUrl] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  // Close on Escape key
  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  }, [onClose]);

  // Generate card image from the hidden card element
  const generateCard = useCallback(async () => {
    if (!cardRef.current) return;
    setGenerating(true);
    try {
      const canvas = await html2canvas(cardRef.current, {
        backgroundColor: "#0a0a0f",
        scale: 2,
        useCORS: true,
        logging: false,
      });
      const url = canvas.toDataURL("image/png");
      setImageUrl(url);
    } catch (err) {
      console.error("Failed to generate card:", err);
    } finally {
      setGenerating(false);
    }
  }, []); // No dependencies — stable reference

  // Auto-generate once on mount
  useEffect(() => {
    const timer = setTimeout(() => generateCard(), 200);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []); // Run only once on mount

  // Download image
  function downloadImage() {
    if (!imageUrl) return;
    const link = document.createElement("a");
    link.download = `ensoul-${handle}-${Date.now()}.png`;
    link.href = imageUrl;
    link.click();
  }

  // Copy image to clipboard
  async function copyImage() {
    if (!imageUrl) return;
    try {
      const res = await fetch(imageUrl);
      const blob = await res.blob();
      await navigator.clipboard.write([
        new ClipboardItem({ "image/png": blob }),
      ]);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback: download instead
      downloadImage();
    }
  }

  // Take only the last Q&A pair for single-message cards, or up to 2 pairs
  const displayMessages = messages.slice(-4);

  // Stage emoji
  const stageEmoji = stage === "embryo" ? "🥚" : stage === "growing" ? "🌱" : stage === "mature" ? "🧠" : "⚡";

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="relative mx-4 flex max-h-[90vh] w-full max-w-lg flex-col rounded-xl border border-[#1e1e2e] bg-[#14141f] shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-[#1e1e2e] px-5 py-3">
          <h3 className="text-sm font-medium text-[#e2e8f0]">{labels.title}</h3>
          <button
            onClick={onClose}
            className="text-[#94a3b8] transition-colors hover:text-[#e2e8f0]"
          >
            ✕
          </button>
        </div>

        {/* Card preview area */}
        <div className="flex-1 overflow-y-auto p-5">
          {/* Rendered preview of the generated image */}
          {imageUrl ? (
            <img
              src={imageUrl}
              alt="Share card"
              className="mx-auto max-w-full rounded-lg shadow-lg"
            />
          ) : (
            <div className="flex items-center justify-center py-16">
              <div className="text-center">
                <div className="mb-2 text-2xl animate-pulse">🎨</div>
                <p className="text-sm text-[#94a3b8]">{labels.generating}</p>
              </div>
            </div>
          )}
        </div>

        {/* Action buttons */}
        <div className="flex items-center gap-2 border-t border-[#1e1e2e] px-5 py-3">
          <button
            onClick={downloadImage}
            disabled={!imageUrl || generating}
            className="flex-1 rounded-lg bg-[#8b5cf6] py-2 text-sm font-medium text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-40"
          >
            📥 {labels.download}
          </button>
          <button
            onClick={copyImage}
            disabled={!imageUrl || generating}
            className="flex-1 rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] py-2 text-sm font-medium text-[#e2e8f0] transition-colors hover:bg-[#1e1e2e] disabled:opacity-40"
          >
            {copied ? `✅ ${labels.copied}` : "📋 Copy"}
          </button>
          {shareUrl && (
            <button
              onClick={() => {
                const text = `I just had an amazing conversation with @${handle}'s digital soul on @ensoul_ac! 🧠`;
                const twitterUrl = `https://x.com/intent/tweet?text=${encodeURIComponent(text)}&url=${encodeURIComponent(shareUrl)}`;
                window.open(twitterUrl, "_blank");
              }}
              disabled={!imageUrl || generating}
              className="rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm font-medium text-[#e2e8f0] transition-colors hover:bg-[#1e1e2e] disabled:opacity-40"
            >
              𝕏
            </button>
          )}
        </div>
      </div>

      {/* ===== Hidden card element for html2canvas capture ===== */}
      <div
        style={{
          position: "fixed",
          left: "-9999px",
          top: "-9999px",
          width: "480px",
        }}
      >
        <div
          ref={cardRef}
          style={{
            width: "480px",
            padding: "32px",
            backgroundColor: "#0a0a0f",
            fontFamily: "'Inter', 'Segoe UI', system-ui, sans-serif",
            color: "#e2e8f0",
            borderRadius: "16px",
            border: "1px solid #1e1e2e",
          }}
        >
          {/* Card header: brand + identity */}
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: "24px" }}>
            <div style={{ display: "flex", alignItems: "center", gap: "10px" }}>
              <span style={{ fontSize: "24px" }}>🧠</span>
              <span style={{ fontSize: "18px", fontWeight: 700, color: "#8b5cf6" }}>Ensoul</span>
            </div>
            <span style={{ fontSize: "12px", color: "#94a3b8" }}>ensoul.ac</span>
          </div>

          {/* Soul identity */}
          <div style={{ display: "flex", alignItems: "center", gap: "12px", marginBottom: "20px" }}>
            {avatarUrl ? (
              <img
                src={avatarUrl}
                alt={handle}
                style={{ width: "48px", height: "48px", borderRadius: "50%", border: "2px solid #1e1e2e" }}
                crossOrigin="anonymous"
              />
            ) : (
              <div style={{ width: "48px", height: "48px", borderRadius: "50%", backgroundColor: "#1e1e2e", display: "flex", alignItems: "center", justifyContent: "center", fontSize: "20px" }}>
                👤
              </div>
            )}
            <div>
              <div style={{ fontSize: "16px", fontWeight: 600 }}>@{handle}</div>
              <div style={{ fontSize: "12px", color: stageColor, marginTop: "2px" }}>
                {stageEmoji} {stage.charAt(0).toUpperCase() + stage.slice(1)} · DNA v{dnaVersion}
              </div>
            </div>
          </div>

          {/* Divider */}
          <div style={{ height: "1px", backgroundColor: "#1e1e2e", margin: "0 0 20px 0" }} />

          {/* Messages */}
          <div style={{ display: "flex", flexDirection: "column", gap: "12px", marginBottom: "24px" }}>
            {displayMessages.map((msg, i) => (
              <div key={i}>
                {msg.role === "user" ? (
                  <div style={{ display: "flex", justifyContent: "flex-end" }}>
                    <div
                      style={{
                        maxWidth: "85%",
                        backgroundColor: "#8b5cf6",
                        color: "#ffffff",
                        borderRadius: "16px 16px 4px 16px",
                        padding: "10px 14px",
                        fontSize: "13px",
                        lineHeight: "1.5",
                      }}
                    >
                      {truncateText(msg.content, 200)}
                    </div>
                  </div>
                ) : (
                  <div style={{ display: "flex", justifyContent: "flex-start" }}>
                    <div
                      style={{
                        maxWidth: "85%",
                        backgroundColor: "#14141f",
                        border: "1px solid #1e1e2e",
                        borderRadius: "16px 16px 16px 4px",
                        padding: "10px 14px",
                        fontSize: "13px",
                        lineHeight: "1.5",
                      }}
                    >
                      {truncateText(msg.content, 400)}
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>

          {/* Footer with share URL */}
          <div style={{ height: "1px", backgroundColor: "#1e1e2e", margin: "0 0 16px 0" }} />
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <span style={{ fontSize: "11px", color: "#94a3b8" }}>
              {shareUrl ? shareUrl.replace(/^https?:\/\//, "") : "ensoul.ac"}
            </span>
            <span style={{ fontSize: "11px", color: "#94a3b8" }}>
              Souls aren&apos;t born. They&apos;re built.
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
