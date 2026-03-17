import Link from "next/link";

export default function Home() {
  return (
    <main>
      <section>
        <h1>The financial data API built for developers</h1>
        <p>Connect bank accounts to your product in minutes. More reliable and more affordable than the alternative.</p>
        <Link href="/signup">Get API keys</Link>
        <Link href="/docs">Read the docs</Link>
      </section>
    </main>
  );
}
