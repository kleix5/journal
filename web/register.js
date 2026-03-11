const registerForm = document.getElementById("register-form");
const statusNode = document.getElementById("status");

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

registerForm.addEventListener("submit", async (event) => {
  event.preventDefault();

  const formData = new FormData(registerForm);
  const email = String(formData.get("email") || "").trim();
  const password = String(formData.get("password") || "");

  try {
    const user = await request("/api/register", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    });

    registerForm.reset();
    setStatus(`Account created for ${user.email}.`);
  } catch (error) {
    setStatus(error.message);
  }
});
