"use client";

import { useRef, useEffect } from "react";

interface RadarChartProps {
  dimensions: Record<string, { score: number; summary: string }>;
  size?: number;
}

const LABELS = ["personality", "knowledge", "stance", "style", "relationship", "timeline"];
const DISPLAY_LABELS = ["Personality", "Knowledge", "Stance", "Style", "Relationship", "Timeline"];

export default function RadarChart({ dimensions, size = 280 }: RadarChartProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const dpr = window.devicePixelRatio || 1;
    canvas.width = size * dpr;
    canvas.height = size * dpr;
    canvas.style.width = `${size}px`;
    canvas.style.height = `${size}px`;
    ctx.scale(dpr, dpr);

    const cx = size / 2;
    const cy = size / 2;
    const maxR = size / 2 - 40;
    const n = LABELS.length;
    const angleStep = (2 * Math.PI) / n;
    const startAngle = -Math.PI / 2; // Start from top

    // Clear
    ctx.clearRect(0, 0, size, size);

    // Draw grid rings (20, 40, 60, 80, 100)
    for (let ring = 1; ring <= 5; ring++) {
      const r = (maxR * ring) / 5;
      ctx.beginPath();
      for (let i = 0; i <= n; i++) {
        const angle = startAngle + i * angleStep;
        const x = cx + r * Math.cos(angle);
        const y = cy + r * Math.sin(angle);
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      }
      ctx.closePath();
      ctx.strokeStyle = ring === 5 ? "#2a2a3e" : "#1e1e2e";
      ctx.lineWidth = 1;
      ctx.stroke();
    }

    // Draw axis lines
    for (let i = 0; i < n; i++) {
      const angle = startAngle + i * angleStep;
      ctx.beginPath();
      ctx.moveTo(cx, cy);
      ctx.lineTo(cx + maxR * Math.cos(angle), cy + maxR * Math.sin(angle));
      ctx.strokeStyle = "#1e1e2e";
      ctx.lineWidth = 1;
      ctx.stroke();
    }

    // Draw data polygon
    const values = LABELS.map((key) => {
      const d = dimensions[key];
      return d ? Math.min(d.score, 100) : 0;
    });

    ctx.beginPath();
    for (let i = 0; i <= n; i++) {
      const idx = i % n;
      const angle = startAngle + idx * angleStep;
      const r = (maxR * values[idx]) / 100;
      const x = cx + r * Math.cos(angle);
      const y = cy + r * Math.sin(angle);
      if (i === 0) ctx.moveTo(x, y);
      else ctx.lineTo(x, y);
    }
    ctx.closePath();
    ctx.fillStyle = "rgba(139, 92, 246, 0.15)";
    ctx.fill();
    ctx.strokeStyle = "#8b5cf6";
    ctx.lineWidth = 2;
    ctx.stroke();

    // Draw data points
    for (let i = 0; i < n; i++) {
      const angle = startAngle + i * angleStep;
      const r = (maxR * values[i]) / 100;
      const x = cx + r * Math.cos(angle);
      const y = cy + r * Math.sin(angle);
      ctx.beginPath();
      ctx.arc(x, y, 3, 0, 2 * Math.PI);
      ctx.fillStyle = "#8b5cf6";
      ctx.fill();
    }

    // Draw labels
    ctx.font = "11px Inter, system-ui, sans-serif";
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.fillStyle = "#94a3b8";
    for (let i = 0; i < n; i++) {
      const angle = startAngle + i * angleStep;
      const labelR = maxR + 24;
      const x = cx + labelR * Math.cos(angle);
      const y = cy + labelR * Math.sin(angle);
      ctx.fillText(DISPLAY_LABELS[i], x, y);

      // Draw score under label
      ctx.font = "bold 10px JetBrains Mono, monospace";
      ctx.fillStyle = "#8b5cf6";
      ctx.fillText(String(values[i]), x, y + 14);
      ctx.font = "11px Inter, system-ui, sans-serif";
      ctx.fillStyle = "#94a3b8";
    }
  }, [dimensions, size]);

  return (
    <canvas
      ref={canvasRef}
      className="mx-auto"
      style={{ width: size, height: size }}
    />
  );
}
