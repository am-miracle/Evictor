import { productName } from "@/lib/product";

export default function Home() {
  return (
    <main className="grid min-h-screen place-items-center px-6">
      <section className="max-w-xl text-center">
        <p className="mb-4 text-sm font-medium tracking-[0.3em] text-emerald-400 uppercase">
          Phase 1
        </p>
        <h1 className="text-5xl font-semibold tracking-tight">{productName}</h1>
        <p className="mt-5 text-lg leading-8 text-zinc-400">
          Observability and control for AI inference workloads running on serverless GPUs.
        </p>
        <p className="mt-8 rounded-lg border border-zinc-800 bg-zinc-900/70 px-4 py-3 text-sm text-zinc-300">
          Dashboard setup is ready. Connect an endpoint in the next milestone.
        </p>
      </section>
    </main>
  );
}
