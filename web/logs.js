(function () {
  const panel = document.getElementById("log-panel");
  const list = document.getElementById("log-list");
  const empty = document.getElementById("log-empty");
  const filterInput = document.getElementById("log-filter");
  const toggleButton = document.getElementById("log-toggle");
  const refreshButton = document.getElementById("log-refresh");

  if (!panel || !list) {
    return;
  }

  let logs = [];
  let collapsed = false;

  function escapeText(value) {
    return String(value || "");
  }

  function formatTime(value) {
    const date = value ? new Date(value) : new Date();
    if (Number.isNaN(date.getTime())) {
      return "";
    }
    return date.toLocaleString();
  }

  function matchesFilter(entry, filter) {
    if (!filter) {
      return true;
    }
    const needle = filter.toLowerCase();
    if (entry.message && entry.message.toLowerCase().includes(needle)) {
      return true;
    }
    if (entry.source && entry.source.toLowerCase().includes(needle)) {
      return true;
    }
    if (entry.meta) {
      return Object.entries(entry.meta).some(([key, value]) => {
        return String(key).toLowerCase().includes(needle) || String(value).toLowerCase().includes(needle);
      });
    }
    return false;
  }

  function render() {
    const filter = String(filterInput?.value || "").trim();
    const filtered = logs.filter((entry) => matchesFilter(entry, filter));

    list.innerHTML = "";

    if (!filtered.length) {
      if (empty) {
        empty.textContent = filter ? "No logs match the filter." : "No logs yet.";
        empty.classList.remove("hidden");
      }
      return;
    }

    if (empty) {
      empty.classList.add("hidden");
    }

    filtered.forEach((entry) => {
      const row = document.createElement("div");
      row.className = "log-entry";

      const header = document.createElement("div");
      header.className = "log-entry-header";
      header.textContent = `${formatTime(entry.time)} · ${escapeText(entry.source)} · ${escapeText(entry.level)}`;

      const message = document.createElement("div");
      message.className = "log-entry-message";
      message.textContent = escapeText(entry.message);

      row.appendChild(header);
      row.appendChild(message);

      if (entry.meta && Object.keys(entry.meta).length) {
        const meta = document.createElement("div");
        meta.className = "log-entry-meta";
        meta.textContent = Object.entries(entry.meta)
          .map(([key, value]) => `${key}: ${value}`)
          .join(" · ");
        row.appendChild(meta);
      }

      list.appendChild(row);
    });
  }

  async function loadLogs() {
    try {
      const response = await fetch("/api/logs?limit=200");
      if (!response.ok) {
        return;
      }
      const payload = await response.json().catch(() => ({}));
      logs = Array.isArray(payload.logs) ? payload.logs : [];
      render();
    } catch (error) {
      // Best-effort logging UI.
    }
  }

  function togglePanel() {
    collapsed = !collapsed;
    panel.classList.toggle("collapsed", collapsed);
    if (toggleButton) {
      toggleButton.textContent = collapsed ? "Show" : "Hide";
    }
  }

  if (filterInput) {
    filterInput.addEventListener("input", render);
  }

  if (toggleButton) {
    toggleButton.addEventListener("click", togglePanel);
  }

  if (refreshButton) {
    refreshButton.addEventListener("click", () => loadLogs());
  }

  window.appLog = async function appLog(action, detail, meta = {}) {
    try {
      const entry = {
        id: "local-" + Date.now(),
        time: new Date().toISOString(),
        level: "info",
        source: "client",
        message: String(action || "client.action"),
        meta: meta && typeof meta === "object" ? { ...meta } : {},
      };
      if (detail) {
        entry.meta.detail = String(detail);
      }
      logs = [...logs, entry];
      render();

      await fetch("/api/logs", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action, detail, meta: entry.meta }),
      });
      await loadLogs();
    } catch (error) {
      // No-op.
    }
  };

  loadLogs();
  setInterval(loadLogs, 3000);
})();
