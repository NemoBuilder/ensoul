"use client";

// V4 Galaxy detail page.
// Public read; logged-in users see the "Upload source" panel which actually
// posts to /api/v4/galaxy/:slug/source (text/markdown variant — web upload
// arrives in Phase 1.x once the fetch step is wired).

import { useEffect, useState, use as usePromise } from "react";
import Link from "next/link";
import { galaxyApi, type Galaxy, type Atom } from "@/lib/api";
import AtomCard from "@/components/AtomCard";

type Props = { params: Promise<{ locale: string; slug: string }> };

export default function GalaxyDetailPage({ params }: Props) {
  const { slug } = usePromise(params);
  const [galaxy, setGalaxy] = useState<Galaxy | null>(null);
  const [atoms, setAtoms] = useState<Atom[]>([]);
  const [tab, setTab] = useState<"node" | "edge">("node");
  const [error, setError] = useState<string | null>(null);

  const refresh = () => {
    galaxyApi.get(slug).then(setGalaxy).catch((e: Error) => setError(e.message));
    galaxyApi.atoms(slug, { kind: tab }).then((r) => setAtoms(r.atoms || []));
  };

  useEffect(refresh, [slug, tab]);

  if (error) {
    return (
      <div className="mx-auto max-w-4xl px-4 pt-24 pb-16">
        <div className="rounded-md border border-red-500/30 bg-red-500/10 p-4 text-red-300">
          {error}
        </div>
        <Link
          href="../galaxy"
          className="mt-4 inline-block text-sm text-[#8b5cf6] hover:underline"
        >
          ← Back to galaxies
        </Link>
      </div>
    );
  }

  if (!galaxy) {
    return <div className="mx-auto max-w-4xl px-4 pt-24 text-[#94a3b8]">Loading…</div>;
  }

  return (
    <div className="mx-auto max-w-5xl px-4 pt-24 pb-16">
      <Link
        href="../galaxy"
        className="mb-6 inline-block text-sm text-[#94a3b8] hover:text-[#8b5cf6]"
      >
        ← All galaxies
      </Link>

      <header className="mb-8">
        <div className="mb-2 flex items-center gap-3">
          <h1 className="text-3xl font-bold text-[#e2e8f0]">{galaxy.title}</h1>
          <span className="rounded bg-[#8b5cf6]/10 px-2 py-0.5 text-xs text-[#8b5cf6]">
            {galaxy.stage}
          </span>
          {(galaxy.stage === "raising" ||
            galaxy.stage === "graduated" ||
            galaxy.stage === "mature") && (
            <Link
              href={`./${slug}/launch`}
              className="rounded border border-[#8b5cf6]/40 px-2 py-0.5 text-xs text-[#8b5cf6] hover:bg-[#8b5cf6]/10"
            >
              公平发射 →
            </Link>
          )}
        </div>
        {galaxy.subtitle && <p className="text-[#94a3b8]">{galaxy.subtitle}</p>}
      </header>

      <section className="mb-8 grid grid-cols-2 gap-3 md:grid-cols-4">
        <Metric label="Atoms" value={galaxy.atom_count} />
        <Metric label="Contributors" value={galaxy.contrib_count} />
        <Metric label="Maturity" value={galaxy.maturity_score.toFixed(1)} suffix="/100" />
        <Metric label="Avg confidence" value={(galaxy.confidence_avg * 100).toFixed(0)} suffix="%" />
      </section>

      <UploadPanel slug={slug} onUploaded={refresh} />

      <section className="mt-10">
        <div className="mb-4 flex gap-2">
          {(["node", "edge"] as const).map((k) => (
            <button
              key={k}
              onClick={() => setTab(k)}
              className={`rounded-md border px-3 py-1.5 text-sm ${
                tab === k
                  ? "border-[#8b5cf6] bg-[#8b5cf6]/10 text-[#8b5cf6]"
                  : "border-[#1e1e2e] text-[#94a3b8]"
              }`}
            >
              {k === "node" ? "Nodes" : "Edges"} ({k === "node" ? galaxy.node_count : galaxy.edge_count})
            </button>
          ))}
        </div>

        {atoms.length === 0 ? (
          <div className="text-[#94a3b8]">No {tab}s yet.</div>
        ) : (
          <ul className="space-y-2">
            {atoms.map((a) => (
              <AtomCard key={a.id} atom={a} onChanged={refresh} />
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

function Metric({ label, value, suffix }: { label: string; value: number | string; suffix?: string }) {
  return (
    <div className="rounded-lg border border-[#1e1e2e] bg-[#0a0a14] p-4">
      <div className="text-2xl font-semibold text-[#e2e8f0]">
        {value}
        {suffix && <span className="ml-0.5 text-sm text-[#64748b]">{suffix}</span>}
      </div>
      <div className="text-xs text-[#64748b]">{label}</div>
    </div>
  );
}

function UploadPanel({ slug, onUploaded }: { slug: string; onUploaded: () => void }) {
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<string | null>(null);

  const submit = async () => {
    if (!text.trim()) return;
    setBusy(true);
    setMsg(null);
    try {
      const r = await galaxyApi.uploadSource(slug, { kind: "text", text });
      setMsg(`Accepted (cost ${r.source.credits_cost} credits). Distillation running in background…`);
      setText("");
      // Give distill a moment; rely on user to refresh.
      setTimeout(onUploaded, 3000);
    } catch (e) {
      setMsg(e instanceof Error ? e.message : "Upload failed");
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="rounded-lg border border-[#1e1e2e] bg-[#0a0a14] p-5">
      <h2 className="mb-3 text-lg font-semibold text-[#e2e8f0]">Contribute a source</h2>
      <p className="mb-3 text-sm text-[#94a3b8]">
        Paste raw text or markdown about this topic. After intake gating, the LLM will distill it
        into nodes and edges (1–5 credits depending on length).
      </p>
      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        rows={6}
        placeholder="Paste source material here…"
        className="w-full rounded-md border border-[#1e1e2e] bg-[#0a0a14] p-3 text-sm text-[#e2e8f0] focus:border-[#8b5cf6] focus:outline-none"
      />
      <div className="mt-3 flex items-center justify-between">
        <button
          onClick={submit}
          disabled={busy || !text.trim()}
          className="rounded-md bg-[#8b5cf6] px-4 py-2 text-sm font-medium text-white hover:bg-[#7c3aed] disabled:opacity-50"
        >
          {busy ? "Uploading…" : "Submit source"}
        </button>
        {msg && <div className="text-xs text-[#94a3b8]">{msg}</div>}
      </div>
    </section>
  );
}
