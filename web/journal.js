const state = {
  groups: [],
  selectedGroupId: "",
};

const groupSelect = document.getElementById("group-select");
const groupSubject = document.getElementById("group-subject");
const statusNode = document.getElementById("status");
const groupTitle = document.getElementById("group-title");
const lessonDate = document.getElementById("lesson-date");
const emptyState = document.getElementById("empty-state");
const recordsForm = document.getElementById("records-form");
const journalHeadRow = document.getElementById("journal-head-row");
const journalBody = document.getElementById("journal-body");
const exportButton = document.getElementById("export-button");
const importFile = document.getElementById("import-file");
const importButton = document.getElementById("import-button");

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

function selectedGroup() {
  return state.groups.find((group) => group.id === state.selectedGroupId) || null;
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function currentDate() {
  return new Date().toISOString().slice(0, 10);
}

function sortedLessons(group) {
  return [...group.lessons].sort((left, right) => left.date.localeCompare(right.date));
}

function renderSelect() {
  groupSelect.innerHTML = "";
  if (!state.groups.length) {
    const option = document.createElement("option");
    option.textContent = "No groups";
    option.value = "";
    groupSelect.appendChild(option);
    groupSelect.disabled = true;
    return;
  }

  groupSelect.disabled = false;
  state.groups.forEach((group) => {
    const option = document.createElement("option");
    option.value = group.id;
    option.textContent = `${group.name} (${group.subject})`;
    option.selected = group.id === state.selectedGroupId;
    groupSelect.appendChild(option);
  });
}

function renderJournalTable(group) {
  const lessons = sortedLessons(group);
  journalHeadRow.innerHTML = `<th>Student</th>${lessons.map((lesson) => `<th>${lesson.date}</th>`).join("")}`;

  const themeRow = `
    <tr>
      <th>Theme</th>
      ${lessons
        .map(
          (lesson) => `
            <td>
              <textarea
                class="comment-input"
                rows="3"
                data-date="${lesson.date}"
                data-field="theme"
                placeholder="Lesson theme"
              >${escapeHTML(lesson.theme || "")}</textarea>
            </td>
          `,
        )
        .join("")}
    </tr>
  `;

  const termRow = `
    <tr>
      <th>Term</th>
      ${lessons
        .map(
          (lesson) => `
            <td>
              <input
                type="text"
                data-date="${lesson.date}"
                data-field="term"
                value="${escapeHTML(lesson.term || "")}"
                placeholder="Quarter 3"
              >
            </td>
          `,
        )
        .join("")}
    </tr>
  `;

  const studentRows = group.students
    .map((student) => {
      const cells = lessons
        .map((lesson) => {
          const record = lesson.records[student.id] || { present: false, score: null };
          const scoreValue = record.score ?? "";
          return `
            <td>
              <label class="cell-block">
                <span class="cell-label">Present</span>
                <input
                  type="checkbox"
                  data-date="${lesson.date}"
                  data-student-id="${student.id}"
                  data-field="present"
                  ${record.present ? "checked" : ""}
                >
              </label>
              <label class="cell-block">
                <span class="cell-label">Score</span>
                <input
                  type="number"
                  min="1"
                  max="5"
                  step="1"
                  inputmode="numeric"
                  data-date="${lesson.date}"
                  data-student-id="${student.id}"
                  data-field="score"
                  value="${scoreValue}"
                  placeholder="-"
                >
              </label>
            </td>
          `;
        })
        .join("");

      return `<tr><th>${escapeHTML(student.name)}</th>${cells}</tr>`;
    })
    .join("");

  journalBody.innerHTML = themeRow + termRow + studentRows;
}

function renderRecords() {
  const group = selectedGroup();
  if (!group) {
    groupTitle.textContent = "No group selected";
    groupSubject.textContent = "-";
    lessonDate.textContent = "";
    emptyState.textContent = "Create a group first on the group setup page.";
    emptyState.classList.remove("hidden");
    recordsForm.classList.add("hidden");
    return;
  }

  groupTitle.textContent = group.name;
  groupSubject.textContent = group.subject;

  const lessons = sortedLessons(group);
  if (!lessons.length) {
    lessonDate.textContent = "";
    emptyState.textContent = "No lessons yet. Use the lesson session page to open today's lesson.";
    emptyState.classList.remove("hidden");
    recordsForm.classList.add("hidden");
    return;
  }

  const todayLesson = lessons.find((lesson) => lesson.date === currentDate());
  lessonDate.textContent = todayLesson ? `Today's lesson: ${todayLesson.date}` : `Last lesson: ${lessons[lessons.length - 1].date}`;
  renderJournalTable(group);
  emptyState.classList.add("hidden");
  recordsForm.classList.remove("hidden");
}

function render() {
  renderSelect();
  renderRecords();
}

function initialSelectedGroupId(groups) {
  const queryId = new URLSearchParams(window.location.search).get("group");
  const savedId = localStorage.getItem("selectedGroupId");
  const candidate = queryId || savedId || groups[0]?.id || "";
  return groups.find((group) => group.id === candidate) ? candidate : groups[0]?.id || "";
}

async function loadGroups() {
  const data = await request("/api/groups");
  state.groups = data.groups || [];
  state.selectedGroupId = initialSelectedGroupId(state.groups);
  if (state.selectedGroupId) {
    localStorage.setItem("selectedGroupId", state.selectedGroupId);
  }
  render();
}

function readScoreInput(input) {
  const value = String(input.value || "").trim();
  if (!value) {
    return null;
  }

  const score = Number(value);
  if (!Number.isInteger(score) || score < 1 || score > 5) {
    throw new Error("Scores must be integers between 1 and 5.");
  }
  return score;
}

groupSelect.addEventListener("change", () => {
  state.selectedGroupId = groupSelect.value;
  localStorage.setItem("selectedGroupId", state.selectedGroupId);
  render();
  setStatus("");
});

recordsForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const group = selectedGroup();
  if (!group) {
    return;
  }

  const lessons = sortedLessons(group);

  try {
    for (const lesson of lessons) {
      const records = {};
      for (const student of group.students) {
        const present = journalBody.querySelector(
          `input[data-date="${lesson.date}"][data-student-id="${student.id}"][data-field="present"]`,
        );
        const score = journalBody.querySelector(
          `input[data-date="${lesson.date}"][data-student-id="${student.id}"][data-field="score"]`,
        );
        const parsedScore = readScoreInput(score);
        records[student.id] = {
          present: Boolean(present?.checked),
          score: parsedScore,
        };
      }

      const theme = journalBody.querySelector(`textarea[data-date="${lesson.date}"][data-field="theme"]`);
      const term = journalBody.querySelector(`input[data-date="${lesson.date}"][data-field="term"]`);
      await request(`/api/groups/${group.id}/lessons/${lesson.date}/records`, {
        method: "PUT",
        body: JSON.stringify({
          theme: String(theme?.value || "").trim(),
          term: String(term?.value || "").trim(),
          records,
        }),
      });
    }

    await loadGroups();
    setStatus("Journal saved.");
  } catch (error) {
    setStatus(error.message);
  }
});

exportButton.addEventListener("click", async () => {
  try {
    const response = await fetch("/api/journal/export");
    if (!response.ok) {
      const payload = await response.json().catch(() => ({}));
      throw new Error(payload.error || "Export failed");
    }

    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "journal-backup.json";
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
    setStatus("Journal backup saved as JSON.");
  } catch (error) {
    setStatus(error.message);
  }
});

importButton.addEventListener("click", async () => {
  if (!importFile.files?.length) {
    setStatus("Choose a saved JSON backup first.");
    return;
  }

  const formData = new FormData();
  formData.append("file", importFile.files[0]);

  try {
    const response = await fetch("/api/journal/import", {
      method: "POST",
      body: formData,
    });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      throw new Error(payload.error || "Restore failed");
    }

    importFile.value = "";
    await loadGroups();
    setStatus("Journal state restored from JSON backup.");
  } catch (error) {
    setStatus(error.message);
  }
});

loadGroups().catch((error) => setStatus(error.message));
