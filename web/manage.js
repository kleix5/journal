const groupForm = document.getElementById("group-form");
const groupList = document.getElementById("group-list");
const statusNode = document.getElementById("status");
const groupCount = document.getElementById("group-count");

function setStatus(message) {
  statusNode.textContent = message;
}

async function request(url, options = {}) {
  const response = await fetch(url, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });

  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(payload.error || "Request failed");
  }
  return payload;
}

function studentLines(value) {
  return value
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function renderGroups(groups) {
  groupCount.textContent = `${groups.length} group(s)`;
  groupList.innerHTML = "";

  if (!groups.length) {
    groupList.innerHTML = `<div class="empty">No groups yet.</div>`;
    return;
  }

  groups.forEach((group) => {
    const node = document.createElement("article");
    node.className = "group-card";
    const studentNames = group.students.map((student) => escapeHTML(student.name)).join(", ");
    node.innerHTML = `
      <strong>${escapeHTML(group.name)}</strong>
      <div class="muted">Subject: ${escapeHTML(group.subject)}</div>
      <div class="muted">Students: ${group.students.length}</div>
      <div>${studentNames}</div>
      <div class="actions">
        <a class="button-link" href="/lesson.html?group=${group.id}">Open session</a>
        <a class="button-link" href="/journal.html?group=${group.id}">Open journal</a>
        <button type="button" class="delete-group-button" data-group-id="${group.id}" data-group-name="${escapeHTML(group.name)}">Remove group</button>
      </div>
    `;
    groupList.appendChild(node);
  });
}

async function loadGroups() {
  const data = await request("/api/groups");
  renderGroups(data.groups || []);
}

groupForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const formData = new FormData(groupForm);
  const name = String(formData.get("name") || "").trim();
  const subject = String(formData.get("subject") || "").trim();
  const students = studentLines(String(formData.get("students") || ""));

  try {
    const group = await request("/api/groups", {
      method: "POST",
      body: JSON.stringify({ name, subject, students }),
    });
    localStorage.setItem("selectedGroupId", group.id);
    groupForm.reset();
    await loadGroups();
    setStatus("Group saved.");
  } catch (error) {
    setStatus(error.message);
  }
});

groupList.addEventListener("click", async (event) => {
  const button = event.target.closest(".delete-group-button");
  if (!button) {
    return;
  }

  const groupID = String(button.dataset.groupId || "");
  const groupName = String(button.dataset.groupName || "").trim();
  if (!groupID) {
    return;
  }

  const confirmed = window.confirm(`Remove group "${groupName}"? This will delete its lessons and scores too.`);
  if (!confirmed) {
    return;
  }

  button.disabled = true;

  try {
    await request(`/api/groups/${groupID}`, { method: "DELETE" });
    if (localStorage.getItem("selectedGroupId") === groupID) {
      localStorage.removeItem("selectedGroupId");
    }
    await loadGroups();
    setStatus(`Group "${groupName}" removed.`);
  } catch (error) {
    button.disabled = false;
    setStatus(error.message);
  }
});

loadGroups().catch((error) => setStatus(error.message));
