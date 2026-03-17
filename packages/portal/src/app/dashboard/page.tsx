"use client";

import { useEffect, useState } from "react";

interface APIKey {
  id: string;
  env: "test" | "live";
  label: string;
  created_at: string;
  revoked_at: string | null;
}

interface UsageSummary {
  items_active: number;
  api_calls_30d: number;
  webhooks_delivered_30d: number;
}

export default function Dashboard() {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [usage, setUsage] = useState<UsageSummary | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([
      fetch("/api/keys").then((r) => r.json()),
      fetch("/api/usage").then((r) => r.json()),
    ])
      .then(([keysData, usageData]) => {
        setKeys(keysData.keys ?? []);
        setUsage(usageData);
      })
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <div>Loading...</div>;

  return (
    <div>
      <h1>Dashboard</h1>

      {usage && (
        <section aria-label="Usage summary">
          <div>
            <span>{usage.items_active}</span>
            <label>Active connections</label>
          </div>
          <div>
            <span>{usage.api_calls_30d.toLocaleString()}</span>
            <label>API calls (30d)</label>
          </div>
          <div>
            <span>{usage.webhooks_delivered_30d.toLocaleString()}</span>
            <label>Webhooks delivered (30d)</label>
          </div>
        </section>
      )}

      <section aria-label="API keys">
        <h2>API Keys</h2>
        <table>
          <thead>
            <tr>
              <th>Environment</th>
              <th>Label</th>
              <th>Created</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {keys.map((key) => (
              <tr key={key.id}>
                <td>
                  <span data-env={key.env}>{key.env}</span>
                </td>
                <td>{key.label || "—"}</td>
                <td>{new Date(key.created_at).toLocaleDateString()}</td>
                <td>
                  <button>Revoke</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        <button>Create new key</button>
      </section>
    </div>
  );
}
