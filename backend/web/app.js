(() => {
  "use strict";

  const { t, setLang, onChange, applyDOMTranslations } = window.SheetI18n;

  const state = {
    printers: [],
    defaultPrinter: "",
    printersError: false,
    selectedFile: null,
    submitting: false,
    healthState: "checking", // "checking" | "online" | "offline"
    activeAlert: null,       // { kind, key, vars, raw }
  };

  const els = {
    healthPill: document.getElementById("healthPill"),
    healthText: document.getElementById("healthText"),
    alertBanner: document.getElementById("alertBanner"),
    printForm: document.getElementById("printForm"),
    printerSelect: document.getElementById("printerSelect"),
    refreshPrintersBtn: document.getElementById("refreshPrintersBtn"),
    printerMeta: document.getElementById("printerMeta"),
    dropzone: document.getElementById("dropzone"),
    fileInput: document.getElementById("fileInput"),
    fileCard: document.getElementById("fileCard"),
    fileName: document.getElementById("fileName"),
    fileDetails: document.getElementById("fileDetails"),
    clearFileBtn: document.getElementById("clearFileBtn"),
    copiesInput: document.getElementById("copiesInput"),
    colorSelect: document.getElementById("colorSelect"),
    pageRangeMode: document.getElementById("pageRangeMode"),
    pageRangeInput: document.getElementById("pageRangeInput"),
    sidesSelect: document.getElementById("sidesSelect"),
    orientationSelect: document.getElementById("orientationSelect"),
    progressWrap: document.getElementById("progressWrap"),
    progressLabel: document.getElementById("progressLabel"),
    progressPercent: document.getElementById("progressPercent"),
    progressBar: document.getElementById("progressBar"),
    submitBtn: document.getElementById("submitBtn"),
  };

  function formatBytes(bytes) {
    if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
    const units = ["B", "KB", "MB", "GB"];
    let idx = 0;
    let val = bytes;
    while (val >= 1024 && idx < units.length - 1) {
      val /= 1024;
      idx++;
    }
    return `${val.toFixed(idx === 0 ? 0 : 2)} ${units[idx]}`;
  }

  function translateState(st) {
    const key = `state_${st || "unknown"}`;
    return t(key);
  }

  function translateApiError(payload) {
    if (payload && payload.error) {
      const key = `err_${payload.error}`;
      const translated = t(key);
      if (translated !== key) return translated;
    }
    if (payload && payload.message) return payload.message;
    return t("err_print_failed");
  }

  function renderAlert() {
    if (!state.activeAlert) {
      els.alertBanner.hidden = true;
      els.alertBanner.textContent = "";
      return;
    }
    const { kind, key, vars, raw } = state.activeAlert;
    els.alertBanner.className = `alert-banner ${kind}`;
    els.alertBanner.textContent = key ? t(key, vars) : raw;
    els.alertBanner.hidden = false;
  }

  function showAlertKey(kind, key, vars = {}) {
    state.activeAlert = { kind, key, vars, raw: "" };
    renderAlert();
  }

  function showAlertRaw(kind, raw) {
    state.activeAlert = { kind, key: "", vars: {}, raw };
    renderAlert();
  }

  function clearAlert() {
    state.activeAlert = null;
    renderAlert();
  }

  function renderHealth() {
    if (state.healthState === "online") {
      els.healthPill.className = "health-pill online";
      els.healthText.textContent = t("health_online");
    } else if (state.healthState === "offline") {
      els.healthPill.className = "health-pill offline";
      els.healthText.textContent = t("health_offline");
    } else {
      els.healthPill.className = "health-pill";
      els.healthText.textContent = t("health_checking");
    }
  }

  async function checkHealth() {
    try {
      const res = await fetch("/api/health", { cache: "no-store" });
      state.healthState = res.ok ? "online" : "offline";
    } catch {
      state.healthState = "offline";
    }
    renderHealth();
  }

  function renderPrinterOptions(preferredSelection) {
    const currentVal = preferredSelection ?? els.printerSelect.value;
    els.printerSelect.innerHTML = "";

    if (state.printersError) {
      const opt = document.createElement("option");
      opt.value = "";
      opt.textContent = t("printer_unavailable");
      els.printerSelect.appendChild(opt);
      els.printerMeta.innerHTML = "";
      return;
    }

    if (state.printers.length === 0) {
      const opt = document.createElement("option");
      opt.value = "";
      opt.textContent = t("printer_none");
      els.printerSelect.appendChild(opt);
      els.printerMeta.innerHTML = "";
      return;
    }

    for (const p of state.printers) {
      const opt = document.createElement("option");
      opt.value = p.name;
      const suffix = p.is_default ? " ★" : "";
      opt.textContent = `${p.name} (${translateState(p.state)})${suffix}`;
      els.printerSelect.appendChild(opt);
    }

    const chosen =
      state.printers.find((p) => p.name === currentVal)?.name ||
      state.defaultPrinter ||
      state.printers[0].name;
    els.printerSelect.value = chosen;
    renderPrinterMeta();
  }

  function renderPrinterMeta() {
    const selectedName = els.printerSelect.value;
    const p = state.printers.find((item) => item.name === selectedName);
    els.printerMeta.innerHTML = "";
    if (!p) return;

    const badge = document.createElement("span");
    const st = p.state || "unknown";
    badge.className = `state-badge state-${st}`;
    badge.textContent = translateState(st);
    els.printerMeta.appendChild(badge);

    if (p.is_default) {
      const defSpan = document.createElement("span");
      defSpan.textContent = t("printer_default_badge");
      els.printerMeta.appendChild(defSpan);
    }

    if (p.reason) {
      const reasonSpan = document.createElement("span");
      reasonSpan.textContent = `• ${p.reason}`;
      els.printerMeta.appendChild(reasonSpan);
    }
  }

  async function loadPrinters(showFeedback = false) {
    els.refreshPrintersBtn.disabled = true;
    const previousSelection = els.printerSelect.value;

    try {
      const res = await fetch("/api/printers", { cache: "no-store" });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        state.printersError = true;
        renderPrinterOptions("");
        showAlertRaw("error", translateApiError(data));
        return;
      }

      state.printersError = false;
      state.printers = Array.isArray(data.printers) ? data.printers : [];
      state.defaultPrinter = data.default || "";
      renderPrinterOptions(previousSelection);

      if (showFeedback) {
        showAlertKey("success", "alert_printers_refreshed");
      }
    } catch {
      state.printersError = true;
      renderPrinterOptions("");
      showAlertKey("error", "alert_network_error");
    } finally {
      els.refreshPrintersBtn.disabled = false;
    }
  }

  function renderFileCard() {
    const file = state.selectedFile;
    if (!file) {
      els.fileCard.hidden = true;
      return;
    }
    els.fileName.textContent = file.name;
    const mimeLabel = file.type || t("unknown_mime");
    els.fileDetails.textContent = `${mimeLabel} • ${formatBytes(file.size)}`;
    els.fileCard.hidden = false;
  }

  function setSelectedFile(file) {
    clearAlert();
    if (!file) {
      state.selectedFile = null;
      els.fileInput.value = "";
      renderFileCard();
      return;
    }
    state.selectedFile = file;
    renderFileCard();
  }

  function renderSubmitButton() {
    els.submitBtn.disabled = state.submitting;
    els.submitBtn.textContent = state.submitting ? t("btn_submitting") : t("btn_submit");
  }

  function setupDropzone() {
    els.dropzone.addEventListener("click", () => els.fileInput.click());
    els.dropzone.addEventListener("keydown", (e) => {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        els.fileInput.click();
      }
    });

    els.fileInput.addEventListener("change", () => {
      if (els.fileInput.files && els.fileInput.files[0]) {
        setSelectedFile(els.fileInput.files[0]);
      }
    });

    ["dragenter", "dragover"].forEach((evt) => {
      els.dropzone.addEventListener(evt, (e) => {
        e.preventDefault();
        e.stopPropagation();
        els.dropzone.classList.add("dragover");
      });
    });

    ["dragleave", "drop"].forEach((evt) => {
      els.dropzone.addEventListener(evt, (e) => {
        e.preventDefault();
        e.stopPropagation();
        els.dropzone.classList.remove("dragover");
      });
    });

    els.dropzone.addEventListener("drop", (e) => {
      const files = e.dataTransfer?.files;
      if (files && files[0]) {
        setSelectedFile(files[0]);
      }
    });

    els.clearFileBtn.addEventListener("click", () => setSelectedFile(null));
  }

  function submitPrintJob(e) {
    e.preventDefault();
    if (state.submitting) return;
    clearAlert();

    if (!state.selectedFile) {
      showAlertKey("warning", "alert_select_file");
      return;
    }

    const formData = new FormData();
    formData.append("file", state.selectedFile);
    formData.append("printer", els.printerSelect.value);
    formData.append("copies", els.copiesInput.value);
    formData.append("color_mode", els.colorSelect.value);
    formData.append("sides", els.sidesSelect.value);
    formData.append("orientation", els.orientationSelect.value);

    if (els.pageRangeMode.value === "custom") {
      formData.append("page_range", els.pageRangeInput.value.trim());
    }

    state.submitting = true;
    renderSubmitButton();
    els.progressWrap.hidden = false;
    els.progressBar.style.width = "0%";
    els.progressPercent.textContent = "0%";

    const xhr = new XMLHttpRequest();
    xhr.open("POST", "/api/print");

    xhr.upload.addEventListener("progress", (evt) => {
      if (evt.lengthComputable) {
        const pct = Math.round((evt.loaded / evt.total) * 100);
        els.progressBar.style.width = `${pct}%`;
        els.progressPercent.textContent = `${pct}%`;
      }
    });

    xhr.addEventListener("load", () => {
      state.submitting = false;
      renderSubmitButton();
      els.progressWrap.hidden = true;

      let payload = {};
      try {
        payload = JSON.parse(xhr.responseText);
      } catch {
        payload = {};
      }

      if (xhr.status >= 200 && xhr.status < 300) {
        const jobSuffix = payload.job_id ? ` (${payload.job_id})` : "";
        setSelectedFile(null);
        showAlertKey("success", "alert_job_queued", {
          printer: payload.printer || "",
          job: jobSuffix,
        });
      } else {
        if (payload && payload.error && t(`err_${payload.error}`) !== `err_${payload.error}`) {
          showAlertKey("error", `err_${payload.error}`);
        } else {
          showAlertRaw("error", translateApiError(payload));
        }
      }
    });

    xhr.addEventListener("error", () => {
      state.submitting = false;
      renderSubmitButton();
      els.progressWrap.hidden = true;
      showAlertKey("error", "alert_network_error");
    });

    xhr.send(formData);
  }

  function setupLanguageButtons() {
    document.querySelectorAll("[data-lang-btn]").forEach((btn) => {
      btn.addEventListener("click", () => {
        setLang(btn.getAttribute("data-lang-btn"));
      });
    });

    onChange(() => {
      renderHealth();
      renderPrinterOptions();
      renderFileCard();
      renderSubmitButton();
      renderAlert();
    });
  }

  function init() {
    applyDOMTranslations();
    setupLanguageButtons();
    setupDropzone();

    els.printerSelect.addEventListener("change", renderPrinterMeta);
    els.refreshPrintersBtn.addEventListener("click", () => loadPrinters(true));
    els.pageRangeMode.addEventListener("change", () => {
      const isCustom = els.pageRangeMode.value === "custom";
      els.pageRangeInput.hidden = !isCustom;
      if (!isCustom) els.pageRangeInput.value = "";
    });
    els.printForm.addEventListener("submit", submitPrintJob);

    checkHealth();
    loadPrinters(false);
    setInterval(checkHealth, 30000);
  }

  document.addEventListener("DOMContentLoaded", init);
})();
