/* Dati delle regole quinta edizione usati dalla scheda e dal motore (regole/motore.js).
   Copia di 01_scheda/sorgenti: data.js, spells1-4.js, featdesc.js (sintesi in italiano). */
/* ===== DATI REGOLE quinta edizione (PHB 2014, supplemento, supplemento) ===== */
const AB=['FOR','DES','COS','INT','SAG','CAR'];
const ABN={FOR:'Forza',DES:'Destrezza',COS:'Costituzione',INT:'Intelligenza',SAG:'Saggezza',CAR:'Carisma'};
const ABS={FOR:'For',DES:'Des',COS:'Cos',INT:'Int',SAG:'Sag',CAR:'Car'};
const SKILLS=[['Acrobazia','DES'],['Addestrare Animali','SAG'],['Arcano','INT'],['Atletica','FOR'],['Furtività','DES'],['Indagare','INT'],['Inganno','CAR'],['Intimidire','CAR'],['Intrattenere','CAR'],['Intuizione','SAG'],['Medicina','SAG'],['Natura','INT'],['Percezione','SAG'],['Persuasione','CAR'],['Rapidità di Mano','DES'],['Religione','INT'],['Sopravvivenza','SAG'],['Storia','INT']];
const ALL_SK=SKILLS.map(s=>s[0]);
const XP=[0,300,900,2700,6500,14000,23000,34000,48000,64000,85000,100000,120000,140000,165000,195000,225000,265000,305000,355000];
const ASI_STD=[4,8,12,16,19];
const ASI_NAME='Aumento dei Punteggi di Caratteristica';

/* slot incantatore completo per livello incantatore 1..20 */
const FULL_SLOTS=[[],[2],[3],[4,2],[4,3],[4,3,2],[4,3,3],[4,3,3,1],[4,3,3,2],[4,3,3,3,1],[4,3,3,3,2],[4,3,3,3,2,1],[4,3,3,3,2,1],[4,3,3,3,2,1,1],[4,3,3,3,2,1,1],[4,3,3,3,2,1,1,1],[4,3,3,3,2,1,1,1],[4,3,3,3,2,1,1,1,1],[4,3,3,3,3,1,1,1,1],[4,3,3,3,3,2,1,1,1],[4,3,3,3,3,2,2,1,1]];
const KNOWN={};
const CANTRIPS={};
function cantripsKnown(id,lvl,sub){
 if(id==='artefice') return lvl>=14?4:lvl>=10?3:2;
 if(sub&&sub.x&&sub.x.cast==='third'){ if(lvl<3) return 0; const b=id==='ladro'?3:2; return lvl>=10?b+1:b; }
 const c=CANTRIPS[id]; if(!c) return 0; return c[0]+(lvl>=c[1]?1:0)+(lvl>=c[2]?1:0);
}

const DRAGONS=[];

/* armi: nome, categoria s/g, m(ischia)/d(istanza), danno, tipo, proprietà, flag (f accurata, l leggera, h pesante, 2 due mani, t lancio, v versatile, r portata) */
const WEAPONS=[];
/* armature: nome, tipo l/m/p, CA base, For richiesta, svantaggio furtività */
const ARMORS=[];

const STYLES=[];
const STYLES_T=[];

const FEATS_FX=[];

/* ===== CLASSI ===== */
const CLASSES={};

/* ===== RAZZE ===== */
/* asi: [[car,val]], flex: {n,v,excl}, skFix, skP, speed(m), size, dv(m scurovisione), lang, prof */
const RACES={};

/* ===== BACKGROUND (PHB) ===== */
const BGS={};
/* Liste di incantesimi aggiunte dai pacchetti (codice di una lettera → nome), lista di ogni classe (id → codice),
   nomi estesi delle fonti (codice → nome): vengono dai pacchetti di contenuti dell'utente. */
const NOMI_LISTE={};
const CODICI_LISTE={};
const FONTI={};
/* Prezzi di armi e armature (nome → costo) e categorie in più dell'equipaggiamento acquistabile ({id, n, voci:[[nome, costo, nota]]}):
   vengono dai pacchetti di contenuti; li usa web/regole/equipaggiamento.js. */
const PREZZI={};
const EQUIP_CAT=[];
const ALIGN=['Legale Buono','Neutrale Buono','Caotico Buono','Legale Neutrale','Neutrale','Caotico Neutrale','Legale Malvagio','Neutrale Malvagio','Caotico Malvagio'];
/* ===== INCANTESIMI: trucchetti e 1° livello =====
 formato: [nome IT, nome EN, livello, scuola, tempo, gittata, componenti, durata, classi, fonte, descrizione, livelli superiori]
 scuole: A abiurazione, C evocazione, D divinazione, E ammaliamento, V invocazione, I illusione, N necromanzia, T trasmutazione
 classi: a artefice, b bardo, c chierico, d druido, p paladino, r ranger, s stregone, w warlock, m mago
 tempo: a azione, ab azione bonus, re reazione, *=rituale | durata: I istantanea, C… concentrazione */
const SPELLS=[];
/* ===== SPIEGAZIONI DI PRIVILEGI, TRATTI E RISORSE (sintesi in italiano) ===== */
const FD_GEN={};
const FD_CLS={};

/* sottoclassi: spiegazioni nell'ordine dei privilegi ('' = spiegazione generica) */
const FD_SUB={};

/* razza */
const FD_RACE={};
/* privilegi di background */
const FD_BG={};
/* risorse */
const FD_RES={};
function featDesc(cls,sub,name,idx){const norm=n=>n.replace(/\s*\(.*?\)\s*/g,' ').replace(/✦/g,'').trim();const n=norm(name);
 if(sub&&FD_SUB[sub]&&idx!=null&&FD_SUB[sub][idx])return FD_SUB[sub][idx];
 if(cls&&FD_CLS[cls+'|'+n])return FD_CLS[cls+'|'+n];
 if(FD_GEN[n])return FD_GEN[n];if(FD_RACE[n])return FD_RACE[n];
 if(/^Stile: /.test(name)){const s=[...STYLES,...STYLES_T].find(x=>x[0]===name.slice(7));return s?s[1]+'.':'';}
 if(/^Incanalare Divinità/.test(n))return 'Un uso di Incanalare Divinità che produce l\'effetto indicato. Gli usi si recuperano con un riposo breve o lungo.';
 if(/^Aumento dei Punteggi/.test(n))return FD_GEN['Aumento dei Punteggi di Caratteristica'];
 return '';}
