var GetLog = false;

const getE = (element) => document.getElementById(element);

function getActiveButton() {
  const active = document.activeElement;
  return active instanceof HTMLButtonElement ? active : null;
}

function setButtonBusy(button, busy, busyLabel) {
  if (!button) {
    return;
  }
  if (busy) {
    if (!button.dataset.originalLabel) {
      button.dataset.originalLabel = button.textContent.trim();
    }
    button.disabled = true;
    button.classList.add("is-busy");
    button.textContent = busyLabel || "处理中...";
  } else {
    button.disabled = false;
    button.classList.remove("is-busy");
    button.textContent = button.dataset.originalLabel || button.textContent;
  }
}

function inferStatusType(message) {
  const normalized = String(message || "").toLowerCase();
  if (normalized.includes("error") || normalized.includes("failed") || normalized.includes("unable") || normalized.includes("失败") || normalized.includes("错误")) {
    return "error";
  }
  if (normalized.includes("download") || normalized.includes("saving") || normalized.includes("initializing") || normalized.includes("设置中") || normalized.includes("下载") || normalized.includes("处理中")) {
    return "warning";
  }
  if (normalized.includes("success") || normalized.includes("saved") || normalized.includes("done") || normalized.includes("完成") || normalized.includes("成功")) {
    return "success";
  }
  return "info";
}

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
  const button = getActiveButton();
  const data = {
    provider: getE("weatherProvider").value,
    key: getE("apiKey").value,
  };

  displayMessage("addWeatherProviderAPIStatus", "正在保存天气配置...");
  setButtonBusy(button, true, "保存中...");

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
    })
    .catch((error) => {
      displayError("addWeatherProviderAPIStatus", `保存天气配置失败：${error}`);
    })
    .finally(() => {
      setButtonBusy(button, false);
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

  elements.forEach((el) => {
    getE(el).style.display = "none";
    getE(el).classList.remove("is-visible");
  });

  if (provider) {
    if (provider === "openai") {
      getE("openAIInput").style.display = "block";
      getE("openAIInput").classList.add("is-visible");
      getE("llmDefaultsNote").style.display = "block";
    } else if (provider === "bigmodel") {
      getE("bigModelInput").style.display = "block";
      getE("bigModelInput").classList.add("is-visible");
      getE("llmDefaultsNote").style.display = "block";
    } else if (provider === "custom") {
      getE("customAIInput").style.display = "block";
      getE("customAIInput").classList.add("is-visible");
      getE("llmDefaultsNote").style.display = "block";
    }
  }
}

function sendKGAPIKey() {
  const button = getActiveButton();
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

  displayMessage("addKGProviderAPIStatus", "正在保存大模型配置...");
  setButtonBusy(button, true, "保存中...");

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
    })
    .catch((error) => {
      displayError("addKGProviderAPIStatus", `保存大模型配置失败：${error}`);
    })
    .finally(() => {
      setButtonBusy(button, false);
    });
}

function sendBigModelConfig() {
  const button = getActiveButton();
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

  displayMessage("bigModelConfigStatus", "正在保存 BigModel 公共配置...");
  setButtonBusy(button, true, "保存中...");

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
    })
    .catch((error) => {
      displayError("bigModelConfigStatus", `保存 BigModel 公共配置失败：${error}`);
    })
    .finally(() => {
      setButtonBusy(button, false);
    });
}

function sendVisionConfig() {
  const button = getActiveButton();
  const data = {
    enable_face_context: getE("visionEnableFaceContext").checked,
  };

  displayMessage("visionConfigStatus", "正在保存视觉设置...");
  setButtonBusy(button, true, "保存中...");

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
    })
    .catch((error) => {
      displayError("visionConfigStatus", `保存视觉设置失败：${error}`);
    })
    .finally(() => {
      setButtonBusy(button, false);
    });
}

function deleteSavedChats() {
  if (confirm("确认删除所有已保存的对话记录吗？")) {
    fetch("/api/delete_chats")
      .then((response) => response.text())
      .then(() => {
        displayMessage("addKGProviderAPIStatus", "已成功删除所有已保存的对话记录。");
      })
      .catch((error) => {
        displayError("addKGProviderAPIStatus", `删除已保存对话失败：${error}`);
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

      getE("memoryText").value = data.memory_text || "";
    });
}

function saveLongTermMemory() {
  const button = getActiveButton();
  const esn = getE("memoryRobotESN").value || "";
  const data = {
    esn,
    memory_text: getE("memoryText").value,
  };

  displayMessage("memoryStatus", "正在保存长期记忆...");
  setButtonBusy(button, true, "保存中...");

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
      updateLongTermMemory();
    })
    .catch((error) => {
      displayError("memoryStatus", `保存长期记忆失败：${error}`);
    })
    .finally(() => {
      setButtonBusy(button, false);
    });
}

function sendRestart() {
  const button = getActiveButton();
  displayMessage("restartStatus", "正在请求重启 wire-pod...");
  setButtonBusy(button, true, "重启中...");
  fetch("/api/reset")
    .then((response) => response.text())
    .then((response) => {
      displayMessage("restartStatus", response);
    })
    .catch((error) => {
      displayError("restartStatus", `请求重启失败：${error}`);
    })
    .finally(() => {
      setButtonBusy(button, false);
    });
}

function displayMessage(elementId, message) {
  const element = getE(elementId);
  if (!element) {
    return;
  }
  element.innerHTML = "";
  element.classList.remove("status-info", "status-success", "status-warning", "status-error");
  element.classList.add(`status-${inferStatusType(message)}`);
  const p = document.createElement("p");
  p.textContent = message;
  element.appendChild(p);
}

function displayError(elementId, message) {
  const element = getE(elementId);
  if (!element) {
    return;
  }
  element.innerHTML = "";
  element.classList.remove("status-info", "status-success", "status-warning", "status-error");
  element.classList.add("status-error");
  const error = document.createElement("p");
  error.innerHTML = message;
  element.appendChild(error);
}

function updateColor(id) {
  const l_id = id.replace("section", "icon");
  const elements = Array.from(document.getElementsByName("icon"));

  elements.forEach((element) => {
    element.classList.remove("selectedicon");
    element.classList.add("nowselectedicon");
  });

  const targetElement = document.getElementById(l_id);
  if (!targetElement) {
    return;
  }
  targetElement.classList.remove("notselectedicon");
  targetElement.classList.add("selectedicon");
}

function syncSectionTabs(sectionToShow) {
  const tabs = document.querySelectorAll(".minimal-tab[data-section], .settings-jump[data-section]");
  tabs.forEach((tab) => {
    const isActive = tab.dataset.section === sectionToShow;
    tab.classList.toggle("is-active", isActive);
    tab.setAttribute("aria-current", isActive ? "page" : "false");
  });
}

function clearSectionIcons() {
  const icons = Array.from(document.getElementsByName("icon"));
  icons.forEach((icon) => {
    icon.classList.remove("selectedicon");
    icon.classList.add("nowselectedicon");
  });
}


function showLog() {
  toggleVisibility(["section-log", "section-botauth"], "section-log", "icon-Logs");
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

function showWeather() {
  toggleVisibility(["section-weather", "section-restart", "section-kg", "section-memory"], "section-weather", "icon-Weather");
}

function showKG() {
  toggleVisibility(["section-weather", "section-restart", "section-kg", "section-memory"], "section-kg", "icon-KG");
}

function showMemory() {
  toggleVisibility(["section-weather", "section-restart", "section-kg", "section-memory"], "section-memory", "icon-Memory");
  updateLongTermMemory();
}

function showRestart() {
  toggleVisibility(["section-weather", "section-restart", "section-kg", "section-memory"], "section-restart", "icon-Restart");
}

function showSetupHome() {
  ["section-weather", "section-restart", "section-kg", "section-memory"].forEach((section) => {
    if (getE(section)) {
      getE(section).style.display = "none";
    }
  });
  GetLog = false;
  syncSectionTabs("");
  clearSectionIcons();
}

function toggleVisibility(sections, sectionToShow, iconId) {
  if (sectionToShow != "section-log") {
    GetLog = false;
  }
  sections.forEach((section) => {
    getE(section).style.display = "none";
  });
  getE(sectionToShow).style.display = "block";
  syncSectionTabs(sectionToShow);
  updateColor(iconId);
}

document.addEventListener("DOMContentLoaded", () => {
  if (getE("section-kg") && getE("section-weather") && getE("section-memory")) {
    showKG();
  }
});
