package handlers

import (
	"net/http"
	"os"
	"path/filepath"
)

// LinkWidget serves the hosted Hound Link React app (popup window).
// Route: GET /link/widget?link_token=...
// The React app reads link_token from the URL and handles the full flow.
func (h *Handler) LinkWidget(w http.ResponseWriter, r *http.Request) {
	// Try to serve the built React app first; fall back to inline HTML for dev.
	indexPath := filepath.Join("static", "link", "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		http.ServeFile(w, r, indexPath)
		return
	}

	// Dev fallback: inline page that loads the Vite dev server script
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(linkWidgetDevPage))
}

// OAuthComplete serves the redirect landing page after Akoya OAuth.
// Akoya redirects here with ?code=...&state=...
//
// In popup mode (window.opener exists) the page postMessages the result to
// its opener (the widget popup) then closes itself.  In legacy redirect mode
// it still works as before.
func (h *Handler) OAuthComplete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(oauthCompletePage))
}

// Demo serves the developer test harness for walking through the full Link flow.
func (h *Handler) Demo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(demoPage))
}

// ── linkWidgetDevPage ─────────────────────────────────────────────────────────
// Shown when the built React assets haven't been compiled yet (local dev).
const linkWidgetDevPage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Connect your bank — Hound</title>
  <meta name="robots" content="noindex">
  <style>
    body { font-family: -apple-system, sans-serif; display: flex; align-items: center;
           justify-content: center; min-height: 100vh; margin: 0; background: #f8f7f4; }
    .card { background: white; border-radius: 16px; padding: 48px 40px; max-width: 420px;
            width: 90%; text-align: center; box-shadow: 0 4px 40px rgba(0,0,0,0.12); }
    .icon { font-size: 40px; margin-bottom: 16px; }
    h2 { color: #2D2800; margin: 0 0 10px; font-size: 20px; font-weight: 700; }
    p  { color: #6b7280; margin: 0; font-size: 14px; line-height: 1.6; }
    code { background: #f3f4f6; border-radius: 4px; padding: 2px 6px;
           font-size: 12px; font-family: monospace; color: #374151; }
  </style>
</head>
<body>
<div class="card">
  <div class="icon">🔗</div>
  <h2>Link widget not built yet</h2>
  <p>Run <code>npm run build:hosted</code> inside <code>packages/link</code> to compile the React app,
     or start the Vite dev server and proxy requests through it.</p>
</div>
</body>
</html>`

// ── oauthCompletePage ─────────────────────────────────────────────────────────
// Handles both popup mode (postMessage → close) and legacy redirect mode.
const oauthCompletePage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Connecting your account...</title>
  <style>
    body { font-family: -apple-system, sans-serif; display: flex; align-items: center;
           justify-content: center; min-height: 100vh; margin: 0; background: #f8f7f4; }
    .card { background: white; border-radius: 12px; padding: 40px; max-width: 420px;
            width: 100%; text-align: center; box-shadow: 0 2px 20px rgba(0,0,0,0.08); }
    .spinner { width: 40px; height: 40px; border: 3px solid #e5e7eb;
               border-top-color: #2D2800; border-radius: 50%; animation: spin 0.8s linear infinite;
               margin: 0 auto 20px; }
    @keyframes spin { to { transform: rotate(360deg); } }
    h2 { color: #1a1a1a; margin: 0 0 8px; font-size: 20px; }
    p  { color: #6b7280; margin: 0; font-size: 14px; }
    .error { color: #dc2626; }
  </style>
</head>
<body>
<div class="card">
  <div class="spinner" id="spinner"></div>
  <h2 id="title">Completing connection...</h2>
  <p id="msg">Please wait while we finish linking your account.</p>
</div>
<script>
(async () => {
  const params    = new URLSearchParams(window.location.search);
  const code      = params.get('code');
  const state     = params.get('state');
  const akoyaError = params.get('error');
  const akoyaDesc  = params.get('error_description');

  // ── Helper: postMessage to widget popup (our opener) then close ──────────
  // The widget popup is window.opener when Akoya redirects back here directly.
  // The developer's page is window.opener.opener.
  function notifyOpenerSuccess(publicToken) {
    if (window.opener && !window.opener.closed) {
      window.opener.postMessage({ type: 'hound:success', publicToken: publicToken, metadata: {} }, '*');
    }
    setTimeout(function() { window.close(); }, 150);
  }

  function notifyOpenerError(errCode, errMsg) {
    if (window.opener && !window.opener.closed) {
      window.opener.postMessage({
        type: 'hound:error',
        error: { errorCode: errCode, errorMessage: errMsg, errorType: 'OAUTH_ERROR' }
      }, '*');
    }
    setTimeout(function() { window.close(); }, 150);
  }

  // ── Show Akoya-level error ───────────────────────────────────────────────
  if (akoyaError) {
    document.getElementById('spinner').style.display = 'none';
    document.getElementById('title').textContent = 'Authorization failed';
    document.getElementById('msg').textContent = akoyaDesc || akoyaError;
    document.getElementById('msg').className = 'error';
    notifyOpenerError(akoyaError, akoyaDesc || 'Authorization was declined.');
    return;
  }

  if (!code || !state) {
    document.getElementById('spinner').style.display = 'none';
    document.getElementById('title').textContent = 'Something went wrong';
    document.getElementById('msg').textContent = 'Missing authorization code or state parameter.';
    document.getElementById('msg').className = 'error';
    notifyOpenerError('MISSING_PARAMS', 'Missing code or state.');
    return;
  }

  // Retrieve the link token — prefer sessionStorage (set by the widget before
  // initiating OAuth), then fall back to URL param for legacy flows.
  const linkToken = sessionStorage.getItem('hound_link_token') || params.get('link_token') || '';

  if (!linkToken) {
    document.getElementById('spinner').style.display = 'none';
    document.getElementById('title').textContent = 'Session expired';
    document.getElementById('msg').textContent = 'Could not find the link session. Please start over.';
    document.getElementById('msg').className = 'error';
    notifyOpenerError('SESSION_EXPIRED', 'Link token not found in session storage.');
    return;
  }

  try {
    const res  = await fetch('/link/oauth/callback?link_token=' + encodeURIComponent(linkToken), {
      method:  'POST',
      headers: { 'Content-Type': 'application/json' },
      body:    JSON.stringify({ code, state }),
    });
    const data = await res.json();

    if (!res.ok) {
      throw new Error(data.error_message || 'Callback failed');
    }

    if (window.opener && !window.opener.closed) {
      // Popup mode: postMessage success to widget opener, then close
      notifyOpenerSuccess(data.public_token);
    } else {
      // Legacy redirect mode: store in sessionStorage and navigate back to demo
      sessionStorage.setItem('hound_public_token', data.public_token);
      window.location.href = '/demo?step=exchange';
    }
  } catch (err) {
    document.getElementById('spinner').style.display = 'none';
    document.getElementById('title').textContent = 'Connection failed';
    document.getElementById('msg').textContent = err.message;
    document.getElementById('msg').className = 'error';
    notifyOpenerError('CALLBACK_ERROR', err.message);
  }
})();
</script>
</body>
</html>`

const demoPage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Hound — Developer Demo</title>
  <style>
    *, *::before, *::after { box-sizing: border-box; }
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
           background: #f8f7f4; color: #1a1a1a; margin: 0; padding: 0; }

    header { background: white; border-bottom: 1px solid #e5e7eb;
             padding: 16px 32px; display: flex; align-items: center; gap: 12px; }
    header h1 { font-size: 20px; font-weight: 700; margin: 0; letter-spacing: -0.3px; }
    .badge { background: #f3f4f6; color: #6b7280; font-size: 11px; font-weight: 600;
             padding: 2px 8px; border-radius: 20px; text-transform: uppercase; letter-spacing: 0.5px; }

    main { max-width: 680px; margin: 48px auto; padding: 0 24px 80px; }
    .intro { margin-bottom: 32px; }
    .intro h2 { font-size: 24px; font-weight: 700; margin: 0 0 8px; }
    .intro p  { color: #6b7280; margin: 0; font-size: 15px; line-height: 1.6; }

    .step { background: white; border: 1px solid #e5e7eb; border-radius: 12px;
            margin-bottom: 16px; overflow: hidden; transition: border-color 0.15s; }
    .step.active { border-color: #3b3000; }
    .step.done   { border-color: #10b981; }
    .step-header { padding: 20px 24px; display: flex; align-items: center; gap: 16px; }
    .step-num  { width: 32px; height: 32px; border-radius: 50%; display: flex; align-items: center;
                 justify-content: center; font-size: 13px; font-weight: 700; flex-shrink: 0;
                 background: #f3f4f6; color: #9ca3af; }
    .step.active .step-num { background: #3b3000; color: white; }
    .step.done   .step-num { background: #10b981; color: white; }
    .step-title { font-size: 15px; font-weight: 600; flex: 1; }
    .step-body  { padding: 0 24px 24px; padding-left: 72px; }

    label { display: block; font-size: 13px; font-weight: 500; color: #374151; margin-bottom: 6px; }
    input, select { width: 100%; padding: 10px 12px; border: 1px solid #d1d5db;
                    border-radius: 8px; font-size: 14px; font-family: 'SF Mono', monospace;
                    outline: none; transition: border-color 0.15s; }
    input:focus, select:focus { border-color: #3b3000; }
    input[readonly] { background: #f9fafb; color: #6b7280; }

    button { background: #3b3000; color: white; border: none; border-radius: 8px;
             padding: 10px 20px; font-size: 14px; font-weight: 600; cursor: pointer;
             transition: opacity 0.15s; margin-top: 12px; }
    button:hover   { opacity: 0.85; }
    button:disabled { opacity: 0.4; cursor: not-allowed; }

    .result { margin-top: 16px; background: #f8f7f4; border-radius: 8px; overflow: hidden; }
    .result-label { padding: 8px 12px; font-size: 11px; font-weight: 600; color: #6b7280;
                    text-transform: uppercase; letter-spacing: 0.5px; border-bottom: 1px solid #e5e7eb; }
    .result pre { margin: 0; padding: 12px; font-size: 12px; font-family: 'SF Mono', monospace;
                  color: #1a1a1a; overflow-x: auto; white-space: pre-wrap; max-height: 240px; overflow-y: auto; }

    .inst-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-top: 4px; }
    .inst-card { border: 1px solid #e5e7eb; border-radius: 8px; padding: 12px;
                 display: flex; align-items: center; gap: 10px; cursor: pointer;
                 transition: border-color 0.15s; }
    .inst-card:hover  { border-color: #9ca3af; }
    .inst-card.selected { border-color: #3b3000; background: #fafaf8; }
    .inst-logo { width: 32px; height: 32px; border-radius: 6px; object-fit: contain;
                 background: #f3f4f6; padding: 2px; }
    .inst-name  { font-size: 13px; font-weight: 500; }

    .note { font-size: 12px; color: #6b7280; margin-top: 8px; line-height: 1.5; }
    .note a { color: #3b3000; }

    .accounts-list { display: flex; flex-direction: column; gap: 8px; margin-top: 4px; }
    .account-row { background: #f9fafb; border-radius: 8px; padding: 12px 16px;
                   display: flex; justify-content: space-between; align-items: center; }
    .account-name { font-size: 14px; font-weight: 500; }
    .account-meta { font-size: 12px; color: #6b7280; margin-top: 2px; }
    .account-bal  { font-size: 15px; font-weight: 600; color: #10b981; }

    .txn-list { display: flex; flex-direction: column; gap: 6px; margin-top: 4px; }
    .txn-row  { background: #f9fafb; border-radius: 8px; padding: 10px 16px;
                display: flex; justify-content: space-between; align-items: center; }
    .txn-name { font-size: 13px; font-weight: 500; }
    .txn-date { font-size: 11px; color: #9ca3af; }
    .txn-amt  { font-size: 13px; font-weight: 600; }
    .txn-amt.debit  { color: #dc2626; }
    .txn-amt.credit { color: #10b981; }

    .success-icon { color: #10b981; font-size: 16px; margin-left: 8px; }

    /* Hound Link button */
    .hound-connect-btn {
      background: #2D2800; color: white; border: none; border-radius: 10px;
      padding: 12px 24px; font-size: 15px; font-weight: 600; cursor: pointer;
      transition: opacity 0.15s; margin-top: 8px; display: inline-flex;
      align-items: center; gap: 8px;
    }
    .hound-connect-btn:hover { opacity: 0.87; }
  </style>
</head>
<body>

<header>
  <svg width="20" height="20" viewBox="0 0 100 100" fill="none">
    <circle cx="50" cy="50" r="46" fill="#2D2800"/>
    <circle cx="50" cy="40" r="14" fill="white"/>
    <path d="M22 76 Q50 52 78 76" stroke="white" stroke-width="6" fill="none" stroke-linecap="round"/>
  </svg>
  <h1>Hound.</h1>
  <span class="badge">Developer Demo</span>
</header>

<main>
  <div class="intro">
    <h2>Connect a bank account</h2>
    <p>Walk through the full Hound Link flow: create a link token, open the Link widget, and fetch live account data.</p>
  </div>

  <!-- Step 1: API Key -->
  <div class="step active" id="step1">
    <div class="step-header">
      <div class="step-num">1</div>
      <div class="step-title">Your API key</div>
    </div>
    <div class="step-body">
      <label for="apiKey">Test API Key</label>
      <input type="text" id="apiKey" value="hound_test_localdev_key_00000000" placeholder="hound_test_..." />
      <p class="note">This key was seeded automatically for local development. Never use test keys in production.</p>
      <button onclick="createLinkToken()">Create Link Token →</button>
      <div id="result1"></div>
    </div>
  </div>

  <!-- Step 2: Open Link Widget -->
  <div class="step" id="step2">
    <div class="step-header">
      <div class="step-num">2</div>
      <div class="step-title">Open Hound Link</div>
    </div>
    <div class="step-body">
      <p class="note" style="margin-bottom:12px;">
        Click the button below to open the Hound Link widget in a popup.
        In the sandbox, use <strong>Mikomo Bank</strong> — it's Akoya's test institution.
      </p>
      <button class="hound-connect-btn" id="openLinkBtn" onclick="openLink()" disabled>
        <svg width="16" height="16" viewBox="0 0 100 100" fill="none">
          <circle cx="50" cy="50" r="46" fill="white"/>
          <circle cx="50" cy="40" r="14" fill="#2D2800"/>
          <path d="M22 76 Q50 52 78 76" stroke="#2D2800" stroke-width="8" fill="none" stroke-linecap="round"/>
        </svg>
        Connect your bank
      </button>
      <div id="result2"></div>
    </div>
  </div>

  <!-- Step 3: Exchange Token -->
  <div class="step" id="step3">
    <div class="step-header">
      <div class="step-num">3</div>
      <div class="step-title">Exchange public token</div>
    </div>
    <div class="step-body">
      <label>Public Token (received after OAuth)</label>
      <input type="text" id="publicTokenInput" readonly placeholder="hound_public_..."/>
      <button onclick="exchangeToken()" id="exchangeBtn">Exchange for Access Token →</button>
      <div id="result3"></div>
    </div>
  </div>

  <!-- Step 4: Fetch Data -->
  <div class="step" id="step4">
    <div class="step-header">
      <div class="step-num">4</div>
      <div class="step-title">Fetch financial data</div>
    </div>
    <div class="step-body">
      <button onclick="fetchAccounts()">Fetch Accounts →</button>
      <div id="result4"></div>
      <div id="txnSection" style="display:none; margin-top:20px;">
        <button onclick="fetchTransactions()">Fetch Transactions →</button>
        <div id="result5"></div>
      </div>
    </div>
  </div>

</main>

<script src="/static/hound.js"></script>
<script>
let linkToken   = '';
let accessToken = '';
let houndHandler = null;

const API_KEY_HEADER = () => document.getElementById('apiKey').value.trim();

function setStep(n) {
  for (let i = 1; i <= 4; i++) {
    const el = document.getElementById('step' + i);
    if      (i < n)  el.className = 'step done';
    else if (i === n) el.className = 'step active';
    else             el.className = 'step';
  }
}

function showResult(id, data, isError) {
  const el = document.getElementById(id);
  el.innerHTML = '<div class="result"><div class="result-label">' +
    (isError ? '&#9888; Error' : '&#10003; Response') + '</div><pre>' +
    JSON.stringify(data, null, 2) + '</pre></div>';
}

async function createLinkToken() {
  const key = API_KEY_HEADER();
  if (!key) { alert('Enter an API key'); return; }
  try {
    const res = await fetch('/v1/link/token/create', {
      method:  'POST',
      headers: { 'Authorization': 'Bearer ' + key, 'Content-Type': 'application/json' },
      body:    JSON.stringify({ user_id: 'demo-user-001', products: ['transactions','identity'], country_codes: ['US'] }),
    });
    const data = await res.json();
    if (!res.ok) { showResult('result1', data, true); return; }

    linkToken = data.link_token;
    sessionStorage.setItem('hound_link_token', linkToken);
    sessionStorage.setItem('hound_api_key', key);
    showResult('result1', data, false);
    setStep(2);

    // Enable the Link button
    document.getElementById('openLinkBtn').disabled = false;

    // Create the Hound Link handler
    houndHandler = Hound.create({
      token: linkToken,
      onSuccess: function(publicToken, metadata) {
        document.getElementById('publicTokenInput').value = publicToken;
        sessionStorage.setItem('hound_public_token', publicToken);
        showResult('result2', { status: 'success', public_token: publicToken, metadata: metadata }, false);
        setStep(3);
      },
      onExit: function(err, metadata) {
        if (err) {
          showResult('result2', { error: err, metadata: metadata }, true);
        } else {
          showResult('result2', { status: 'exited', metadata: metadata }, false);
        }
      },
    });
  } catch(e) { showResult('result1', { error: e.message }, true); }
}

function openLink() {
  if (!houndHandler) { alert('Create a link token first'); return; }
  houndHandler.open();
}

async function exchangeToken() {
  const publicToken = document.getElementById('publicTokenInput').value.trim();
  const key = sessionStorage.getItem('hound_api_key') || API_KEY_HEADER();
  if (!publicToken) { alert('No public token'); return; }
  try {
    const res = await fetch('/v1/item/public_token/exchange', {
      method:  'POST',
      headers: { 'Authorization': 'Bearer ' + key, 'Content-Type': 'application/json' },
      body:    JSON.stringify({ public_token: publicToken }),
    });
    const data = await res.json();
    if (!res.ok) { showResult('result3', data, true); return; }
    accessToken = data.access_token;
    showResult('result3', data, false);
    setStep(4);
  } catch(e) { showResult('result3', { error: e.message }, true); }
}

async function fetchAccounts() {
  const key = sessionStorage.getItem('hound_api_key') || API_KEY_HEADER();
  try {
    const res = await fetch('/v1/accounts', {
      headers: { 'Authorization': 'Bearer ' + key, 'Hound-Access-Token': accessToken },
    });
    const data = await res.json();
    if (!res.ok) { showResult('result4', data, true); return; }
    showResult('result4', data, false);
    document.getElementById('txnSection').style.display = 'block';
  } catch(e) { showResult('result4', { error: e.message }, true); }
}

async function fetchTransactions() {
  const key = sessionStorage.getItem('hound_api_key') || API_KEY_HEADER();
  try {
    const res = await fetch('/v1/transactions?start_date=2019-01-01&end_date=2021-12-31', {
      headers: { 'Authorization': 'Bearer ' + key, 'Hound-Access-Token': accessToken },
    });
    const data = await res.json();
    showResult('result5', data, !res.ok);
  } catch(e) { showResult('result5', { error: e.message }, true); }
}

// On page load: check if returning from legacy OAuth redirect (non-popup mode)
window.addEventListener('DOMContentLoaded', () => {
  const params = new URLSearchParams(window.location.search);
  if (params.get('step') === 'exchange') {
    const publicToken = sessionStorage.getItem('hound_public_token');
    const apiKey      = sessionStorage.getItem('hound_api_key');
    linkToken         = sessionStorage.getItem('hound_link_token') || '';
    if (publicToken) {
      if (apiKey) document.getElementById('apiKey').value = apiKey;
      document.getElementById('publicTokenInput').value = publicToken;
      setStep(3);
      document.getElementById('step3').scrollIntoView({ behavior: 'smooth' });
    }
  }
});
</script>
</body>
</html>`
