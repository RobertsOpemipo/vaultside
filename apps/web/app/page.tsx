import { ThemeToggle } from "./theme-toggle";

const swatches = [
  { name: "ink",          hex: "#1F2430", cls: "bg-[#1F2430] text-[#F6F1E7]" },
  { name: "ink-muted",    hex: "#4A5262", cls: "bg-[#4A5262] text-[#F6F1E7]" },
  { name: "paper",        hex: "#F6F1E7", cls: "bg-[#F6F1E7] text-[#1F2430]" },
  { name: "card",         hex: "#FFFDF8", cls: "bg-[#FFFDF8] text-[#1F2430]" },
  { name: "harbour",      hex: "#0F5C5A", cls: "bg-[#0F5C5A] text-white" },
  { name: "marigold",     hex: "#E8A317", cls: "bg-[#E8A317] text-[#1F2430]" },
  { name: "vermilion",    hex: "#B8381F", cls: "bg-[#B8381F] text-white" },
  { name: "moss",         hex: "#1E6B43", cls: "bg-[#1E6B43] text-white" },
  { name: "cobalt",       hex: "#2F4BA0", cls: "bg-[#2F4BA0] text-white" },
];

export default function Home() {
  return (
    <main className="mx-auto max-w-4xl px-6 py-16">
      <header className="flex items-center justify-between">
        <div>
          <p className="text-sm uppercase tracking-widest text-ink-muted">Expedition Atlas</p>
          <h1 className="mt-2 font-display text-5xl">Vaultside design tokens</h1>
        </div>
        <ThemeToggle />
      </header>

      <p className="mt-6 max-w-xl text-ink-muted">
        Every colour below is served from <code className="font-mono">@vaultside/design-tokens</code>.
        Toggle dark mode to confirm the token overrides cascade.
      </p>

      <section className="mt-10 grid grid-cols-2 gap-4 sm:grid-cols-3">
        {swatches.map((s) => (
          <div key={s.name} className={`rounded-[var(--radius-card)] p-6 shadow-[var(--shadow-paper)] ${s.cls}`}>
            <p className="font-display text-2xl">{s.name}</p>
            <p className="mt-1 font-mono text-sm opacity-80">{s.hex}</p>
          </div>
        ))}
      </section>

      <section className="mt-12 rounded-[var(--radius-card)] bg-card p-8 shadow-[var(--shadow-paper)]">
        <h2 className="font-display text-3xl">Motion + ease</h2>
        <p className="mt-2 text-ink-muted">
          Shared easing <code className="font-mono">cubic-bezier(.22, 1, .36, 1)</code>,
          durations 140 / 260 / 520 ms.
        </p>
        <button
          className="mt-6 rounded-full bg-harbour px-6 py-3 text-white
                     transition-transform duration-200 hover:-translate-y-0.5
                     focus-visible:outline focus-visible:outline-3 focus-visible:outline-harbour focus-visible:outline-offset-4"
        >
          Continue
        </button>
      </section>
    </main>
  );
}