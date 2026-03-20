import React, { useEffect, useState } from "react";
import styles from "./Link.module.css";

// Deterministic color by first character (same palette as InstitutionSearch)
const ICON_COLORS: Record<string, string> = {
  A: "#6366f1", B: "#3b82f6", C: "#0ea5e9", D: "#14b8a6",
  E: "#10b981", F: "#22c55e", G: "#84cc16", H: "#eab308",
  I: "#f59e0b", J: "#f97316", K: "#ef4444", L: "#ec4899",
  M: "#a855f7", N: "#8b5cf6", O: "#6366f1", P: "#3b82f6",
  Q: "#0ea5e9", R: "#14b8a6", S: "#10b981", T: "#22c55e",
  U: "#84cc16", V: "#eab308", W: "#f59e0b", X: "#f97316",
  Y: "#ef4444", Z: "#ec4899",
};

function iconColor(name: string): string {
  const ch = name.charAt(0).toUpperCase();
  return ICON_COLORS[ch] || "#6b7280";
}

interface Props {
  institution: { id: string; name: string };
  linkToken: string;
  onComplete: (publicToken: string) => void;
  onBack: () => void;
}

export function OAuthRedirect({ institution, linkToken, onComplete, onBack }: Props) {
  const [status, setStatus] = useState<"idle" | "redirecting" | "waiting" | "error">("idle");

  useEffect(() => {
    // Check if we're returning from an OAuth redirect (code + state in URL)
    const params = new URLSearchParams(window.location.search);
    const code = params.get("code");
    const state = params.get("state");

    if (code && state) {
      setStatus("waiting");
      fetch("/link/oauth/callback?link_token=" + encodeURIComponent(linkToken), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ code, state }),
      })
        .then((r) => r.json())
        .then((data) => {
          if (data.public_token) {
            onComplete(data.public_token);
          } else {
            setStatus("error");
          }
        })
        .catch(() => setStatus("error"));
    }
  }, []);

  function handleConnect() {
    setStatus("redirecting");
    fetch("/link/oauth/initiate?link_token=" + encodeURIComponent(linkToken), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ institution_id: institution.id }),
    })
      .then((r) => r.json())
      .then((data) => {
        if (data.oauth_url) {
          window.location.href = data.oauth_url;
        } else {
          setStatus("error");
        }
      })
      .catch(() => setStatus("error"));
  }

  if (status === "waiting" || status === "redirecting") {
    return (
      <div className={styles.connectingWrapper} aria-live="polite">
        <div className={styles.spinner} aria-hidden="true" />
        <p className={styles.connectingTitle}>
          {status === "waiting" ? "Completing connection..." : "Redirecting to your bank..."}
        </p>
        <p className={styles.connectingSubtitle}>
          {status === "waiting"
            ? "Please wait while we finish linking your account."
            : "You'll be redirected back here automatically."}
        </p>
      </div>
    );
  }

  if (status === "error") {
    return (
      <div className={styles.errorWrapper}>
        <div className={styles.errorIcon}>
          <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#dc2626" strokeWidth="2" aria-hidden="true">
            <circle cx="12" cy="12" r="10" />
            <path d="M12 8v4M12 16h.01" strokeLinecap="round" />
          </svg>
        </div>
        <p className={styles.errorTitle}>Connection failed</p>
        <p className={styles.errorSubtitle}>
          Something went wrong while connecting to {institution.name}. Please try again.
        </p>
        <button className={styles.connectButton} onClick={onBack}>
          Go back
        </button>
      </div>
    );
  }

  return (
    <div className={styles.connectWrapper}>
      <div
        className={styles.connectIconCircle}
        style={{ background: iconColor(institution.name) }}
        aria-hidden="true"
      >
        {institution.name.charAt(0)}
      </div>

      <h2 className={styles.connectTitle}>Connect to {institution.name}</h2>
      <p className={styles.connectSubtitle}>
        You'll be securely redirected to {institution.name} to authorize access.
        Hound never sees your login credentials.
      </p>

      <button
        className={styles.connectButton}
        onClick={handleConnect}
        disabled={status !== "idle" && status !== "error"}
      >
        Continue to {institution.name} →
      </button>

      <button className={styles.backButton} onClick={onBack}>
        Choose a different bank
      </button>

      <p className={styles.connectPrivacyNote}>
        Your credentials are entered directly on {institution.name}&rsquo;s website
        and are never shared with Hound.
      </p>
    </div>
  );
}
