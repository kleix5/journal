const state = {
  groups: [],
  selectedGroupId: "",
  lesson: null,
};

const groupSelect = document.getElementById("group-select");
const groupSubject = document.getElementById("group-subject");
const lessonButton = document.getElementById("lesson-button");
const statusNode = document.getElementById("status");
const groupTitle = document.getElementById("group-title");
const lessonDate = document.getElementById("lesson-date");
const emptyState = document.getElementById("empty-state");
const lessonForm = document.getElementById("lesson-form");
const lessonTheme = document.getElementById("lesson-theme");
const lessonTerm = document.getElementById("lesson-term");
const lessonBody = document.getElementById("lesson-body");
const lessonSessionDate = document.getElementById("lesson-session-date");
const existingLessonSelect = document.getElementById("existing-lesson-select");
const editLessonButton = document.getElementById("edit-lesson-button");

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

function selectedLessonDate() {
  return lessonSessionDate.value || currentDate();
}

function journalURL(groupID, lessonDateValue) {
  const params = new URLSearchParams();
  params.set("group", groupID);
  params.set("date", lessonDateValue);
  return `/journal.html?${params.toString()}`;
}

function initialSelectedGroupId(groups) {
  const queryId = new URLSearchParams(window.location.search).get("group");
  const savedId = localStorage.getItem("selectedGroupId");
  const candidate = queryId || savedId || groups[0]?.id || "";
  return groups.find((group) => group.id === candidate) ? candidate : groups[0]?.id || "";
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

function syncCurrentLesson() {
  const group = selectedGroup();
  if (!group) {
    state.lesson = null;
    return;
  }

  state.lesson = group.lessons.find((lesson) => lesson.date === selectedLessonDate()) || null;
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

function renderExistingLessons(group) {
  existingLessonSelect.innerHTML = "";

  if (!group) {
    const option = document.createElement("option");
    option.value = "";
    option.textContent = "No group selected";
    existingLessonSelect.appendChild(option);
    existingLessonSelect.disabled = true;
    editLessonButton.disabled = true;
    return;
  }

  const lessons = [...group.lessons].sort((left, right) => right.date.localeCompare(left.date));
  if (!lessons.length) {
    const option = document.createElement("option");
    option.value = "";
    option.textContent = "No saved lessons";
    existingLessonSelect.appendChild(option);
    existingLessonSelect.disabled = true;
    editLessonButton.disabled = true;
    return;
  }

  existingLessonSelect.disabled = false;
  editLessonButton.disabled = false;

  lessons.forEach((lesson) => {
    const option = document.createElement("option");
    option.value = lesson.date;
    option.textContent = lesson.theme ? `${lesson.date} - ${lesson.theme}` : lesson.date;
    option.selected = lesson.date === selectedLessonDate();
    existingLessonSelect.appendChild(option);
  });
}

function renderLessonRows(group) {
  lessonBody.innerHTML = "";

  group.students.forEach((student) => {
    const record = state.lesson?.records?.[student.id] || { present: false, score: null };
    const row = document.createElement("tr");
    row.innerHTML = `
      <th>${escapeHTML(student.name)}</th>
      <td><input type="checkbox" data-student-id="${student.id}" data-field="present" ${record.present ? "checked" : ""}></td>
      <td><input type="number" min="1" max="5" step="1" inputmode="numeric" data-student-id="${student.id}" data-field="score" value="${record.score ?? ""}" placeholder="-"></td>
    `;
    lessonBody.appendChild(row);
  });
}

function render() {
  if (!lessonSessionDate.value) {
    lessonSessionDate.value = currentDate();
  }
  renderSelect();
  syncCurrentLesson();

  const group = selectedGroup();
  if (!group) {
    groupTitle.textContent = "No group selected";
    groupSubject.textContent = "-";
    lessonDate.textContent = "";
    renderExistingLessons(null);
    emptyState.textContent = "Create a group first on the group setup page.";
    emptyState.classList.remove("hidden");
    lessonForm.classList.add("hidden");
    return;
  }

  groupTitle.textContent = group.name;
  groupSubject.textContent = group.subject;
  renderExistingLessons(group);

  if (!state.lesson) {
    lessonDate.textContent = "";
    emptyState.textContent = "Open a lesson session for the chosen date and group.";
    emptyState.classList.remove("hidden");
    lessonForm.classList.add("hidden");
    lessonTheme.value = "";
    lessonTerm.value = "";
    lessonBody.innerHTML = "";
    return;
  }

  lessonDate.textContent = `Lesson date: ${state.lesson.date}`;
  lessonTheme.value = state.lesson.theme || "";
  lessonTerm.value = state.lesson.term || "";
  renderLessonRows(group);
  emptyState.classList.add("hidden");
  lessonForm.classList.remove("hidden");
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

groupSelect.addEventListener("change", () => {
  state.selectedGroupId = groupSelect.value;
  localStorage.setItem("selectedGroupId", state.selectedGroupId);
  render();
  setStatus("");
});

lessonButton.addEventListener("click", async () => {
  if (!state.selectedGroupId) {
    setStatus("Choose a group first.");
    return;
  }

  try {
    const lessonDateValue = selectedLessonDate();
    await request(`/api/groups/${state.selectedGroupId}/lessons`, {
      method: "POST",
      body: JSON.stringify({ date: lessonDateValue }),
    });
    localStorage.setItem("selectedGroupId", state.selectedGroupId);
    window.location.assign(journalURL(state.selectedGroupId, lessonDateValue));
  } catch (error) {
    setStatus(error.message);
  }
});

lessonSessionDate.addEventListener("change", () => {
  render();
  setStatus("");
});

editLessonButton.addEventListener("click", () => {
  if (!existingLessonSelect.value) {
    setStatus("Choose a saved lesson first.");
    return;
  }

  lessonSessionDate.value = existingLessonSelect.value;
  render();
  setStatus("Saved lesson loaded for editing.");
});

lessonForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const group = selectedGroup();
  if (!group || !state.lesson) {
    return;
  }

  const records = {};

  try {
    group.students.forEach((student) => {
      const present = lessonBody.querySelector(`input[data-student-id="${student.id}"][data-field="present"]`);
      const score = lessonBody.querySelector(`input[data-student-id="${student.id}"][data-field="score"]`);
      records[student.id] = {
        present: Boolean(present?.checked),
        score: readScoreInput(score),
      };
    });

    await request(`/api/groups/${group.id}/lessons/${state.lesson.date}/records`, {
      method: "PUT",
      body: JSON.stringify({
        theme: lessonTheme.value.trim(),
        term: lessonTerm.value.trim(),
        records,
      }),
    });

    await loadGroups();
    setStatus("Lesson saved.");
  } catch (error) {
    setStatus(error.message);
  }
});

loadGroups().catch((error) => setStatus(error.message));
