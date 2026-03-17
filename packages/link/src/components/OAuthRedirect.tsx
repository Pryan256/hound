import React, { useEffect, useState } from "react";

interface Props {
  institution: { id: string; name: string };
  linkToken: string;
  onComplete: (publicToken: string) => void;
  onBack: () => void;
}

export function OAuthRedirect({ institution, linkToken, onComplete, onBack }: Props) {
  const [status, setStatus] = useState<"idle" | "redirecting" | "waiting" | "error">("idle");

  useEffect(() => {
    // Check if we're returning from an OAuth redirect
    const params = new URLSearchParams(window.location.search);
    const code = params.get("code");
    const state = params.get("state");

    if (code && state) {
      setStatus("waiting");
      // Exchange the OAuth code for a public token via our API
      fetch("/v1/link/oauth/callback", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ code, state, link_token: linkToken }),
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
    // Get the OAuth URL from our API, then redirect
    fetch("/v1/link/oauth/initiate", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ institution_id: institution.id, link_token: linkToken }),
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

  if (status === "waiting") {
    return (
      <div>
        <div aria-live="polite">Completing connection...</div>
      </div>
    );
  }

  if (status === "error") {
    return (
      <div>
        <h2>Connection failed</h2>
        <p>Something went wrong. Please try again.</p>
        <button onClick={onBack}>Go back</button>
      </div>
    );
  }

  return (
    <div>
      <h2>Connect to {institution.name}</h2>
      <p>
        You'll be redirected to {institution.name} to securely authorize access.
        Hound never sees your login credentials.
      </p>
      <button onClick={handleConnect} disabled={status === "redirecting"}>
        {status === "redirecting" ? "Redirecting..." : `Continue to ${institution.name}`}
      </button>
      <button onClick={onBack}>Choose a different bank</button>
    </div>
  );
}
