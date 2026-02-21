(function() {
  function setOffline(offline) {
    var bar = document.getElementById("offline-bar");
    if (bar) {
      bar.classList.toggle("show", offline);
    }
  }

  function startRestrictedCountdown() {
    var bar = document.getElementById("restricted-bar");
    if (!bar) {
      return;
    }

    var expiresAt = parseInt(bar.getAttribute("data-expires-at"), 10);
    if (!expiresAt) {
      return;
    }

    var countdown = bar.querySelector(".restricted-bar-countdown");
    if (!countdown) {
      return;
    }

    function pad(value) {
      if (value < 10) {
        return "0" + value;
      }

      return String(value);
    }

    function formatRemaining(totalSeconds) {
      var hours = Math.floor(totalSeconds / 3600);
      var minutes = Math.floor((totalSeconds % 3600) / 60);
      var seconds = totalSeconds % 60;

      if (hours > 0) {
        return hours + ":" + pad(minutes) + ":" + pad(seconds);
      }

      return pad(minutes) + ":" + pad(seconds);
    }

    function updateCountdown() {
      var now = Math.floor(Date.now() / 1000);
      var remaining = expiresAt - now;
      if (remaining <= 0) {
        countdown.textContent = " - Lock expired";
        return;
      }

      countdown.textContent = " - Locks in " + formatRemaining(remaining);
    }

    updateCountdown();
    setInterval(updateCountdown, 1000);
  }

  function checkOnline() {
    fetch("/connectivity", { cache: "no-store" })
      .then(function(response) {
        setOffline(!response.ok);
      })
      .catch(function() {
        setOffline(true);
      });
  }

  document.addEventListener("DOMContentLoaded", function() {
    checkOnline();
    startRestrictedCountdown();
  });

  setInterval(checkOnline, 5000);
  window.addEventListener("online", checkOnline);
  window.addEventListener("offline", function() {
    setOffline(true);
  });
})();
