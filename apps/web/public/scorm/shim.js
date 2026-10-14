/*!
 * Vaultside SCORM API shim.
 *
 * This runs inside the SCORM content document. It exposes the standard
 * SCORM 1.2 (window.API) and SCORM 2004 (window.API_1484_11) objects, keeps
 * a local mirror of the CMI state so the synchronous Get/Set calls work, and
 * relays commits/finishes to the parent frame via postMessage. The parent
 * (host.js) owns the network call and the attempt token.
 *
 * Bootstrap: the parent sets iframe.name to a JSON string
 *   { version, cmi }
 * before assigning iframe.src. Cross-origin readable via window.name.
 */
(function () {
  'use strict';

  var boot = {};
  try { boot = JSON.parse(window.name || '{}') || {}; } catch (e) { /* ignore */ }
  var version = boot.version === '2004' ? '2004' : '1.2';
  var state = boot.cmi || {};
  var initialized = false;
  var finished = false;
  var lastError = '0';

  function post(op, payload) {
    try {
      window.parent.postMessage(
        Object.assign({ __vaultside: 'scorm', op: op }, payload || {}),
        '*'
      );
    } catch (e) { /* ignore */ }
  }

  window.addEventListener('message', function (e) {
    var d = e.data;
    if (!d || d.__vaultside !== 'scorm-host') return;
    if (d.op === 'reset-cmi' && d.cmi) {
      state = d.cmi;
    }
  });

  // Flush the local mirror to the parent. The parent does the fetch.
  function flush(op) {
    post(op, { cmi: state, version: version });
  }

  // ── SCORM 1.2 ─────────────────────────────────────────────────────────────
  var API = {
    LMSInitialize: function () { initialized = true; return 'true'; },
    LMSGetValue:   function (k) { return (state[k] === undefined ? '' : String(state[k])); },
    LMSSetValue:   function (k, v) { state[k] = String(v); return 'true'; },
    LMSCommit:     function () { flush('commit'); return 'true'; },
    LMSFinish:     function () { flush('finish'); finished = true; return 'true'; },
    LMSGetLastError: function () { return lastError; },
    LMSGetErrorString: function () { return lastError === '0' ? 'No error' : 'Error'; },
    LMSGetDiagnostic: function () { return ''; }
  };

  // ── SCORM 2004 ────────────────────────────────────────────────────────────
  var API_1484_11 = {
    Initialize:  function () { initialized = true; return 'true'; },
    GetValue:    function (k) { return (state[k] === undefined ? '' : String(state[k])); },
    SetValue:    function (k, v) { state[k] = String(v); return 'true'; },
    Commit:      function () { flush('commit'); return 'true'; },
    Terminate:   function () { flush('finish'); finished = true; return 'true'; },
    GetLastError: function () { return lastError; },
    GetErrorString: function () { return lastError === '0' ? 'No error' : 'Error'; },
    GetDiagnostic: function () { return ''; }
  };

  // If a same-origin parent already exposes an API, prefer it (direct sync).
  var w = window;
  try {
    while (w.parent && w.parent !== w) {
      w = w.parent;
      if (w.API || w.API_1484_11) {
        if (w.API)        { window.API = w.API; }
        if (w.API_1484_11) { window.API_1484_11 = w.API_1484_11; }
        return;
      }
    }
  } catch (e) { /* cross-origin: fall through to our own API */ }

  if (!window.API)          { window.API = API; }
  if (!window.API_1484_11)  { window.API_1484_11 = API_1484_11; }

  // Tell the parent we are ready; it can send us a refreshed CMI mirror.
  post('ready', { version: version });

  // Best-effort unload handler so an abandoned attempt is flagged.
  window.addEventListener('beforeunload', function () {
    if (initialized && !finished) {
      flush('abandon');
    }
  });
})();