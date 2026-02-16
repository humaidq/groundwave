(function() {
  function getCSRFToken() {
    var meta = document.querySelector('meta[name="csrf-token"]');
    if (!meta) {
      return "";
    }
    return meta.getAttribute("content") || "";
  }

  function postJSON(url, body) {
    var headers = {
      "Content-Type": "application/json",
    };
    var csrfToken = getCSRFToken();
    if (csrfToken) {
      headers["X-CSRF-Token"] = csrfToken;
    }
    return fetch(url, {
      method: "POST",
      credentials: "same-origin",
      headers: headers,
      body: JSON.stringify(body || {}),
    }).then(function(response) {
      return response.json().then(function(data) {
        if (!response.ok) {
          var message = data && data.error ? data.error : "Request failed";
          throw new Error(message);
        }
        return data;
      }).catch(function(err) {
        if (response.ok) {
          return {};
        }
        throw err;
      });
    });
  }

  function showError(message) {
    var errorBox = document.querySelector("[data-pow-error]");
    var errorMessage = document.querySelector("[data-pow-error-message]");
    if (!errorBox || !errorMessage) {
      return;
    }

    errorMessage.textContent = message;
    errorBox.classList.remove("hidden");
  }

  function clearError() {
    var errorBox = document.querySelector("[data-pow-error]");
    if (!errorBox) {
      return;
    }

    errorBox.classList.add("hidden");
  }

  function showSuccess(message) {
    var successBox = document.querySelector("[data-pow-success]");
    var successMessage = document.querySelector("[data-pow-success-message]");
    if (!successBox || !successMessage) {
      return;
    }

    successMessage.textContent = message;
    successBox.classList.remove("hidden");
  }

  function clearSuccess() {
    var successBox = document.querySelector("[data-pow-success]");
    if (!successBox) {
      return;
    }

    successBox.classList.add("hidden");
  }

  function updateStatus(message) {
    var status = document.querySelector("[data-pow-status]");
    if (!status) {
      return;
    }

    status.textContent = message;
  }

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

  function yieldToBrowser() {
    return new Promise(function(resolve) {
      setTimeout(resolve, 0);
    });
  }

  async function solveChallengeSingleThread(challenge, difficulty, expiresAt, onProgress) {
    if (!(window.crypto && window.crypto.subtle && window.TextEncoder)) {
      throw new Error("Web Crypto is not available in this browser.");
    }

    var encoder = new TextEncoder();
    var nonce = 0;
    var batchSize = 1000;

    while (true) {
      if (expiresAt > 0 && Math.floor(Date.now() / 1000) >= expiresAt) {
        throw new Error("Challenge expired, please retry.");
      }

      var payload = challenge + ":" + String(nonce);
      var digest = await window.crypto.subtle.digest("SHA-256", encoder.encode(payload));
      if (hasLeadingZeroBits(new Uint8Array(digest), difficulty)) {
        onProgress(nonce + 1);
        return nonce;
      }

      nonce += 1;
      if (nonce % batchSize === 0) {
        onProgress(nonce);
        await yieldToBrowser();
      }
    }
  }

  function getSuggestedWorkerCount() {
    var cores = navigator.hardwareConcurrency || 1;
    if (!(cores > 0)) {
      return 1;
    }

    return Math.max(1, Math.min(cores, 8));
  }

  function createWorkerUnavailableError(message) {
    var err = new Error(message);
    err.powWorkerUnavailable = true;
    return err;
  }

  function createWorker(workerURL) {
    try {
      return new Worker(workerURL);
    } catch (_err) {
      return null;
    }
  }

  function solveChallengeWithWorkers(challenge, difficulty, expiresAt, workerURL, workerCount, onProgress) {
    return new Promise(function(resolve, reject) {
      var workers = [];
      var done = false;
      var totalChecked = 0;
      var progressInterval = 8000;

      function finish(err, nonce) {
        if (done) {
          return;
        }

        done = true;
        for (var i = 0; i < workers.length; i++) {
          workers[i].terminate();
        }

        if (err) {
          reject(err);
          return;
        }

        resolve(nonce);
      }

      for (var i = 0; i < workerCount; i++) {
        var worker = createWorker(workerURL);
        if (!worker) {
          finish(createWorkerUnavailableError("Web Worker unavailable"));
          return;
        }

        worker.onmessage = function(event) {
          if (done) {
            return;
          }

          var message = event.data || {};

          if (message.type === "progress") {
            totalChecked += message.checkedDelta || 0;
            onProgress(totalChecked);
            return;
          }

          if (message.type === "found") {
            totalChecked += message.checkedDelta || 0;
            onProgress(totalChecked);
            finish(null, message.nonce);
            return;
          }

          if (message.type === "expired") {
            finish(new Error("Challenge expired, please retry."));
            return;
          }

          if (message.type === "error") {
            finish(createWorkerUnavailableError(message.error || "Worker failed while solving challenge."));
          }
        };

        worker.onerror = function() {
          finish(createWorkerUnavailableError("Worker crashed while solving challenge."));
        };

        workers.push(worker);
      }

      for (var workerIndex = 0; workerIndex < workers.length; workerIndex++) {
        workers[workerIndex].postMessage({
          type: "start",
          challenge: challenge,
          difficulty: difficulty,
          expiresAt: expiresAt,
          startNonce: workerIndex,
          step: workerCount,
          progressInterval: progressInterval,
        });
      }
    });
  }

  async function solveChallenge(challenge, difficulty, expiresAt, onProgress) {
    if (!(window.crypto && window.crypto.subtle && window.TextEncoder)) {
      throw new Error("Web Crypto is not available in this browser.");
    }

    var workerURL = "/pow-worker.js";
    var workerCount = getSuggestedWorkerCount();

    if (window.Worker && workerCount > 1) {
      try {
        return await solveChallengeWithWorkers(challenge, difficulty, expiresAt, workerURL, workerCount, onProgress);
      } catch (err) {
        if (!(err && err.powWorkerUnavailable)) {
          throw err;
        }
      }
    }

    return solveChallengeSingleThread(challenge, difficulty, expiresAt, onProgress);
  }

  function parseIntData(node, key) {
    var value = node.getAttribute(key) || "";
    var parsed = parseInt(value, 10);
    if (isNaN(parsed)) {
      return 0;
    }

    return parsed;
  }

  function setProgress(percentage) {
    var bar = document.querySelector("[data-pow-progress-bar]");
    var text = document.querySelector("[data-pow-progress-text]");
    var clamped = Math.max(0, Math.min(100, Math.floor(percentage)));

    if (bar) {
      bar.style.width = String(clamped) + "%";
    }

    if (text) {
      text.textContent = String(clamped) + "%";
    }
  }

  function estimateProgress(difficulty, checkedNonces) {
    var expected = Math.pow(2, difficulty);
    if (!(expected > 0)) {
      return 0;
    }
    return Math.min(99, (checkedNonces / expected) * 100);
  }

  function formatKiloHashesPerSecond(hashesPerSecond) {
    if (!(hashesPerSecond > 0)) {
      return "0.00";
    }

    var kiloHashesPerSecond = hashesPerSecond / 1000;
    if (kiloHashesPerSecond >= 100) {
      return kiloHashesPerSecond.toFixed(0);
    }
    if (kiloHashesPerSecond >= 10) {
      return kiloHashesPerSecond.toFixed(1);
    }

    return kiloHashesPerSecond.toFixed(2);
  }

  function formatNonces(value) {
    if (!(value >= 0)) {
      return "0";
    }

    return value.toLocaleString();
  }

  function startProof(panel) {
    clearError();
    clearSuccess();

    var challenge = panel.getAttribute("data-challenge") || "";
    var difficulty = parseIntData(panel, "data-difficulty");
    var expiresAt = parseIntData(panel, "data-expires-at");
    var verifyURL = panel.getAttribute("data-verify-url") || "/pow/verify";

    if (!challenge || difficulty <= 0) {
      showError("Missing challenge data. Refresh and try again.");
      return;
    }

    setProgress(1);
    updateStatus("Solving challenge...");

    var startedAt = Date.now();
    var lastStatusUpdateAt = 0;
    var checkedNoncesTotal = 0;
    var solutionNonce = 0;
    var solveCompletedAt = startedAt;

    solveChallenge(challenge, difficulty, expiresAt, function(checkedNonces) {
      if (checkedNonces > checkedNoncesTotal) {
        checkedNoncesTotal = checkedNonces;
      }

      var now = Date.now();
      if (now - lastStatusUpdateAt < 120) {
        return;
      }

      lastStatusUpdateAt = now;
      var elapsedSeconds = (now - startedAt) / 1000;
      var hashesPerSecond = elapsedSeconds > 0 ? checkedNonces / elapsedSeconds : 0;
      setProgress(estimateProgress(difficulty, checkedNonces));
      updateStatus("Solving challenge... checked " + formatNonces(checkedNonces) + " nonces (" + formatKiloHashesPerSecond(hashesPerSecond) + " kH/s)");
    }).then(function(nonce) {
      solutionNonce = nonce;
      solveCompletedAt = Date.now();

      if (checkedNoncesTotal === 0) {
        checkedNoncesTotal = nonce + 1;
      }

      updateStatus("Verifying proof...");
      return postJSON(verifyURL, { nonce: nonce });
    }).then(function(result) {
      setProgress(100);

      var solveElapsedSeconds = (solveCompletedAt - startedAt) / 1000;
      var averageHashesPerSecond = solveElapsedSeconds > 0 ? checkedNoncesTotal / solveElapsedSeconds : 0;
      var confirmation = "Verified proof. Checked " + formatNonces(checkedNoncesTotal) + " nonces (solution nonce #" + formatNonces(solutionNonce) + ") at average " + formatKiloHashesPerSecond(averageHashesPerSecond) + " kH/s.";

      showSuccess(confirmation);

      var destination = result && result.redirect ? result.redirect : "/";
      updateStatus(confirmation + " Redirecting...");

      setTimeout(function() {
        window.location.assign(destination);
      }, 1000);
    }).catch(function(err) {
      showError(err.message || "Verification failed");
      updateStatus("Verification failed.");
    });
  }

  function init() {
    var panel = document.querySelector("[data-pow-panel]");
    if (!panel) {
      return;
    }

    startProof(panel);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
