(function () {
  const currentUserEmail = localStorage.getItem("currentUserEmail");
  const path = window.location.pathname;
  const isLoginPage = path === "/" || path === "/index.html";

  if (isLoginPage && currentUserEmail) {
    window.location.replace("/manage.html");
    return;
  }

  if (!currentUserEmail) {
    return;
  }

  document.querySelectorAll('a[href="/"]').forEach((link) => {
    link.href = "/manage.html";
  });

  document.querySelectorAll(".topbar-action").forEach((link) => {
    link.textContent = "Open App";
    link.href = "/manage.html";
  });
})();
