import Link from "next/link";
import { ThemeToggle } from "./theme-toggle";

export default function Home() {
  return (
    <main className="mx-auto max-w-4xl px-6 py-16">
      <header className="flex items-center justify-between">
        <div>
          <p className="text-sm uppercase tracking-widest text-ink-muted">
            Vaultside
          </p>
          <h1 className="mt-2 font-display text-5xl text-ink">
            Compliance LMS for financial services
          </h1>
        </div>
        <ThemeToggle />
      </header>

      <p className="mt-6 max-w-xl text-ink-muted">
        Multi-tenant, gamified, audit-ready. Sign in to open your expedition.
      </p>

      <section className="mt-10 flex flex-wrap gap-3">
        <Link
          href="/today"
          className="rounded-full bg-harbour px-6 py-3 text-white transition-transform duration-200 hover:-translate-y-0.5"
        >
          Open Today
        </Link>
        <a
          href="http://localhost:8080/v1/auth/oidc/harbourline/start"
          className="rounded-full border border-ink/20 px-6 py-3 text-ink"
        >
          Sign in with OIDC
        </a>
        <a
          href="http://localhost:8080/v1/auth/saml/harbourline/start"
          className="rounded-full border border-ink/20 px-6 py-3 text-ink"
        >
          Sign in with SAML
        </a>
      </section>
    </main>
  );
}