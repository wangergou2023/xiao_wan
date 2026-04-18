var GetLog = false;

const getE = (element) => document.getElementById(element);

function checkInited() {
  fetch("/api/is_api_v3").then((response) => {
    if (!response.ok) {
      alert(
        "This webserver does not match with the wire-pod binary. Some functionality will be broken. There was either an error during the last update, or you did not precisely follow the update guide. https://github.com/kercre123/wire-pod/wiki/Things-to-Know#updating-wire-pod"
      );
    }
  });

  fetch("/api/get_config")
    .then((response) => response.json())
    .then((config) => {
      if (!config.pastinitialsetup) {
        window.location.href = "/initial.html";
      }
    });
}


function checkWeather() {
  getE("apiKeySpan").style.display = getE("weatherProvider").value ? "block" : "none";
}

function sendWeatherAPIKey() {
  const data = {
    provider: getE("weatherProvider").value,
    key: getE("apiKey").value,
  };

  displayMessage("addWeatherProviderAPIStatus", "Saving...");

  fetch("/api/set_weather_api", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  })
    .then((response) => response.text())
    .then((response) => {
      displayMessage("addWeatherProviderAPIStatus", response);
    });
}

function updateWeatherAPI() {
  fetch("/api/get_weather_api")
    .then((response) => response.json())
    .then((data) => {
      getE("weatherProvider").value = data.provider;
      getE("apiKey").value = data.key;
      checkWeather();
    });
}

function checkKG() {
  const provider = getE("kgProvider").value;
  const elements = [
    "bigModelInput",
    "customAIInput",
    "openAIInput",
    "llmDefaultsNote",
  ];

  elements.forEach((el) => (getE(el).style.display = "none"));

  if (provider) {
    if (provider === "openai") {
      getE("openAIInput").style.display = "block";
      getE("llmDefaultsNote").style.display = "block";
    } else if (provider === "bigmodel") {
      getE("bigModelInput").style.display = "block";
      getE("llmDefaultsNote").style.display = "block";
    } else if (provider === "custom") {
      getE("customAIInput").style.display = "block";
      getE("llmDefaultsNote").style.display = "block";
    }
  }
}

function sendKGAPIKey() {
  const provider = getE("kgProvider").value;
  const data = {
    enable: true,
    provider,
    key: "",
    model: "",
    intentgraph: false,
    openai_prompt: "",
    save_chat: true,
    commands_enable: true,
    endpoint: "",
  };
  if (provider === "openai") {
    data.key = getE("openaiKey").value;
    data.openai_prompt = getE("openAIPrompt").value;
    data.intentgraph = true
  } else if (provider === "bigmodel") {
    data.key = getE("bigmodelKey").value;
    data.model = getE("bigmodelModel").value;
    data.openai_prompt = getE("bigmodelPrompt").value;
    data.intentgraph = true
  } else if (provider === "custom") {
    data.key = getE("customKey").value;
    data.model = getE("customModel").value;
    data.openai_prompt = getE("customAIPrompt").value;
    data.endpoint = getE("customAIEndpoint").value;
    data.intentgraph = true
  } else {
    data.enable = false;
    data.save_chat = false;
    data.commands_enable = false;
  }

  fetch("/api/set_kg_api", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  })
    .then((response) => response.text())
    .then((response) => {
      displayMessage("addKGProviderAPIStatus", response);
      alert(response);
    });
}

function sendBigModelConfig() {
  const toNumber = (value, fallback) => {
    const parsed = parseFloat(value);
    return Number.isFinite(parsed) ? parsed : fallback;
  };

  const data = {
    key: getE("bigmodelSharedKey").value,
    asr_model: getE("bigmodelASRModel").value,
    llm_model: getE("bigmodelSharedLLMModel").value,
    tts_model: getE("bigmodelTTSModel").value,
    tts_voice: getE("bigmodelTTSVoice").value,
    tts_speed: toNumber(getE("bigmodelTTSSpeed").value, 1.0),
    tts_volume: toNumber(getE("bigmodelTTSVolume").value, 1.0),
  };

  fetch("/api/set_bigmodel_config", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  })
    .then((response) => response.text())
    .then((response) => {
      displayMessage("bigModelConfigStatus", response);
      alert(response);
    });
}

function sendVisionConfig() {
  const data = {
    enable_face_context: getE("visionEnableFaceContext").checked,
  };

  fetch("/api/set_vision_config", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  })
    .then((response) => response.text())
    .then((response) => {
      displayMessage("visionConfigStatus", response);
      alert(response);
    });
}

function deleteSavedChats() {
  if (confirm("确认删除所有已保存的对话记录吗？")) {
    fetch("/api/delete_chats")
      .then((response) => response.text())
      .then(() => {
        alert("已成功删除所有已保存的对话记录。");
      });
  }
}

function updateKGAPI() {
  fetch("/api/get_kg_api")
    .then((response) => response.json())
    .then((data) => {
      getE("kgProvider").value = data.provider;
      if (data.provider === "openai") {
        getE("openaiKey").value = data.key;
        getE("openAIPrompt").value = data.openai_prompt;
      } else if (data.provider === "bigmodel") {
        getE("bigmodelKey").value = data.key;
        getE("bigmodelModel").value = data.model;
        getE("bigmodelPrompt").value = data.openai_prompt;
      } else if (data.provider === "custom") {
        getE("customKey").value = data.key;
        getE("customModel").value = data.model;
        getE("customAIPrompt").value = data.openai_prompt;
        getE("customAIEndpoint").value = data.endpoint;
      }
      checkKG();
    });
}

function updateBigModelConfig() {
  fetch("/api/get_bigmodel_config")
    .then((response) => response.json())
    .then((data) => {
      getE("bigmodelSharedKey").value = data.key || "";
      getE("bigmodelASRModel").value = data.asr_model || "";
      getE("bigmodelSharedLLMModel").value = data.llm_model || "";
      getE("bigmodelTTSModel").value = data.tts_model || "";
      getE("bigmodelTTSVoice").value = data.tts_voice || "";
      getE("bigmodelTTSSpeed").value = data.tts_speed || "";
      getE("bigmodelTTSVolume").value = data.tts_volume || "";
    });
}

function updateVisionConfig() {
  fetch("/api/get_vision_config")
    .then((response) => response.json())
    .then((data) => {
      getE("visionEnableFaceContext").checked = !!data.enable_face_context;
    });
}

function updateLongTermMemory() {
  const esn = getE("memoryRobotESN").value || "";
  const query = esn ? `?esn=${encodeURIComponent(esn)}` : "";
  fetch(`/api/get_long_term_memory${query}`)
    .then((response) => response.json())
    .then((data) => {
      const select = getE("memoryRobotESN");
      const currentValue = data.esn || "";
      select.innerHTML = "";
      (data.robots || []).forEach((robotEsn) => {
        const option = document.createElement("option");
        option.value = robotEsn;
        option.text = robotEsn;
        if (robotEsn === currentValue) {
          option.selected = true;
        }
        select.appendChild(option);
      });

      const profile = data.profile || {};
      getE("memoryUserName").value = profile.user_name || "";
      getE("memoryOwnerName").value = profile.owner_name || "";
      getE("memoryNickname").value = profile.nickname || "";
      getE("memoryPreferredLanguage").value = profile.preferred_language || "";
      getE("memoryPreferredGreeting").value = profile.preferred_greeting || "";
      getE("memoryFavoriteTopics").value = (profile.favorite_topics || []).join(",");
      getE("memoryForbiddenTopics").value = (profile.forbidden_topics || []).join(",");
      getE("memoryFacts").value = (profile.facts || []).join("\n");
      getE("memoryManualNotes").value = data.manual_notes || "";
    });
}

function saveLongTermMemory() {
  const splitCSV = (value) =>
    value
      .split(",")
      .map((item) => item.trim())
      .filter((item) => item.length > 0);

  const splitLines = (value) =>
    value
      .split("\n")
      .map((item) => item.trim())
      .filter((item) => item.length > 0);

  const esn = getE("memoryRobotESN").value || "";
  const data = {
    esn,
    profile: {
      esn,
      user_name: getE("memoryUserName").value.trim(),
      owner_name: getE("memoryOwnerName").value.trim(),
      nickname: getE("memoryNickname").value.trim(),
      preferred_language: getE("memoryPreferredLanguage").value.trim(),
      preferred_greeting: getE("memoryPreferredGreeting").value.trim(),
      favorite_topics: splitCSV(getE("memoryFavoriteTopics").value),
      forbidden_topics: splitCSV(getE("memoryForbiddenTopics").value),
      facts: splitLines(getE("memoryFacts").value),
    },
    manual_notes: getE("memoryManualNotes").value,
  };

  fetch("/api/set_long_term_memory", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  })
    .then((response) => response.text())
    .then((response) => {
      displayMessage("memoryStatus", response);
      alert(response);
      updateLongTermMemory();
    });
}

function setSTTLanguage() {
  const data = { language: getE("languageSelection").value };

  displayMessage("languageStatus", "设置中...");

  fetch("/api/set_stt_info", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  })
    .then((response) => response.text())
    .then((response) => {
      if (response.includes("downloading")) {
        displayMessage("languageStatus", "正在下载模型...");
        updateSTTLanguageDownload();
      } else {
        displayMessage("languageStatus", response);
        getE("languageSelectionDiv").style.display = response.includes("success") ? "block" : "none";
      }
    });
}

function updateSTTLanguageDownload() {

  const interval = setInterval(() => {
    fetch("/api/get_download_status")
      .then((response) => response.text())
      .then((response) => {
        displayMessage("languageStatus", response.includes("not downloading") ? "正在初始化下载..." : response)
        if (response.includes("success") || response.includes("error")) {
          displayMessage("languageStatus", response);
          getE("languageSelectionDiv").style.display = "block";
          clearInterval(interval);
        }
      });
  }, 500);
}

function sendRestart() {
  fetch("/api/reset")
    .then((response) => response.text())
    .then((response) => {
      displayMessage("restartStatus", response);
    });
}

function displayMessage(elementId, message) {
  const element = getE(elementId);
  element.innerHTML = "";
  const p = document.createElement("p");
  p.textContent = message;
  element.appendChild(p);
}

function displayError(elementId, message) {
  const element = getE(elementId);
  element.innerHTML = "";
  const error = document.createElement("p");
  error.innerHTML = message;
  element.appendChild(error);
}

function updateColor(id) {
  const l_id = id.replace("section", "icon");
  const elements = document.getElementsByName("icon");

  elements.forEach((element) => {
    element.classList.remove("selectedicon");
    element.classList.add("nowselectedicon");
  });

  const targetElement = document.getElementById(l_id);
  targetElement.classList.remove("notselectedicon");
  targetElement.classList.add("selectedicon");
}


function showLog() {
  toggleVisibility(["section-log", "section-botauth", "section-version", "section-uicustomizer"], "section-log", "icon-Logs");
  logDivArea = getE("botTranscriptedTextArea");
  getE("logscrollbottom").checked = true;
  logP = document.createElement("p");
  GetLog = true
  const interval = setInterval(() => {
    if (!GetLog) {
      clearInterval(interval);
      return;
    }
    const url = getE("logdebug").checked ? "/api/get_debug_logs" : "/api/get_logs";
    fetch(url)
      .then((response) => response.text())
      .then((logs) => {
        logDivArea.innerHTML = logs || "No logs yet, you must say a command to Vector. (this updates automatically)";
        if (getE("logscrollbottom").checked) {
          logDivArea.scrollTop = logDivArea.scrollHeight;
        }
      });
  }, 500);
}

function checkUpdate() {
  displayMessage("cVersion", "Checking for updates...");
  displayMessage("aUpdate", "");
  displayMessage("cCommit", "");
  fetch("/api/get_version_info")
    // type VersionInfo struct {
    // 	FromSource      bool   `json:"fromsource"`
    // 	InstalledVer    string `json:"installedversion"`
    // 	InstalledCommit string `json:"installedcommit"`
    // 	CurrentVer      string `json:"currentver"`
    // 	CurrentCommit   string `json:"currentcommit"`
    // 	UpdateAvailable bool   `json:"avail"`
    // }
    .then((response) => response.text())
    .then((response) => {
      if (response.includes("error")) {
        // <p id="cVersion"></p>
        // <p style="display: none;" id="cCommit"></p>
        // <p id="aUpdate"></p>
        displayMessage(
          "cVersion",
          "There was an error: " + response
        );
        getE("updateGuideLink").style.display = "none";
      } else {
        const parsed = JSON.parse(response);
        if (parsed.fromsource) {
          if (!parsed.avail) {
            displayMessage("aUpdate", `You are on the latest version.`);
            getE("updateGuideLink").style.display = "none";
          } else {
            displayMessage("aUpdate", `A newer version of WirePod (commit: ${parsed.currentcommit}) is available! Use this guide to update WirePod: `);
            getE("updateGuideLink").style.display = "block";
          }
          displayMessage("cVersion", `Installed Commit: ${parsed.installedcommit}`);
        } else {
          displayMessage("cVersion", `Installed Version: ${parsed.installedversion}`);
          displayMessage("cCommit", `Based on wire-pod commit: ${parsed.installedcommit}`);
          getE("cCommit").style.display = "block";
          if (parsed.avail) {
            displayMessage("aUpdate", `A newer version of WirePod (${parsed.currentversion}) is available! Use this guide to update WirePod: `);
            getE("updateGuideLink").style.display = "block";
          } else {
            displayMessage("aUpdate", "You are on the latest version.");
            getE("updateGuideLink").style.display = "none";
          }
        }
      }
    });
}

function showLanguage() {
  toggleVisibility(["section-weather", "section-restart", "section-kg", "section-language", "section-memory"], "section-language", "icon-Language");
  fetch("/api/get_stt_info")
    .then((response) => response.json())
    .then((parsed) => {
      if (parsed.provider !== "vosk" && parsed.provider !== "whisper.cpp") {
        displayError("languageStatus", `To set the STT language, the provider must be Vosk or Whisper. The current one is '${parsed.sttProvider}'.`);
        getE("languageSelectionDiv").style.display = "none";
      } else {
        getE("languageSelectionDiv").style.display = "block";
        getE("languageSelection").value = parsed.language;
      }
    });
}

function showVersion() {
  toggleVisibility(["section-log", "section-botauth", "section-version", "section-uicustomizer"], "section-version", "icon-Version");
  checkUpdate();
}

function showWeather() {
  toggleVisibility(["section-weather", "section-restart", "section-language", "section-kg", "section-memory"], "section-weather", "icon-Weather");
}

function showKG() {
  toggleVisibility(["section-weather", "section-restart", "section-language", "section-kg", "section-memory"], "section-kg", "icon-KG");
}

function showMemory() {
  toggleVisibility(["section-weather", "section-restart", "section-language", "section-kg", "section-memory"], "section-memory", "icon-Memory");
  updateLongTermMemory();
}

function toggleVisibility(sections, sectionToShow, iconId) {
  if (sectionToShow != "section-log") {
    GetLog = false;
  }
  sections.forEach((section) => {
    getE(section).style.display = "none";
  });
  getE(sectionToShow).style.display = "block";
  updateColor(iconId);
}
