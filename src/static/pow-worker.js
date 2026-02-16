(function() {
  function hasLeadingZeroBits(hashBytes, bits) {
    var fullBytes = Math.floor(bits / 8);
    for (var i = 0; i < fullBytes; i++) {
      if (hashBytes[i] !== 0) {
        return false;
      }
    }

    var remaining = bits % 8;
    if (remaining === 0) {
      return true;
    }

    var mask = (0xff << (8 - remaining)) & 0xff;
    return (hashBytes[fullBytes] & mask) === 0;
  }

  async function solve(message) {
    if (!(self.crypto && self.crypto.subtle && self.TextEncoder)) {
      self.postMessage({
        type: "error",
        error: "Web Crypto is not available in this browser.",
      });
      return;
    }

    var challenge = message.challenge || "";
    var difficulty = Number(message.difficulty) || 0;
    var expiresAt = Number(message.expiresAt) || 0;
    var nonce = Number(message.startNonce) || 0;
    var step = Number(message.step) || 1;
    var progressInterval = Number(message.progressInterval) || 2000;
    var encoder = new TextEncoder();
    var checkedSinceLastProgress = 0;

    if (!challenge || !(difficulty > 0) || !(step > 0)) {
      self.postMessage({
        type: "error",
        error: "Missing challenge parameters.",
      });
      return;
    }

    while (true) {
      if (expiresAt > 0 && Math.floor(Date.now() / 1000) >= expiresAt) {
        if (checkedSinceLastProgress > 0) {
          self.postMessage({
            type: "progress",
            checkedDelta: checkedSinceLastProgress,
          });
        }

        self.postMessage({ type: "expired" });
        return;
      }

      var payload = challenge + ":" + String(nonce);
      var digest = await self.crypto.subtle.digest("SHA-256", encoder.encode(payload));
      checkedSinceLastProgress += 1;

      if (hasLeadingZeroBits(new Uint8Array(digest), difficulty)) {
        self.postMessage({
          type: "found",
          nonce: nonce,
          checkedDelta: checkedSinceLastProgress,
        });
        return;
      }

      if (checkedSinceLastProgress >= progressInterval) {
        self.postMessage({
          type: "progress",
          checkedDelta: checkedSinceLastProgress,
        });
        checkedSinceLastProgress = 0;
      }

      nonce += step;
    }
  }

  self.onmessage = function(event) {
    var message = event.data || {};
    if (message.type !== "start") {
      return;
    }

    solve(message).catch(function(err) {
      self.postMessage({
        type: "error",
        error: err && err.message ? err.message : "Worker failed while solving challenge.",
      });
    });
  };
})();
