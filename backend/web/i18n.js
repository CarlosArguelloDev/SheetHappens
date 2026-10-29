(() => {
  "use strict";

  const STORAGE_KEY = "sheethappens_lang";

  const DICTIONARIES = {
    en: {
      page_title: "SheetHappens — Web Print Portal",
      brand_subtitle: "Universal CUPS Web Print Portal",
      health_tooltip: "CUPS Server Status",
      health_checking: "Checking CUPS…",
      health_online: "CUPS Online",
      health_offline: "CUPS Offline",
      lang_aria: "Switch language",

      label_printer: "Target Printer",
      printer_loading: "Loading printers…",
      printer_none: "No printers registered in CUPS",
      printer_unavailable: "Unavailable",
      printer_default_badge: "• System default",
      refresh_btn: "Refresh",
      refresh_tooltip: "Refresh printers list",

      state_idle: "Idle",
      state_processing: "Printing",
      state_stopped: "Stopped",
      state_unknown: "Unknown",

      label_document: "Document",
      dropzone_aria: "Upload document",
      dropzone_title: "Drag & drop your file here, or click to browse",
      dropzone_hint: "Allowed: PDF, PNG, JPEG, TXT (up to 20 MB)",
      btn_remove_file: "Remove",
      unknown_mime: "unknown type",

      label_copies: "Copies",
      label_color: "Color Mode",
      opt_color: "Color",
      opt_mono: "Grayscale / Monochrome",

      label_page_range: "Page Range",
      opt_pages_all: "All Pages",
      opt_pages_custom: "Custom Range",
      placeholder_page_range: "e.g. 1-3, 5, 8-10",

      label_sides: "Duplex / Sides",
      opt_one_sided: "One-Sided",
      opt_duplex_long: "Two-Sided (Long Edge)",
      opt_duplex_short: "Two-Sided (Short Edge)",

      label_orientation: "Orientation",
      opt_portrait: "Portrait",
      opt_landscape: "Landscape",

      progress_uploading: "Uploading & spooling…",
      btn_submit: "Print Document",
      btn_submitting: "Sending to printer…",
      footer_note: "SheetHappens • Powered by CUPS & Go • Multi-Printer Architecture",

      alert_printers_refreshed: "Printer list refreshed.",
      alert_select_file: "Please select a document to print.",
      alert_job_queued: "Print job queued on {printer}{job}.",
      alert_network_error: "Network error while communicating with the print server.",

      err_invalid_printer: "Invalid printer name.",
      err_printer_not_found: "Selected printer is not registered in CUPS.",
      err_printer_stopped: "Selected printer is currently stopped or paused.",
      err_no_default_printer: "No printer selected and no system default is configured.",
      err_invalid_copies: "Copies must be a number between 1 and 99.",
      err_invalid_color_mode: "Unsupported color mode selection.",
      err_invalid_page_range: "Invalid page range. Use a format like 1-3, 5, 8-10.",
      err_invalid_sides: "Unsupported duplex mode.",
      err_invalid_orientation: "Unsupported page orientation.",
      err_unsupported_type: "Unsupported file format. Only PDF, PNG, JPEG, and TXT are allowed.",
      err_empty_file: "The selected file is empty.",
      err_cups_unavailable: "CUPS print scheduler is unreachable.",
      err_print_failed: "CUPS rejected the print job.",
      err_internal_error: "Internal server error while processing the job.",
      err_missing_file: "Please attach a document to print.",
      err_file_too_large: "File exceeds the maximum allowed upload size.",
      err_invalid_request: "Invalid print request.",
      err_server_busy: "The print server is busy. Please try again in a few seconds.",
    },

    es: {
      page_title: "SheetHappens — Portal Web de Impresión",
      brand_subtitle: "Portal Universal de Impresión Web con CUPS",
      health_tooltip: "Estado del servidor CUPS",
      health_checking: "Verificando CUPS…",
      health_online: "CUPS en línea",
      health_offline: "CUPS sin conexión",
      lang_aria: "Cambiar idioma",

      label_printer: "Impresora de destino",
      printer_loading: "Cargando impresoras…",
      printer_none: "No hay impresoras registradas en CUPS",
      printer_unavailable: "No disponible",
      printer_default_badge: "• Predeterminada del sistema",
      refresh_btn: "Actualizar",
      refresh_tooltip: "Actualizar lista de impresoras",

      state_idle: "Inactiva",
      state_processing: "Imprimiendo",
      state_stopped: "Detenida",
      state_unknown: "Desconocido",

      label_document: "Documento",
      dropzone_aria: "Subir documento",
      dropzone_title: "Arrastra y suelta tu archivo aquí, o haz clic para explorar",
      dropzone_hint: "Permitidos: PDF, PNG, JPEG, TXT (hasta 20 MB)",
      btn_remove_file: "Quitar",
      unknown_mime: "tipo desconocido",

      label_copies: "Copias",
      label_color: "Modo de color",
      opt_color: "Color",
      opt_mono: "Escala de grises / Monocromo",

      label_page_range: "Rango de páginas",
      opt_pages_all: "Todas las páginas",
      opt_pages_custom: "Rango personalizado",
      placeholder_page_range: "ej. 1-3, 5, 8-10",

      label_sides: "Dúplex / Caras",
      opt_one_sided: "Una cara",
      opt_duplex_long: "Doble cara (borde largo)",
      opt_duplex_short: "Doble cara (borde corto)",

      label_orientation: "Orientación",
      opt_portrait: "Vertical",
      opt_landscape: "Horizontal",

      progress_uploading: "Subiendo y enviando a la cola…",
      btn_submit: "Imprimir documento",
      btn_submitting: "Enviando a la impresora…",
      footer_note: "SheetHappens • Impulsado por CUPS y Go • Arquitectura Multi-Impresora",

      alert_printers_refreshed: "Lista de impresoras actualizada.",
      alert_select_file: "Por favor selecciona un documento para imprimir.",
      alert_job_queued: "Trabajo enviado a la cola de {printer}{job}.",
      alert_network_error: "Error de red al comunicarse con el servidor de impresión.",

      err_invalid_printer: "Nombre de impresora inválido.",
      err_printer_not_found: "La impresora seleccionada no está registrada en CUPS.",
      err_printer_stopped: "La impresora seleccionada está detenida o en pausa.",
      err_no_default_printer: "No se seleccionó impresora y no hay una predeterminada configurada.",
      err_invalid_copies: "El número de copias debe estar entre 1 y 99.",
      err_invalid_color_mode: "Modo de color no compatible.",
      err_invalid_page_range: "Rango de páginas inválido. Usa un formato como 1-3, 5, 8-10.",
      err_invalid_sides: "Modo dúplex no compatible.",
      err_invalid_orientation: "Orientación de página no compatible.",
      err_unsupported_type: "Formato no permitido. Solo se aceptan archivos PDF, PNG, JPEG y TXT.",
      err_empty_file: "El archivo seleccionado está vacío.",
      err_cups_unavailable: "No se pudo conectar con el servidor de impresión CUPS.",
      err_print_failed: "CUPS rechazó el trabajo de impresión.",
      err_internal_error: "Error interno del servidor al procesar el trabajo.",
      err_missing_file: "Por favor adjunta un documento para imprimir.",
      err_file_too_large: "El archivo supera el tamaño máximo permitido.",
      err_invalid_request: "Solicitud de impresión inválida.",
      err_server_busy: "El servidor de impresión está ocupado. Inténtalo de nuevo en unos segundos.",
    },
  };

  function detectInitialLang() {
    try {
      const saved = localStorage.getItem(STORAGE_KEY);
      if (saved === "en" || saved === "es") return saved;
    } catch {
      // Ignore storage access errors in restricted contexts.
    }
    const nav = (navigator.language || "en").toLowerCase();
    return nav.startsWith("es") ? "es" : "en";
  }

  let currentLang = detectInitialLang();
  const listeners = [];

  function t(key, vars = {}) {
    const dict = DICTIONARIES[currentLang] || DICTIONARIES.en;
    let str = dict[key] ?? DICTIONARIES.en[key] ?? key;
    for (const [k, v] of Object.entries(vars)) {
      str = str.replaceAll(`{${k}}`, v);
    }
    return str;
  }

  function setLang(lang) {
    if (lang !== "en" && lang !== "es") return;
    currentLang = lang;
    try {
      localStorage.setItem(STORAGE_KEY, lang);
    } catch {
      // Ignore storage errors.
    }
    applyDOMTranslations();
    for (const fn of listeners) {
      fn(currentLang);
    }
  }

  function getLang() {
    return currentLang;
  }

  function onChange(fn) {
    listeners.push(fn);
  }

  function applyDOMTranslations() {
    document.documentElement.lang = currentLang;
    document.title = t("page_title");

    document.querySelectorAll("[data-i18n]").forEach((el) => {
      el.textContent = t(el.getAttribute("data-i18n"));
    });

    document.querySelectorAll("[data-i18n-placeholder]").forEach((el) => {
      el.setAttribute("placeholder", t(el.getAttribute("data-i18n-placeholder")));
    });

    document.querySelectorAll("[data-i18n-title]").forEach((el) => {
      el.setAttribute("title", t(el.getAttribute("data-i18n-title")));
    });

    document.querySelectorAll("[data-i18n-aria]").forEach((el) => {
      el.setAttribute("aria-label", t(el.getAttribute("data-i18n-aria")));
    });

    document.querySelectorAll("[data-lang-btn]").forEach((btn) => {
      const isActive = btn.getAttribute("data-lang-btn") === currentLang;
      btn.classList.toggle("active", isActive);
      btn.setAttribute("aria-pressed", String(isActive));
    });
  }

  window.SheetI18n = { t, setLang, getLang, onChange, applyDOMTranslations };
})();
