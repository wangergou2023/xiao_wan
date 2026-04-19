function updateSetupStatus(statusString) {
  const setupStatus = document.getElementById("setup-status");
  setupStatus.innerHTML = `<p>${statusString}</p>`;
}

function sendSetupInfo() {
  const button = document.activeElement instanceof HTMLButtonElement ? document.activeElement : null;
  if (button) {
    if (!button.dataset.originalLabel) {
      button.dataset.originalLabel = button.textContent.trim();
    }
    button.disabled = true;
    button.classList.add("is-busy");
    button.textContent = "初始化中...";
  }

  document.getElementById("config-options").style.display = "none";
  updateSetupStatus("正在初始化设置...");
  setConn();
}

function checkConn() {
  const connValue = document.getElementById("connSelection").value;
  document.getElementById("portViz").style.display = connValue === "ip" ? "block" : "none";
}

function setConn() {
  updateSetupStatus("正在设置连接方式（Escape Pod 或 IP）...");
  const connValue = document.getElementById("connSelection").value;
  let port = document.getElementById("portInput").value;
  port = port ? port : "443";
  const url = connValue === "ep" ? "/api-chipper/use_ep" : `/api-chipper/use_ip?port=${port}`;

  fetch(url)
    .then((response) => response.text())
    .then((response) => {
      if (response) {
        updateSetupStatus("设置完成！Wire-pod 已启动，正在跳转到主页...");
        setTimeout(() => window.location.href = "/", 3000);
      } else {
        updateSetupStatus("初始化 wire-pod 失败，请检查日志。");
        document.getElementById("config-options").style.display = "block";
        const button = document.querySelector("button.is-busy");
        if (button) {
          button.disabled = false;
          button.classList.remove("is-busy");
          button.textContent = button.dataset.originalLabel || button.textContent;
        }
      }
    });
}

function directToIndex() {
  window.location.href = "/index.html";
}
