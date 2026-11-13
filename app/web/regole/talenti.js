/* ===== TALENTI (regole 2014: PHB, supplemento, supplemento) =====
 [nome italiano, nome inglese, fonte, +1 ammessi (null = nessun +1, '*' = qualsiasi), effetti, descrizione breve]
 I nomi italiani sono indicativi: la scheda riconosce anche il nome inglese. Le descrizioni sono
 sintesi scritte per l'app, non il testo dei manuali.
 Effetti calcolati dalla scheda: init (iniziativa), speed (metri), hpLvl (PF per livello),
 pass (Percezione passiva), saveAb (TS nella caratteristica del +1), armor (competenze nelle
 armature: l/m/p/sc), medDex3 (Des fino a +3 con armatura media), unarmored (CA senza armatura
 13 + Des), res ([nome, valore] risorsa). */
const FEATS=[];
/* Ricerca per nome italiano o inglese, senza badare a maiuscole e accenti. */
const _fnorm=s=>String(s||'').toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g,'').replace(/[^a-z0-9]+/g,' ').trim();
const FEAT_IDX={};FEATS.forEach(f=>{FEAT_IDX[_fnorm(f[0])]=f;FEAT_IDX[_fnorm(f[1])]=f;});
function featInfo(n){const f=FEAT_IDX[_fnorm(n)];return f?{it:f[0],en:f[1],src:f[2],ab:f[3],fx:f[4],d:f[5]}:null;}
