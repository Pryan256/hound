/**
 * hosted.tsx — Entry point for the Hound Link hosted page.
 *
 * This is the page served at /link/widget?link_token=... inside a popup window.
 * It renders the HoundLink modal and postMessages results back to window.opener.
 *
 * Message flow:
 *   /link/oauth/complete  → (postMessage) → /link/widget (this page) → (postMessage) → developer's page
 */
import { createRoot } from "react-dom/client";
import { HoundLink } from "./components/Link";
import styles from "./components/Link.module.css";

// ── Completing OAuth ─────────────────────────────────────────────────────────
// If URL has ?code=...&state=..., we're the OAuth callback target inside the popup.
// Exchange the code for a public_token, then postMessage success to our opener
// (the developer's page), and close.

function OAuthCompletingScreen() {
  return (
    <div className={styles.overlay}>
      <div className={styles.modal}>
        <div className={styles.header}>
          <div className={styles.logoWordmark}>
            <svg width="26" height="26" viewBox="0 0 100 100" fill="none" aria-hidden="true">
              <circle cx="50" cy="50" r="46" fill="#2D2800" />
              <circle cx="50" cy="40" r="14" fill="white" />
              <path d="M22 76 Q50 52 78 76" stroke="white" strokeWidth="6" fill="none" strokeLinecap="round" />
            </svg>
            <span>Hound</span>
          </div>
        </div>
        <div className={styles.body}>
          <div className={styles.connectingWrapper}>
            <div className={styles.spinner} aria-hidden="true" />
            <p className={styles.connectingTitle}>Completing connection...</p>
            <p className={styles.connectingSubtitle}>Please wait while we finish linking your account.</p>
          </div>
        </div>
        <div className={styles.footer}>
          <svg className={styles.footerLock} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
            <rect x="3" y="7" width="10" height="8" rx="2" />
            <path d="M5 7V5a3 3 0 0 1 6 0v2" strokeLinecap="round" />
          </svg>
          <span>Secured by Hound</span>
        </div>
      </div>
    </div>
  );
}

function postSuccess(publicToken: string, metadata: object) {
  window.opener?.postMessage(
    { type: "hound:success", publicToken, metadata },
    "*"
  );
  setTimeout(() => window.close(), 150);
}

function postExit() {
  window.opener?.postMessage({ type: "hound:exit" }, "*");
  setTimeout(() => window.close(), 150);
}

function postError(error: { errorCode: string; errorMessage: string; errorType: string }) {
  window.opener?.postMessage({ type: "hound:error", error }, "*");
  setTimeout(() => window.close(), 150);
}

// ── Bootstrap ────────────────────────────────────────────────────────────────
const params = new URLSearchParams(window.location.search);
const oauthCode  = params.get("code");
const oauthState = params.get("state");
const oauthError = params.get("error");
const linkToken  = params.get("link_token") ?? "";

const root = createRoot(document.getElementById("root")!);

// Case 1: OAuth error callback (e.g. user cancelled at bank)
if (oauthError) {
  postError({
    errorCode:    oauthError,
    errorMessage: params.get("error_description") ?? "Authorization was cancelled.",
    errorType:    "OAUTH_ERROR",
  });
  root.render(<OAuthCompletingScreen />);
}
// Case 2: OAuth success callback — exchange code for public_token
else if (oauthCode && oauthState) {
  root.render(<OAuthCompletingScreen />);

  // The link_token for the callback lives in sessionStorage (stored by the widget
  // before it redirected to the bank), OR it may be passed as a URL param by
  // some OAuth flows.  We try both.
  const storedLinkToken = sessionStorage.getItem("hound_link_token") ?? linkToken;

  fetch("/link/oauth/callback?link_token=" + encodeURIComponent(storedLinkToken), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code: oauthCode, state: oauthState }),
  })
    .then((r) => r.json())
    .then((data) => {
      if (data.public_token) {
        postSuccess(data.public_token, {});
      } else {
        postError({
          errorCode:    data.error_code    ?? "CALLBACK_ERROR",
          errorMessage: data.error_message ?? "OAuth callback failed.",
          errorType:    "OAUTH_ERROR",
        });
      }
    })
    .catch((err) => {
      postError({
        errorCode:    "NETWORK_ERROR",
        errorMessage: String(err),
        errorType:    "OAUTH_ERROR",
      });
    });
}
// Case 3: Normal widget open — render the full HoundLink modal
else if (linkToken) {
  // Store for use during OAuth return
  sessionStorage.setItem("hound_link_token", linkToken);

  root.render(
    <HoundLink
      config={{
        token: linkToken,
        onSuccess: (publicToken, metadata) => {
          postSuccess(publicToken, metadata);
        },
        onExit: (_error, _metadata) => {
          postExit();
        },
      }}
      onClose={() => {
        postExit();
      }}
    />
  );
} else {
  // No link_token — render an error state
  root.render(
    <div className={styles.overlay}>
      <div className={styles.modal}>
        <div className={styles.body}>
          <div className={styles.errorWrapper}>
            <div className={styles.errorIcon}>⚠️</div>
            <p className={styles.errorTitle}>Invalid session</p>
            <p className={styles.errorSubtitle}>
              No link token provided. Please close this window and try again.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
