(() => {
  "use strict";

  const state = {
    printers: [],
    defaultPrinter: "",
    selectedFile: null,
    submitting: false,
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

  function showAlert(kind, message) {
    els.alertBanner.className = `alert-banner ${kind}`;
    els.alertBanner.textContent = message;
    els.alertBanner.hidden = false;
  }

  function clearAlert() {
    els.alertBanner.hidden = true;
    els.alertBanner.textContent = "";
  }

  async function checkHealth() {
    try {
      const res = await fetch("/api/health", { cache: "no-store" });
      if (res.ok) {
        els.healthPill.className = "health-pill online";
        els.healthText.textContent = "CUPS Online";
      } else {
        els.healthPill.className = "health-pill offline";
        els.healthText.textContent = "CUPS Offline";
      }
    } catch {
      els.healthPill.className = "health-pill offline";
      els.healthText.textContent = "CUPS Offline";
    }
  }

  function renderPrinterMeta() {
    const selectedName = els.printerSelect.value;
    const p = state.printers.find((item) => item.name === selectedName);
    els.printerMeta.innerHTML = "";
    if (!p) return;

    const badge = document.createElement("span");
    const st = p.state || "unknown";
    badge.className = `state-badge state-${st}`;
    badge.textContent = st.toUpperCase();
    els.printerMeta.appendChild(badge);

    if (p.is_default) {
      const defSpan = document.createElement("span");
      defSpan.textContent = "• System default";
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
        throw new Error(data.message || "Failed to load printers from CUPS");
      }

      state.printers = Array.isArray(data.printers) ? data.printers : [];
      state.defaultPrinter = data.default || "";

      els.printerSelect.innerHTML = "";

      if (state.printers.length === 0) {
        const opt = document.createElement("option");
        opt.value = "";
        opt.textContent = "No printers registered in CUPS";
        els.printerSelect.appendChild(opt);
      } else {
        for (const p of state.printers) {
          const opt = document.createElement("option");
          opt.value = p.name;
          const suffix = p.is_default ? " ★" : "";
          opt.textContent = `${p.name} (${p.state})${suffix}`;
          els.printerSelect.appendChild(opt);
        }

        const preferred =
          state.printers.find((p) => p.name === previousSelection)?.name ||
          state.defaultPrinter ||
          state.printers[0].name;
        els.printerSelect.value = preferred;
      }

      renderPrinterMeta();
      if (showFeedback) {
        showAlert("success", "Printer list refreshed.");
      }
    } catch (err) {
      els.printerSelect.innerHTML = "";
      const opt = document.createElement("option");
      opt.value = "";
      opt.textContent = "Unavailable";
      els.printerSelect.appendChild(opt);
      els.printerMeta.innerHTML = "";
      showAlert("error", err.message);
    } finally {
      els.refreshPrintersBtn.disabled = false;
    }
  }

  function setSelectedFile(file) {
    clearAlert();
    if (!file) {
      state.selectedFile = null;
      els.fileInput.value = "";
      els.fileCard.hidden = true;
      return;
    }
    state.selectedFile = file;
    els.fileName.textContent = file.name;
    const mimeLabel = file.type || "unknown type";
    els.fileDetails.textContent = `${mimeLabel} • ${formatBytes(file.size)}`;
    els.fileCard.hidden = false;
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
      showAlert("warning", "Please select a document to print.");
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
    els.submitBtn.disabled = true;
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
      els.submitBtn.disabled = false;
      els.progressWrap.hidden = true;

      let payload = {};
      try {
        payload = JSON.parse(xhr.responseText);
      } catch {
        payload = {};
      }

      if (xhr.status >= 200 && xhr.status < 300) {
        const jobInfo = payload.job_id ? ` (${payload.job_id})` : "";
        showAlert("success", `Print job queued on ${payload.printer}${jobInfo}.`);
        setSelectedFile(null);
      } else {
        showAlert("error", payload.message || "Print job failed.");
      }
    });

    xhr.addEventListener("error", () => {
      state.submitting = false;
      els.submitBtn.disabled = false;
      els.progressWrap.hidden = true;
      showAlert("error", "Network error while sending print job.");
    });

    xhr.send(formData);
  }

  function init() {
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
