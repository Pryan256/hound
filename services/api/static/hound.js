/**
 * hound.js — Hound Link embed script
 *
 * Usage:
 *   <script src="https://api.hound.fi/static/hound.js"></script>
 *   <script>
 *     const handler = Hound.create({
 *       token: 'link-sandbox-...',
 *       onSuccess: function(publicToken, metadata) {
 *         // Exchange publicToken server-side for an access_token
 *       },
 *       onExit: function(err, metadata) { },
 *     });
 *     document.getElementById('connect-btn').onclick = function() { handler.open(); };
 *   </script>
 */
(function (w) {
  'use strict';

  function HoundLinkHandler(config) {
    var popup = null;
    var messageHandler = null;
    var pollTimer = null;

    function cleanup() {
      if (pollTimer) {
        clearInterval(pollTimer);
        pollTimer = null;
      }
      if (messageHandler) {
        w.removeEventListener('message', messageHandler);
        messageHandler = null;
      }
      if (popup && !popup.closed) {
        popup.close();
      }
      popup = null;
    }

    function open() {
      // Prevent double-open
      if (popup && !popup.closed) {
        popup.focus();
        return;
      }

      var base = config.apiHost || w.location.origin;
      var url = base + '/link/widget?link_token=' + encodeURIComponent(config.token);

      var width   = 480;
      var height  = 680;
      var left    = Math.round((w.screen.width  - width)  / 2);
      var top     = Math.round((w.screen.height - height) / 2);
      var features =
        'width='  + width  + ',' +
        'height=' + height + ',' +
        'left='   + left   + ',' +
        'top='    + top    + ',' +
        'resizable=yes,scrollbars=yes,status=no,toolbar=no,menubar=no,location=no';

      popup = w.open(url, 'hound-link', features);

      if (!popup) {
        // Popup blocked
        if (config.onExit) {
          config.onExit({ errorCode: 'POPUP_BLOCKED', errorMessage: 'Popup was blocked by the browser.', errorType: 'LINK_ERROR' }, {});
        }
        return;
      }

      messageHandler = function (event) {
        // Validate origin in production: event.origin should match your API host
        if (!event.data || typeof event.data.type !== 'string') return;

        switch (event.data.type) {
          case 'hound:success':
            if (config.onSuccess) {
              config.onSuccess(event.data.publicToken, event.data.metadata || {});
            }
            cleanup();
            break;

          case 'hound:exit':
            if (config.onExit) {
              config.onExit(null, event.data.metadata || {});
            }
            cleanup();
            break;

          case 'hound:error':
            if (config.onExit) {
              config.onExit(event.data.error || null, {});
            }
            cleanup();
            break;

          default:
            break;
        }
      };

      w.addEventListener('message', messageHandler);

      // Poll for popup closed by the user (clicking the native X)
      pollTimer = setInterval(function () {
        if (popup && popup.closed) {
          clearInterval(pollTimer);
          pollTimer = null;
          if (config.onExit) {
            config.onExit(null, { userClosed: true });
          }
          cleanup();
        }
      }, 500);
    }

    return {
      open: open,
      destroy: cleanup,
    };
  }

  w.Hound = {
    /**
     * Create a new Hound Link handler.
     *
     * @param {object} config
     * @param {string}   config.token      — link_token from POST /v1/link/token/create
     * @param {function} config.onSuccess  — called with (publicToken, metadata) on success
     * @param {function} [config.onExit]   — called with (error|null, metadata) on exit or error
     * @param {string}   [config.apiHost]  — override API host (defaults to current origin)
     * @returns {{ open: function, destroy: function }}
     */
    create: function (config) {
      if (!config || !config.token) {
        throw new Error('[Hound] config.token is required');
      }
      if (typeof config.onSuccess !== 'function') {
        throw new Error('[Hound] config.onSuccess must be a function');
      }
      return new HoundLinkHandler(config);
    },
  };
})(window);
