package handlers

import (
	"net/http"
)

// OAuthComplete serves the redirect landing page after Akoya OAuth.
// Akoya redirects here with ?code=...&state=...
// This page reads the link_token from sessionStorage, calls /link/oauth/callback,
// stores the public_token, and redirects back to /demo.
func (h *Handler) OAuthComplete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(oauthCompletePage))
}

// Demo serves the developer test harness for walking through the full Link flow.
func (h *Handler) Demo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(demoPage))
}

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
               border-top-color: #3b3000; border-radius: 50%; animation: spin 0.8s linear infinite;
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
  const params = new URLSearchParams(window.location.search);
  const code  = params.get('code');
  const state = params.get('state');
  const linkToken = sessionStorage.getItem('hound_link_token');

  // Show Akoya error if present
  const akoyaError = params.get('error');
  const akoyaDesc  = params.get('error_description');
  if (akoyaError) {
    document.getElementById('spinner').style.display = 'none';
    document.getElementById('title').textContent = 'Authorization failed';
    document.getElementById('msg').innerHTML = '<strong>' + akoyaError + '</strong>: ' + (akoyaDesc || '') +
      '<br><br><small>URL params: ' + window.location.search + '</small>';
    document.getElementById('msg').className = 'error';
    return;
  }

  if (!code || !state || !linkToken) {
    document.getElementById('spinner').style.display = 'none';
    document.getElementById('title').textContent = 'Something went wrong';
    document.getElementById('msg').innerHTML = 'Missing code, state, or link token.<br><small>URL: ' +
      window.location.search + '<br>linkToken in storage: ' + (linkToken ? 'yes' : 'NO') + '</small>';
    document.getElementById('msg').className = 'error';
    return;
  }

  try {
    const res = await fetch('/link/oauth/callback?link_token=' + encodeURIComponent(linkToken), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ code, state }),
    });

    const data = await res.json();

    if (!res.ok) {
      throw new Error(data.error_message || 'Callback failed');
    }

    sessionStorage.setItem('hound_public_token', data.public_token);
    window.location.href = '/demo?step=exchange';
  } catch (err) {
    document.getElementById('spinner').style.display = 'none';
    document.getElementById('title').textContent = 'Connection failed';
    document.getElementById('msg').textContent = err.message;
    document.getElementById('msg').className = 'error';
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
  </style>
</head>
<body>

<header>
  <svg width="20" height="20" viewBox="0 0 100 100" fill="none">
    <circle cx="50" cy="50" r="48" stroke="#3b3000" stroke-width="6"/>
    <circle cx="50" cy="42" r="16" fill="#3b3000"/>
    <path d="M20 80 Q50 55 80 80" stroke="#3b3000" stroke-width="6" fill="none"/>
  </svg>
  <h1>Hound.</h1>
  <span class="badge">Developer Demo</span>
</header>

<main>
  <div class="intro">
    <h2>Connect a bank account</h2>
    <p>Walk through the full Hound Link flow: create a link token, authenticate with your bank via OAuth, and fetch live account data.</p>
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

  <!-- Step 2: Select Institution -->
  <div class="step" id="step2">
    <div class="step-header">
      <div class="step-num">2</div>
      <div class="step-title">Select a bank</div>
    </div>
    <div class="step-body">
      <p class="note" style="margin-bottom:12px;">
        In the Akoya sandbox, use <strong>mikomo</strong> — it's Akoya's test institution.
        In production, your users will see all major US banks.
      </p>
      <div class="inst-grid" id="instGrid">
        <div class="inst-card selected" onclick="selectInst(this,'mikomo')" data-id="mikomo">
          <div class="inst-logo" style="background:#6366f1;display:flex;align-items:center;justify-content:center;color:white;font-size:10px;font-weight:700;">MK</div>
          <div><div class="inst-name">Mikomo Bank</div><div class="account-meta">Akoya Sandbox</div></div>
        </div>
        <div class="inst-card" onclick="selectInst(this,'chase')" data-id="chase">
          <img class="inst-logo" src="https://logo.clearbit.com/chase.com" onerror="this.style.display='none'"/>
          <div><div class="inst-name">Chase</div><div class="account-meta">Production only</div></div>
        </div>
        <div class="inst-card" onclick="selectInst(this,'bofa')" data-id="bofa">
          <img class="inst-logo" src="https://logo.clearbit.com/bankofamerica.com" onerror="this.style.display='none'"/>
          <div><div class="inst-name">Bank of America</div><div class="account-meta">Production only</div></div>
        </div>
        <div class="inst-card" onclick="selectInst(this,'capitalonebank')" data-id="capitalonebank">
          <img class="inst-logo" src="https://logo.clearbit.com/capitalone.com" onerror="this.style.display='none'"/>
          <div><div class="inst-name">Capital One</div><div class="account-meta">Production only</div></div>
        </div>
      </div>
      <button onclick="initiateOAuth()" id="connectBtn">Connect with Akoya →</button>
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

<script>
let linkToken = '';
let accessToken = '';
let selectedInst = 'mikomo';
const API_KEY_HEADER = () => document.getElementById('apiKey').value.trim();

function setStep(n) {
  for (let i = 1; i <= 4; i++) {
    const el = document.getElementById('step' + i);
    if (i < n)  { el.className = 'step done'; }
    else if (i === n) { el.className = 'step active'; }
    else        { el.className = 'step'; }
  }
}

function showResult(id, data, isError) {
  const el = document.getElementById(id);
  el.innerHTML = '<div class="result"><div class="result-label">' +
    (isError ? '⚠ Error' : '✓ Response') + '</div><pre>' +
    JSON.stringify(data, null, 2) + '</pre></div>';
}

function selectInst(card, id) {
  document.querySelectorAll('.inst-card').forEach(c => c.classList.remove('selected'));
  card.classList.add('selected');
  selectedInst = id;
}

async function createLinkToken() {
  const key = API_KEY_HEADER();
  if (!key) { alert('Enter an API key'); return; }
  try {
    const res = await fetch('/v1/link/token/create', {
      method: 'POST',
      headers: { 'Authorization': 'Bearer ' + key, 'Content-Type': 'application/json' },
      body: JSON.stringify({ user_id: 'demo-user-001', products: ['transactions','identity'], country_codes: ['US'] }),
    });
    const data = await res.json();
    if (!res.ok) { showResult('result1', data, true); return; }
    linkToken = data.link_token;
    sessionStorage.setItem('hound_link_token', linkToken);
    sessionStorage.setItem('hound_api_key', key);
    showResult('result1', data, false);
    setStep(2);
  } catch(e) { showResult('result1', { error: e.message }, true); }
}

async function initiateOAuth() {
  if (!linkToken) { alert('Create a link token first'); return; }
  try {
    const res = await fetch('/link/oauth/initiate?link_token=' + encodeURIComponent(linkToken), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ institution_id: selectedInst }),
    });
    const data = await res.json();
    if (!res.ok) { showResult('result2', data, true); return; }
    showResult('result2', { oauth_url: data.oauth_url }, false);
    // Full-page redirect — Akoya will redirect back to /link/oauth/complete
    setTimeout(() => { window.location.href = data.oauth_url; }, 800);
  } catch(e) { showResult('result2', { error: e.message }, true); }
}

async function exchangeToken() {
  const publicToken = document.getElementById('publicTokenInput').value.trim();
  const key = sessionStorage.getItem('hound_api_key') || API_KEY_HEADER();
  if (!publicToken) { alert('No public token'); return; }
  try {
    const res = await fetch('/v1/item/public_token/exchange', {
      method: 'POST',
      headers: { 'Authorization': 'Bearer ' + key, 'Content-Type': 'application/json' },
      body: JSON.stringify({ public_token: publicToken }),
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
    const res = await fetch('/v1/transactions?start_date=2026-01-01&end_date=2026-03-17', {
      headers: { 'Authorization': 'Bearer ' + key, 'Hound-Access-Token': accessToken },
    });
    const data = await res.json();
    showResult('result5', data, !res.ok);
  } catch(e) { showResult('result5', { error: e.message }, true); }
}

// On page load: check if returning from OAuth
window.addEventListener('DOMContentLoaded', () => {
  const params = new URLSearchParams(window.location.search);
  if (params.get('step') === 'exchange') {
    const publicToken = sessionStorage.getItem('hound_public_token');
    const apiKey = sessionStorage.getItem('hound_api_key');
    linkToken = sessionStorage.getItem('hound_link_token') || '';
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
