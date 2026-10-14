/*!
 * Vaultside SCORM host.
 *
 * Runs on the course-player page (the app origin). Owns the launch response
 * and the short-lived attempt token, receives postMessage from the SCORM
 * content iframe (shim.js), and forwards commits/finishes to the API.
 *
 * Usage:
 *   var host = VaultsideScorm.attach(iframeEl, {
 *     endpoint: 'https://api.vaultside.example/v1/scorm/attempts/' + res.attempt_id,
 *     token:    res.session_token,
 *     version:  res.version,
 *     cmi:      res.cmi,
 *     contentUrl: '/scorm/content/' + res.package_id + '/' + res.entry_point
 *   });
 */
(function (global) {
  'use strict';

  function attach(iframe, opts) {
    var endpoint = opts.endpoint.replace(/\/$/, '');
    var token = opts.token;
    var version = opts.version === '2004' ? '2004' : '1.2';
    var cmi = opts.cmi || {};

    // Bootstrap via window.name (cross-origin readable).
    iframe.name = JSON.stringify({
      version: version,
      cmi: cmi
    });
    if (opts.contentUrl) {
      iframe.src = opts.contentUrl;
    }

    function post(payload) {
      try { iframe.contentWindow.postMessage(payload, '*'); } catch (e) {}
    }

    function fetchJSON(url, init) {
      return fetch(url, Object.assign({
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer ' + token
        },
        credentials: 'omit'
      }, init)).then(function (r) {
        if (!r.ok) { throw new Error('SCORM ' + url + ' -> ' + r.status); }
        return r.status === 204 ? null : r.json();
      });
    }

    function onMessage(e) {
      var d = e.data;
      if (!d || d.__vaultside !== 'scorm') { return; }
      var body = JSON.stringify({ cmi: d.cmi || {} });
      switch (d.op) {
        case 'ready':
          post({ __vaultside: 'scorm-host', op: 'reset-cmi', cmi: cmi });
          break;
        case 'commit':
          fetchJSON(endpoint + '/commit', { method: 'POST', body: body })
            .then(function (snap) { post({ __vaultside: 'scorm-host', op: 'commit-ok', snapshot: snap }); })
            .catch(function (err) { console.error('[scorm] commit failed', err); });
          break;
        case 'finish':
          fetchJSON(endpoint + '/finish', { method: 'POST', body: body })
            .then(function (snap) {
              post({ __vaultside: 'scorm-host', op: 'finish-ok', snapshot: snap });
              if (typeof opts.onFinish === 'function') { opts.onFinish(snap); }
            })
            .catch(function (err) { console.error('[scorm] finish failed', err); });
          break;
        case 'abandon':
          fetchJSON(endpoint + '/abandon', { method: 'POST' }).catch(function () {});
          break;
      }
    }

    global.addEventListener('message', onMessage);
    return {
      detach: function () { global.removeEventListener('message', onMessage); },
      cmi: function () { return cmi; }
    };
  }

  global.VaultsideScorm = { attach: attach };
})(window);