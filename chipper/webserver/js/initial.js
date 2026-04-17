function checkLanguage() {
  fetch("/api/get_stt_info")
    .then((response) => response.json())
    .then((parsed) => {
      const sectionLanguage = document.getElementById("section-language");
      const languageSelection = document.getElementById("languageSelection");

      if (parsed.provider !== "vosk" && parsed.provider !== "whisper.cpp") {
        console.log("stt provider is not vosk or whisper");
        sectionLanguage.style.display = "none";
        languageSelection.value = "en-US";
      } else {
        sectionLanguage.style.display = "block";
        console.log(parsed.language);
        languageSelection.value = "en-US";
      }
    });
}

function updateSetupStatus(statusString) {
  const setupStatus = document.getElementById("setup-status");
  setupStatus.innerHTML = `<p>${statusString}</p>`;
}

function sendSetupInfo() {
  document.getElementById("config-options").style.display = "none";
  updateSetupStatus("正在初始化设置...");

  const language = document.getElementById("languageSelection").value;
  const langData = { language };

  document.getElementById("languageSelectionDiv").style.display = "none";

  fetch("/api/set_stt_info", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(langData),
  })
    .then((response) => response.text())
    .then((response) => {
      if (response.includes("success")) {
        updateSetupStatus("语言设置成功。");
        setConn();
      } else if (response.includes("downloading")) {
        updateSetupStatus("正在下载语言模型...");
        var interval = setInterval(() => {
          fetch("/api/get_download_status")
            .then((response) => response.text())
            .then((statusText) => {
              updateSetupStatus(statusText);
              if (statusText.includes("success")) {
                updateSetupStatus("语言设置成功。");
                clearInterval(interval);
                setConn();
              } else if (statusText.includes("error")) {
                document.getElementById("config-options").style.display = "block";
                clearInterval(interval);
              } else if (statusText.includes("not downloading")) {
                updateSetupStatus("正在初始化语言模型下载...");
              }
            });
        }, 500);
      } else if (response.includes("vosk")) {
        console.log(response)
        setConn();
      } else if (response.includes("error")) {
        updateSetupStatus(response);
        document.getElementById("config-options").style.display = "block";
      }
    });
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
      }
    });
}

function directToIndex() {
  window.location.href = "/index.html";
}
