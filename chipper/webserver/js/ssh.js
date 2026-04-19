function updateSSHStatus(statusString, isError = false) {
  const setupStatus = document.getElementById("oskrSetupProgress");
  setupStatus.innerHTML = "";
  setupStatus.classList.remove("status-info", "status-success", "status-warning", "status-error");
  if (isError || statusString.toLowerCase().includes("error")) {
    setupStatus.classList.add("status-error");
  } else if (statusString.toLowerCase().includes("complete") || statusString.toLowerCase().includes("done")) {
    setupStatus.classList.add("status-success");
  } else {
    setupStatus.classList.add("status-warning");
  }
  const setupStatusP = document.createElement("p");
  setupStatusP.innerHTML = statusString;
  setupStatus.appendChild(setupStatusP);
}

function doSSHSetup() {
  const button = document.activeElement instanceof HTMLButtonElement ? document.activeElement : null;
  if (button) {
    if (!button.dataset.originalLabel) {
      button.dataset.originalLabel = button.textContent.trim();
    }
    button.disabled = true;
    button.classList.add("is-busy");
    button.textContent = "设置中...";
  }

  const ip = document.getElementById("sshIp").value;
  const key = document.getElementById("sshKeyFile").files[0];

  if (ip && key) {
    const formData = new FormData();
    formData.append("key", key);
    formData.append("ip", ip);

    fetch("/api-ssh/setup", {
      method: "POST",
      body: formData,
    })
      .then((response) => response.text())
      .then((response) => {
        if (response.includes("running")) {
          document.getElementById("oskrSetup").style.display = "none";
          updateSSHSetup();
          return;
        } else {
          updateSSHStatus(response);
          if (button) {
            button.disabled = false;
            button.classList.remove("is-busy");
            button.textContent = button.dataset.originalLabel || button.textContent;
          }
        }
      })
      .catch((error) => {
        updateSSHStatus(`设置机器人失败：${error}`);
        if (button) {
          button.disabled = false;
          button.classList.remove("is-busy");
          button.textContent = button.dataset.originalLabel || button.textContent;
        }
      });
  } else {
    updateSSHStatus("你需要填写 IP 地址并上传 SSH Key。", true);
    if (button) {
      button.disabled = false;
      button.classList.remove("is-busy");
      button.textContent = button.dataset.originalLabel || button.textContent;
    }
  }
}

function updateSSHSetup() {
  interval = setInterval(function () {
    fetch("/api-ssh/get_setup_status")
      .then((response) => response.text())
      .then((response) => {
        statusText = response;
        if (response.includes("done")) {
          updateSSHStatus(
            "File transfer complete! Use the above section to complete bot setup. The bot should eventually be on the onboarding screen."
          );
          document.getElementById("oskrSetup").style.display = "block";
          const button = document.querySelector("button.is-busy");
          if (button) {
            button.disabled = false;
            button.classList.remove("is-busy");
            button.textContent = button.dataset.originalLabel || button.textContent;
          }
          clearInterval(interval);
        } else if (response.includes("error")) {
          resp = response;
          if (response.includes("no route to host")) {
            resp =
              "Wire-pod was unable to connect to the robot. Make sure the robot is running OSKR/dev software and that it is on the same network as this wire-pod instance. Also double-check the IP.";
          }
          updateSSHStatus(resp, true);
          clearInterval(interval);
          document.getElementById("oskrSetup").style.display = "block";
          const button = document.querySelector("button.is-busy");
          if (button) {
            button.disabled = false;
            button.classList.remove("is-busy");
            button.textContent = button.dataset.originalLabel || button.textContent;
          }
          return;
        } else if (response.includes("not running")) {
          updateSSHStatus("Initiating SSH transfer...");
        } else {
          updateSSHStatus(response);
        }
      });
  }, 500);
}
