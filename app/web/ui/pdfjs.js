// Lettore PDF (pdf.js, in web/vendor/pdfjs), caricato una volta sola quando serve. Modulo senza altre dipendenze: lo usano
// la sezione Manuali, la lettura delle schede dai manuali (manuale_scheda.js) e le pagine preparate (pagine_manuali.js).
let pdfjsPronto = null;

export function caricaPdfJs() {
  if (!pdfjsPronto) pdfjsPronto = new Promise((ok, ko) => {
    if (window.pdfjsLib) return ok(window.pdfjsLib);
    const s = document.createElement("script");
    s.src = "/vendor/pdfjs/pdf.min.js";
    s.onload = () => { window.pdfjsLib.GlobalWorkerOptions.workerSrc = "/vendor/pdfjs/pdf.worker.min.js"; ok(window.pdfjsLib); };
    s.onerror = () => { pdfjsPronto = null; ko(new Error("Impossibile caricare il lettore PDF.")); };
    document.head.append(s);
  });
  return pdfjsPronto;
}
