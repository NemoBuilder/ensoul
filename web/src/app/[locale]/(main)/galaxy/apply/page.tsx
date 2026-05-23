"use client";

// V4 Apply form — submit a GalaxyApplication. The slug becomes the URL.
// A curator (admin) must approve before the Galaxy row is created.

import { useRouter } from "next/navigation";
import { useState } from "react";
import { galaxyApi } from "@/lib/api";

export default function GalaxyApplyPage() {
  const router = useRouter();
  const [slug, setSlug] = useState("");
  const [title, setTitle] = useState("");
  const [pitch, setPitch] = useState("");
  const [category, setCategory] = useState("");
  const [seeds, setSeeds] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const slugValid = /^[a-z0-9][a-z0-9-]{2,62}$/.test(slug);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!slugValid || !title.trim()) return;
    setBusy(true);
    setError(null);
    try {
      const seed_urls = seeds
        .split("\n")
        .map((s) => s.trim())
        .filter(Boolean);
      const r = await galaxyApi.apply({ slug, title, pitch, category, seed_urls });
      router.push(`./apply/submitted?id=${r.application.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Submit failed");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="mx-auto max-w-2xl px-4 pt-24 pb-16">
      <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">Propose a galaxy</h1>
      <p className="mb-8 text-[#94a3b8]">
        Any topic can become a galaxy: a person, a project, a discipline, an event. A curator will
        review your application before it goes live.
      </p>

      <form onSubmit={submit} className="space-y-5">
        <Field label="Slug (URL handle)" hint="lowercase, 3–63 chars, hyphens allowed">
          <input
            value={slug}
            onChange={(e) => setSlug(e.target.value.toLowerCase())}
            placeholder="e.g. vitalik-buterin"
            className="w-full rounded-md border border-[#1e1e2e] bg-[#0a0a14] p-2.5 text-sm text-[#e2e8f0] focus:border-[#8b5cf6] focus:outline-none"
          />
          {slug && !slugValid && (
            <div className="mt-1 text-xs text-red-400">Invalid slug format.</div>
          )}
        </Field>

        <Field label="Title">
          <input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Display name shown on listing"
            className="w-full rounded-md border border-[#1e1e2e] bg-[#0a0a14] p-2.5 text-sm text-[#e2e8f0] focus:border-[#8b5cf6] focus:outline-none"
          />
        </Field>

        <Field label="Category">
          <select
            value={category}
            onChange={(e) => setCategory(e.target.value)}
            className="w-full rounded-md border border-[#1e1e2e] bg-[#0a0a14] p-2.5 text-sm text-[#e2e8f0] focus:border-[#8b5cf6] focus:outline-none"
          >
            <option value="">— choose —</option>
            <option value="person">Person</option>
            <option value="project">Project</option>
            <option value="discipline">Discipline</option>
            <option value="event">Event</option>
            <option value="place">Place</option>
            <option value="other">Other</option>
          </select>
        </Field>

        <Field label="Pitch" hint="Why does this deserve a galaxy?">
          <textarea
            value={pitch}
            onChange={(e) => setPitch(e.target.value)}
            rows={4}
            className="w-full rounded-md border border-[#1e1e2e] bg-[#0a0a14] p-2.5 text-sm text-[#e2e8f0] focus:border-[#8b5cf6] focus:outline-none"
          />
        </Field>

        <Field label="Seed URLs" hint="One per line (optional)">
          <textarea
            value={seeds}
            onChange={(e) => setSeeds(e.target.value)}
            rows={3}
            placeholder="https://…"
            className="w-full rounded-md border border-[#1e1e2e] bg-[#0a0a14] p-2.5 text-sm text-[#e2e8f0] focus:border-[#8b5cf6] focus:outline-none"
          />
        </Field>

        {error && (
          <div className="rounded-md border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">
            {error}
          </div>
        )}

        <button
          type="submit"
          disabled={busy || !slugValid || !title.trim()}
          className="w-full rounded-md bg-[#8b5cf6] py-2.5 text-sm font-medium text-white hover:bg-[#7c3aed] disabled:opacity-50"
        >
          {busy ? "Submitting…" : "Submit application"}
        </button>
      </form>
    </div>
  );
}

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <label className="block">
      <div className="mb-1 flex items-baseline justify-between">
        <span className="text-sm font-medium text-[#e2e8f0]">{label}</span>
        {hint && <span className="text-xs text-[#64748b]">{hint}</span>}
      </div>
      {children}
    </label>
  );
}
