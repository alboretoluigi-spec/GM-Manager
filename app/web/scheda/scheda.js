/* ===== LOGICA SCHEDA ===== */
const $=id=>document.getElementById(id);
const sg=n=>(n>=0?'+':'')+n;
const esc=s=>String(s==null?'':s).replace(/[&<>"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]));
const KEY='dnd5e-scheda-digitale-v1';
const parseF=s=>{const o=[];(s||'').split(';').forEach(p=>{if(!p)return;const i=p.indexOf(':');const l=+p.slice(0,i);p.slice(i+1).split('|').forEach(n=>o.push([l,n]));});return o;};
const fmtM=m=>(Math.round(m*10)/10).toString().replace('.',',')+' m';

function blank(){return{v:1,f:{},cls:'',sub:'',lvl:1,race:'',sr:'',bg:'',al:'',
 base:{FOR:10,DES:10,COS:10,INT:10,SAG:10,CAR:10},rA:[],anc:'',sAnc:'',lin:'dv',rFeat:'',rFeatA:'',
 sk:{},ex:{},asi:{},hp:{},hpCur:'',hpTmp:'',style:'',style2:'',armor:'',shield:false,
 wp:[{w:'',a:'auto',m:0},{w:'',a:'auto',m:0},{w:'',a:'auto',m:0},{w:'',a:'auto',m:0}],
 sp:{},spp:{},slotU:{},ds:{},insp:false,exh:0,tOrig:false,tOpt:false,eq:'',eqAuto:'',coins:{MR:'',MA:'',ME:'',MO:'',MP:''},tl:{},img:'',sym:'',spl:{},mc:[],terra:'',san:{v:'',metodo:'DMG_STANDARD'},mag:[],inv:[]};}
let st=blank();
let EFF=null; /* Gaster Bless: effetti in corso nello scontro mandati dall'app ({ca, caOrig, voci}), per la CA e il riquadro «Difese» */
if(window.parent===window)try{const s=localStorage.getItem(KEY);if(s){st=Object.assign(blank(),JSON.parse(s));st.mag=OggettiMagici.sanificaVoci(st.mag);}}catch(e){}
let saveT=null;
function save(){clearTimeout(saveT);saveT=setTimeout(()=>{if(window.parent!==window)return;try{localStorage.setItem(KEY,JSON.stringify(st));}catch(e){}},250);}
function toast(t,ms){const e=$('toast');e.textContent=t;e.classList.add('on');clearTimeout(toast.t);toast.t=setTimeout(()=>e.classList.remove('on'),ms||1800);}
/* ===== Tiri dalla scheda (richiesta dell'utente, 2026-11-06) =====
 Clic su un tiro salvezza o su un'abilità: d20 + il bonus mostrato. Dentro l'app il tiro lo fa il server
 (dadi.tira) e il risultato lo vedono giocatore e Master; fuori dall'app è solo locale. I pallini della
 competenza e della maestria restano pulsanti a parte. */
const numBonus=t=>parseInt(String(t).replace(/[−–]/g,'-'),10);
function tiroScheda(et,mod,modo){const nome=(st.f&&st.f.nome||'').trim();const etichetta=((nome?nome.slice(0,14)+' · ':'')+et).slice(0,40);
 if(typeof HOST!=='undefined'&&HOST.on){HOST.send({type:'scheda:tiro',etichetta,mod,modo:modo||''});return}
 const d1=1+Math.floor(Math.random()*20),d2=1+Math.floor(Math.random()*20);const d=modo==='vantaggio'?Math.max(d1,d2):modo==='svantaggio'?Math.min(d1,d2):d1;toast(`🎲 ${et}: d20 (${d}) ${sg(mod)} = ${d+mod}${d===20?' · 20 naturale!':d===1?' · 1 naturale':''} (tiro locale)`,4500);}
document.addEventListener('click',e=>{const r=e.target.closest('[data-tiro]');if(!r||e.target.closest('button,input,select'))return;
 const [k,x]=r.dataset.tiro.split(':');let mod,et;
 if(k==='ts'){mod=numBonus($('sv_'+x).textContent);et='TS '+ABN[x];}
 else{const i=+x;mod=numBonus($('kv_'+i).textContent);et=SKILLS[i][0];}
 if(Number.isFinite(mod)){const Ex=magE();const av=k==='ts'?Ex.vantaggio('TS','TS_'+x):Ex.vantaggio('PROVE','ABIL:'+SKILLS[+x][0]);
  tiroScheda(et,mod,av==='vantaggio'||av==='svantaggio'?av:undefined);}});

/* ---------- accesso ai dati ---------- */
const C=()=>CLASSES[st.cls]||null;
function S(){const c=C();if(!c||!st.sub)return null;const s=c.subs.find(x=>x[0]===st.sub);if(!s)return null;return{n:s[0],src:s[1],f:s[2],x:s[3]||{}};}
const subOn=()=>{const c=C(),s=S();return !!(c&&s&&st.lvl>=c.subL);};
const SX=()=>subOn()?S().x:{};
function RD(){
 const r=RACES[st.race];if(!r)return null;const s=(r.subs&&r.subs[st.sr])||{};
 const o={n:s.n||r.n,src:r.src,asi:[...(r.asi||[]),...(s.asi||[])],flex:s.flex||r.flex,t:[...(r.t||[]),...(s.t||[])],
  weapN:[...(r.weapN||[]),...(s.weapN||[])],armorP:[...(r.armorP||[]),...(s.armorP||[])],skFix:[...(r.skFix||[])],skP:(r.skP||0)+(s.skP||0),
  speed:s.speed||r.speed||9,dv:Math.max(r.dv||0,s.dv||0),lang:r.lang||[],langX:(r.langX||0)+(s.langX||0),feat:r.feat||s.feat,
  hpLvl:s.hpLvl||0,tools:[r.tools,s.tools].filter(Boolean),size:r.size,noMove:s.noMove,dragon:r.dragon,drow:s.drow,tief:r.tief,dwarf:r.dwarf,lineage:r.lineage,hasSubs:!!r.subs,
  flexL:s.flexL||r.flexL,inn:[...(r.inn||[]),...(s.inn||[])],innC:s.innC||r.innC,innN:s.innN||r.innN,natAc:s.natAc||r.natAc};
 if(o.lineage){if(st.lin==='sk')o.skP+=1;else o.dv=18;}
 return o;}
const BG=()=>BGS[st.bg]||null;
const prof=()=>Regole5e.competenza(st);
/* Gaster Bless, multiclasse: livello del personaggio (LT), livello in una classe (lv), classi (KL). */
const LT=()=>Regole5e.livelloTotale(st);const lv=id=>Regole5e.livelloClasse(st,id);const KL=()=>Regole5e.classi(st);
const multi=()=>KL().length>1;const classiTxt=()=>Regole5e.descrizioneClassi(st);
function asiLabel(k){const m=/^([a-z]+):(\d+)$/.exec(String(k));return m?(CLASSES[m[1]]?CLASSES[m[1]].n:m[1])+' '+m[2]:(multi()&&C()?C().n+' ':'Liv. ')+k;}

/* ---------- caratteristiche ---------- */
function racialList(){
 const r=RD();if(!r)return[];const L=[];
 r.asi.forEach(([a,v])=>L.push({v,def:a,fixed:true}));
 if(r.flex)for(let i=0;i<r.flex.n;i++)L.push({v:r.flex.v,def:null,excl:r.flex.excl||[]});
 (r.flexL||[]).forEach(v=>L.push({v,def:null,excl:[]})); /* lignaggi di Corvonero: +2 e +1 a scelta */
 L.forEach((b,i)=>{const movable=!b.fixed||(st.tOrig&&!r.noMove&&b.v>0); /* i malus (orco, coboldo) restano fissi */b.movable=movable;b.ab=movable?(st.rA[i]||b.def||''):b.def;});
 return L;}
function asiLevels(){return Regole5e.aumenti(st).map(a=>a.chiave);}
function feats(){const F=[];asiLevels().forEach(l=>{const a=st.asi[l];if(a&&a.m==='f'&&a.feat)F.push({n:a.feat.trim(),a:a.fa,l});});
 const r=RD();if(r&&r.feat&&st.rFeat)F.push({n:st.rFeat.trim(),a:st.rFeatA,l:1,race:1});return F;}
const hasFeat=n=>feats().some(f=>f.n.toLowerCase()===n.toLowerCase());
/* effetti dei talenti (regole/talenti.js, portato dal ramo main-ke8sy4): ogni talento riconosciuto (nome
   italiano o inglese) con il suo effetto; i talenti non in elenco si gestiscono a mano come prima */
function featFx(k){return feats().map(f=>({f,i:featInfo(f.n)})).filter(x=>x.i&&x.i.fx[k]!=null);}
const featSum=k=>featFx(k).reduce((s,x)=>s+(+x.i.fx[k]||0),0);
/* il +1 del talento vale solo per i mezzi talenti, sulle caratteristiche ammesse (talento sconosciuto: come prima) */
function featAbOk(n,a){if(!a)return false;const i=featInfo(n);if(!i)return true;if(i.ab==null)return false;return i.ab==='*'||i.ab.includes(a);}
function abil(){
 const T={},D={};AB.forEach(a=>{T[a]=+st.base[a]||0;D[a]={base:T[a],race:0,asi:0,x:0};});
 racialList().forEach(b=>{if(b.ab){T[b.ab]+=b.v;D[b.ab].race+=b.v;}});
 const inc=(a,v)=>{if(!a||!T.hasOwnProperty(a))return;const nv=Math.max(T[a],Math.min(20,T[a]+v));D[a].asi+=nv-T[a];T[a]=nv;};
 asiLevels().forEach(l=>{const x=st.asi[l];if(!x)return;if(x.m==='2')inc(x.a,2);else if(x.m==='11'){inc(x.a,1);inc(x.b,1);}else if(x.m==='f'&&featAbOk(x.feat,x.fa))inc(x.fa,1);});
 const r=RD();if(r&&r.feat&&featAbOk(st.rFeat,st.rFeatA))inc(st.rFeatA,1);
 if(lv('barbaro')>=20){['FOR','COS'].forEach(a=>{const nv=Math.max(T[a],Math.min(24,T[a]+4));D[a].x+=nv-T[a];T[a]=nv;});}
 /* Oggetti magici (regole/oggetti.js): effetto calcolato al volo e mostrato a parte, MAI scritto in st.base. */
 {const pm=OggettiMagici.punteggi(mag());AB.forEach(a=>{const nv=OggettiMagici.applicaPunteggio(T[a],pm[a]);D[a].item=nv-T[a];D[a].itemDa=pm[a].da;T[a]=nv;});}
 const M={};AB.forEach(a=>M[a]=Math.floor((T[a]-10)/2));return{T,M,D};}

/* ---------- competenze ---------- */
function armorProfs(){const s=new Set(Regole5e.competenzeClassi(st).armorP);const r=RD();if(r)r.armorP.forEach(x=>s.add(x));featFx('armor').forEach(x=>x.i.fx.armor.forEach(a=>s.add(a)));return s;}
function weapProf(w){const c=C();if(!c)return false;const k=Regole5e.competenzeClassi(st);const cats=new Set(k.weapP);
 const names=new Set(k.weapN);const r=RD();if(r)r.weapN.forEach(n=>names.add(n));
 return cats.has(w[1])||names.has(w[0]);}
function weapProfClass(n){const c=C();const w=WEAPONS.find(x=>x[0]===n);if(!c||!w)return false;const k=Regole5e.competenzeClassi(st);return new Set(k.weapP).has(w[1])||k.weapN.includes(n);}
function saveProfs(){const c=C();const s=new Set(c?c.saves:[]);featFx('saveAb').forEach(x=>{if(x.f.a&&featAbOk(x.f.n,x.f.a))s.add(x.f.a);});if(lv('monaco')>=14)AB.forEach(a=>s.add(a));return s;}
function autoSkills(){const s=new Set();const b=BG();if(b)b.sk.forEach(x=>s.add(x));const r=RD();if(r)r.skFix.forEach(x=>s.add(x));(SX().skFix||[]).forEach(x=>s.add(x));return s;}
function skillPicks(){const c=C(),r=RD(),b=BG();const mcSk=KL().filter(k=>!k.primaria).reduce((a,k)=>a+((Regole5e.COMPETENZE_MC[k.id]||{}).skP||0),0);return{cls:(c?c.skN:0)+mcSk,race:r?r.skP:0,sub:SX().skP||0,bg:b&&b.skP||0};}
function expAllowed(){let n=0;const lL=lv('ladro'),lB=lv('bardo');if(lL)n+=lL>=6?4:2;if(lB)n+=lB>=10?4:lB>=3?2:0;n+=SX().exP||0;return n;}
function autoExp(){return new Set(SX().exFix||[]);}
function isProfSk(n){return autoSkills().has(n)||!!st.sk[n];}

/* ---------- incantesimi ---------- */
function caster(){if(!C())return null;const A=abil();const inc0=Regole5e.incantatore(st,A.M,prof());if(!inc0.classi.length)return null;
 /* oggetti magici: bonus a CD e attacco degli incantesimi (su una copia: i dati del motore non cambiano) */
 const Em=magE(),dC=Em.somma('CD_INC'),dA=Em.somma('ATT_INC');const inc=dC||dA?{...inc0,classi:inc0.classi.map(x=>({...x,cd:x.cd+dC,atk:x.atk+dA}))}:inc0;
 /* La prima classe incantatrice dà CD e attacco mostrati in alto; con il multiclasse ognuna ha i suoi (cs.classi). */
 const pr=inc.classi[0];const mc=inc.classi.length>1;
 return{type:pr.tipo,ab:pr.ab,m:pr.m,dc:pr.cd,atk:pr.atk,slots:inc.slots,pact:inc.patto,known:mc?null:pr.known,kLabel:pr.kLabel,
  cantrips:inc.classi.reduce((a,x)=>a+(x.trucchetti||0),0),active:inc.classi.some(x=>x.attivo),classi:inc.classi,mc,inc};}

/* ---------- difesa / movimento ---------- */
/* ---------- oggetti magici: accesso allo stato e agli effetti (il calcolo è in regole/oggetti.js) ---------- */
function mag(){return Array.isArray(st.mag)?st.mag:[];}
function armorMag(){return OggettiMagici.armatura(mag());}
function scudoMag(){return OggettiMagici.scudo(mag());}
function hasShield(){return !!(st.shield||scudoMag());}
/* un'armatura magica equipaggiata (e sintonizzata, se serve) sostituisce quella scelta nel menu; ha la forma di ARMORS + regola della Des */
function armorObj(){const m=armorMag();if(m)return m.entry;return ARMORS.find(a=>a[0]===st.armor)||null;}
function magE(){const A=abil(),ar=armorObj();return OggettiMagici.calcola(mag(),{M:A.M,T:A.T,comp:prof(),liv:LT(),senzaArmatura:!ar&&!hasShield()});}
/* Gaster Bless, difese dei personaggi (2026-11-12, regole/difese.js): resistenze, immunità e vantaggi sempre attivi da razza,
 classe, sottoclasse e talenti, più quelli degli oggetti magici. Vanno nel riepilogo per il motore degli scontri. */
function difeseCtx(){const r=RD();const d=r&&r.dragon?DRAGONS.find(x=>x[0]===st.anc):null;const ar=armorObj();
 return{razza:st.race,sottorazza:st.sr,draconico:d?d[1]:'',classi:KL().map(k=>({id:k.id,lvl:k.lvl,sub:k.subOn&&k.sub?k.sub.n:''})),
  talenti:feats().map(f=>f.n),scelte:{amb:st.amb||'',genio:st.genio||'',resRip:st.resRip||'',totem:st.totem||''},armPesante:!!(ar&&ar[1]==='p')};}
function difeseOggetti(){const E=magE();const l=a=>[...new Set((a||[]).map(x=>String(x.t||'').toLowerCase().trim()).filter(Boolean))];return{res:l(E.testi.resist),imm:l(E.testi.immun)};}
function difesePG(){return typeof DifesePG!=='undefined'?DifesePG.calcola(difeseCtx()):null;}
/* tiri salvezza come li mostra la scheda (competenza, Aura di protezione, oggetti magici) */
function tsTotali(){const A=abil(),M=A.M,p=prof(),E=magE(),sp=saveProfs();const aura=lv('paladino')>=6?Math.max(1,M.CAR):0;const o={};
 AB.forEach(a=>{o[a]=M[a]+(sp.has(a)?p:0)+aura+E.somma('TS','TS_'+a);});return o;}
function acCalc(){const A=abil(),M=A.M;const ar=armorObj();const opts=[];const E=magE();const sh=scudoMag();const shB=sh?sh.ca:(st.shield?2:0);
 if(ar){const dm=ar[5]||(ar[1]==='l'?'completo':ar[1]==='m'?'max2':'nessuno'); /* ar[5]: regola della Destrezza di un'armatura magica */
  const d=dm==='completo'?M.DES:dm==='max2'?Math.min(featFx('medDex3').length?3:2,M.DES):0;opts.push([ar[2]+d,ar[0]]);}
 else{opts.push([10+M.DES,'senza armatura']);
  if(lv('barbaro'))opts.push([10+M.DES+M.COS,'Difesa senza armatura']);
  if(lv('monaco')&&!hasShield())opts.push([10+M.DES+M.SAG,'Difesa senza armatura']);
  if(KL().some(k=>k.x.drac))opts.push([13+M.DES,'Resilienza draconica']);
  {const rr=RD();if(rr&&rr.natAc)opts.push([rr.natAc+M.DES,'Armatura naturale']);}
  featFx('unarmored').forEach(x=>opts.push([13+M.DES,x.i.it]));}
 E.cand.forEach(c=>opts.push([c.ca,c.da])); /* basi alternative di oggetti magici (sostituzione o formula) */
 opts.sort((a,b)=>b[0]-a[0]);let [ac,src]=opts[0];
 if(shB){ac+=shB;src+=' + '+(sh?sh.nome:'scudo');}
 if(ar&&(st.style==='Difesa'||st.style2==='Difesa')){ac+=1;src+=' + Difesa';}
 {const fl=E.somma('CA');if(fl){ac+=fl;src+=' + oggetti ('+E.fonti('CA').join(', ')+')';}ac=E.finale('CA',ac);}
 return{ac,src};}
function speedCalc(){const r=RD();const A=abil();let s=r?r.speed:9;const n=[];const ar=armorObj();
 const lM=lv('monaco');if(lM>=2&&!ar&&!hasShield()){const b=lM>=18?9:lM>=14?7.5:lM>=10?6:lM>=6?4.5:3;s+=b;n.push('mov. senza armatura');}
 if(lv('barbaro')>=5&&!(ar&&ar[1]==='p')){s+=3;n.push('mov. veloce');}
 featFx('speed').forEach(x=>{s+=+x.i.fx.speed;n.push(x.i.it);});
 if(ar&&ar[3]&&A.T.FOR<ar[3]&&!(r&&r.dwarf)){s-=3;n.push('For insufficiente');}
 {const E=magE(),vb=E.somma('VELOCITA');if(vb){s+=vb;n.push('oggetti');}s=E.finale('VELOCITA',s);}
 return{s,n:n.join(', ')};}
function initCalc(){const A=abil(),M=A.M,p=prof();let v=M.DES;const n=[];
 const kG=KL().find(k=>k.id==='guerriero');
 if(lv('bardo')>=2){v+=Math.floor(p/2);n.push('Factotum');}
 else if(kG&&kG.x.champ&&kG.lvl>=7){v+=Math.ceil(p/2);n.push('Atleta Straord.');}
 featFx('init').forEach(x=>{v+=+x.i.fx.init;n.push(x.i.it);});
 {const E=magE(),ib=E.somma('INIZIATIVA');if(ib){v+=ib;n.push('oggetti');}const av=E.vantaggio('INIZIATIVA');if(av==='vantaggio'||av==='svantaggio')n.push(av+' (oggetti)');}
 KL().forEach(k=>{const sx=k.x;if(sx.initCar&&k.lvl>=3){v+=M.CAR;n.push('Audacia');}
 if(sx.initInt&&k.lvl>=2){v+=M.INT;n.push('Intuito Tattico');}
 if(sx.initSag&&k.lvl>=3){v+=M.SAG;n.push('Imboscata');}});
 return{v,n:n.join(', ')};}

/* ---------- punti ferita ---------- */
function hpCalc(){const c=C();if(!c)return null;const A=abil();const con=A.M.COS;const hd=c.hd,avg=hd/2+1;const L=LT();
 const b=Regole5e.puntiFerita(st,con);let tot=b.max;const r=RD();let ex=[];if(r&&r.hpLvl){tot+=L;ex.push('Robustezza nanica');}
 const dr=KL().find(k=>k.x.drac);if(dr){tot+=dr.lvl;ex.push('Resilienza draconica');}
 featFx('hpLvl').forEach(x=>{tot+=x.i.fx.hpLvl*L;ex.push(x.i.it);});
 {const pf=magE().somma('PF_MAX');if(pf){tot+=pf;ex.push('oggetti');}}
 return{max:tot,hd,avg,meth:b.metodo,ex,dadi:b.dadi};}

/* ---------- risorse di classe ---------- */
function resources(){const R=[];const mc=multi();let att=0;
 KL().forEach(k=>risorseClasse(k).forEach(([a,b])=>{if(a==='Attacchi per azione'){att=Math.max(att,+b||0);return;}R.push([mc&&!FD_RES[a+' ('+k.c.n+')']&&R.some(x=>x[0]===a)?a+' ('+k.c.n+')':a,b]);}));
 if(att)R.push(['Attacchi per azione',att]); /* l'Attacco Extra di classi diverse non si somma */
 featFx('res').forEach(x=>R.push([x.i.fx.res[0],String(x.i.fx.res[1])]));
 return R;}
function risorseClasse(k){const c=k.c;const L=k.lvl,A=abil(),M=A.M,p=prof(),R=[];const sx=k.x,so=k.subOn;
 const add=(a,b)=>R.push([a,b]);const atk=n=>add('Attacchi per azione',n);
 const mn1=x=>Math.max(1,x);
 switch(k.id){
 case'artefice':if(L>=2)add('Infusioni conosciute / oggetti infusi',(L>=18?12:L>=14?10:L>=10?8:L>=6?6:4)+' / '+(L>=18?6:L>=14?5:L>=10?4:L>=6?3:2));
  if(L>=7)add('Lampo di Genio',mn1(M.INT)+' usi, +'+M.INT);if(so&&sx.ea&&L>=5)atk(2);break;
 case'barbaro':add('Ire',L>=20?'illimitate':[2,2,3,3,3,4,4,4,4,4,4,5,5,5,5,5,6,6,6][L-1]);add('Danni dell\'ira','+'+(L>=16?4:L>=9?3:2));
  if(L>=9)add('Critico brutale','+'+(L>=17?3:L>=13?2:1)+' dado/i');if(L>=5)atk(2);break;
 case'bardo':{const d=L>=15?'d12':L>=10?'d10':L>=5?'d8':'d6';add('Ispirazione bardica',d+' × '+mn1(M.CAR)+(L>=5?' (riposo breve)':' (riposo lungo)'));
  if(L>=2)add('Canto di riposo',L>=17?'d12':L>=13?'d10':L>=9?'d8':'d6');if(so&&sx.ea&&L>=6)atk(2);break;}
 case'chierico':if(L>=2)add('Incanalare Divinità',(L>=18?3:L>=6?2:1)+' / riposo breve');
  if(L>=5)add('Distruggere Non Morti','GS '+(L>=17?4:L>=14?3:L>=11?2:L>=8?1:'1/2'));if(L>=10)add('Intervento Divino',L>=20?'automatico':L+'% di successo');break;
 case'druido':if(L>=2){const moon=so&&sx.moon;const cr=moon?(L>=6?Math.floor(L/3):1):(L>=8?1:L>=4?'1/2':'1/4');
  add('Forma Selvatica',(L>=20?'illimitata':'2 / riposo breve')+' · GS max '+cr+(L<4?' (no nuoto/volo)':L<8?' (no volo)':''));}break;
 case'guerriero':add('Recuperare Energie','1d10+'+L);if(L>=2)add('Azione Impetuosa',L>=17?2:1);if(L>=9)add('Indomabile',L>=17?3:L>=13?2:1);
  atk(L>=20?4:L>=11?3:L>=5?2:1);
  if(so&&sx.bm)add('Dadi superiorità',(L>=15?6:L>=7?5:4)+'× '+(L>=18?'d12':L>=10?'d10':'d8')+' · manovre '+(L>=15?9:L>=10?7:L>=7?5:3)+' · CD '+(8+p+Math.max(M.FOR,M.DES)));
  if(so&&sx.psi)add('Dadi energia psionica',(2*p)+'× '+(L>=17?'d12':L>=11?'d10':L>=5?'d8':'d6'));
  if(so&&sx.rune)add('Rune conosciute',L>=15?5:L>=10?4:L>=7?3:2);break;
 case'ladro':add('Attacco Furtivo',Math.ceil(L/2)+'d6');if(so&&sx.psi)add('Dadi energia psionica',(2*p)+'× '+(L>=17?'d12':L>=11?'d10':L>=5?'d8':'d6'));break;
 case'mago':add('Recupero Arcano','livelli di slot: '+Math.ceil(L/2));add('Incantesimi nel libro (min.)',6+2*(L-1));if(so&&sx.ea&&L>=6)atk(2);break;
 case'monaco':add('Arti Marziali',L>=17?'1d10':L>=11?'1d8':L>=5?'1d6':'1d4');if(L>=2)add('Punti Ki',L+' · CD Ki '+(8+p+M.SAG));if(L>=5)atk(2);break;
 case'paladino':add('Imposizione delle Mani',5*L+' PF');add('Percezione del Divino',Math.max(1,1+M.CAR)+' / riposo lungo');
  if(L>=3)add('Incanalare Divinità','1 / riposo breve');if(L>=6)add('Aura (raggio)',L>=18?'9 m':'3 m');if(L>=5)atk(2);break;
 case'ranger':add('Nemici prescelti / terreni',(L>=14?3:L>=6?2:1)+' / '+(L>=10?3:L>=6?2:1));if(L>=5)atk(2);break;
 case'stregone':if(L>=2)add('Punti Stregoneria',L);if(L>=3)add('Opzioni di Metamagia',L>=17?4:L>=10?3:2);break;
 case'warlock':if(L>=2)add('Suppliche occulte',[0,2,2,3,3,4,4,5,5,5,6,6,6,7,7,7,8,8,8,8][L-1]);
  if(L>=11)add('Arcanum Mistico',[6,7,8,9].filter((x,i)=>L>=[11,13,15,17][i]).map(x=>x+'°').join(', '));break;}
 return R;}
/* ===== STRUMENTI E PROFESSIONI (Gaster Bless, 2026-11-06) =====
 Dati in /regole/strumenti.js. Un'operazione si può fare solo con la competenza nello strumento E lo strumento
 scritto nell'equipaggiamento. La guida di uno strumento la vedono il Master (sempre) e chi lo possiede. */
const TLS=()=>(typeof STRUMENTI!=='undefined'?STRUMENTI:[]);
/* strumenti che sono anche professioni (catalogo di Gaster Bless: raccolta e fabbricazione) */
let PROF_IDS=new Set();
fetch('/regole/professioni.json').then(r=>r.ok?r.json():null).then(j=>{if(!j)return;PROF_IDS=new Set([...Object.keys(j.professioni||{}),...(j.artigiani||[]).map(a=>a.id)]);if(typeof renderTools==='function')renderTools();}).catch(()=>{});
const normT=t=>String(t||'').toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g,'').replace(/\s+/g,' ');
function toolInTesto(T,txt){const t=normT(txt);return [T.n,...(T.alias||[])].some(a=>t.includes(normT(a)));}
/* competenze date da classe, razza e background quando il testo nomina uno strumento preciso (non "a scelta") */
function toolAuto(T){const c=C(),r=RD(),b=BG();
 const parti=[c&&c.tools,...(r?r.tools:[]),b&&b.tools].filter(Boolean).join(',').split(/[,;]/).map(x=>x.trim())
  .filter(x=>x&&!/^(un|uno|una|due|tre)\b/i.test(x)&&!/a scelta|\so\s/i.test(x));
 return parti.some(x=>toolInTesto(T,x));}
const toolComp=T=>toolAuto(T)||!!(st.tl&&st.tl[T.id]);
const toolHa=T=>toolInTesto(T,st.eq+'\n'+(Array.isArray(st.inv)?st.inv.map(x=>x&&x.n||'').join('\n'):'')); /* anche gli oggetti dell'elenco (2026-11-11) */
const skProf=n=>autoSkills().has(n)||!!st.sk[n];
function toolPronto(T){return toolComp(T)&&toolHa(T);}
function renderTools(){const box=$('tlBox');if(!box)return;
 const l=TLS().filter(T=>toolComp(T)||toolHa(T));
 box.innerHTML=(l.length?l.map(T=>{const c=toolComp(T),h=toolHa(T),ok=c&&h;
  return `<div class="tlr"><b>${esc(T.n)}</b><span class="tlc ${c?'on':''}" title="Competenza">${c?'✓':'✗'} competenza</span><span class="tlc ${h?'on':''}" title="Nell'equipaggiamento">${h?'✓':'✗'} con sé</span>
   <span class="tlb">${ok&&PROF_IDS.has(T.id)?`<button class="linkbtn" data-tlprof="${T.id}" title="Raccolta e fabbricazione">⚒ Professione</button>`:''}<button class="linkbtn" data-tlu="${T.id}"${ok?'':' disabled title="Servono la competenza e lo strumento nell\'equipaggiamento"'}>Usa…</button>${h||IS_MASTER?`<button class="linkbtn" data-tlg="${T.id}">ℹ Guida</button>`:''}</span></div>`;}).join('')
  :'<div class="hint">Nessuno strumento: scegli le competenze e scrivi gli strumenti nell\'equipaggiamento.</div>')
  +`<div class="tlb2"><button class="linkbtn" id="tlPick">Competenze negli strumenti…</button>${IS_MASTER?'<button class="linkbtn" id="tlAll">📖 Guida a tutti gli strumenti</button>':''}</div>`;}
function tlApri(html){$('tlBody').innerHTML=html;$('mTl').classList.add('on');}
function tlGuida(T){return `<div class="tlg"><p><b>Come si usa.</b> ${esc(T.uso)}</p><p><b>Cosa si può fare</b></p><ul>${T.puo.map(x=>`<li>${esc(x)}</li>`).join('')}</ul>
 <p><b>Cosa non si può fare</b></p><ul>${T.nonpuo.map(x=>`<li>${esc(x)}</li>`).join('')}</ul>
 <p class="hint">Costo ${esc(T.costo)}. Abilità collegate: ${esc((T.abil||[]).join(', ')||'—')}. ${esc(STRUMENTI_REGOLE)}</p></div>`;}
function tlBonus(car){const A=abil();return A.M[car]+prof();}
function tlUso(T){const A=abil();
 const ops=T.ops.map((o,i)=>{const v=o.ab&&skProf(o.ab);return `<tr><td>${esc(o.n)}${o.ab?`<br><small class="hint">vantaggio con competenza in ${esc(o.ab)}${v?' ✓':''}</small>`:''}</td>
  <td><select data-tlcar="${i}">${AB.map(a=>`<option value="${a}"${a===o.car?' selected':''}>${a}</option>`).join('')}</select></td>
  <td><input type="number" min="1" max="40" data-tlcd="${i}" value="${o.cd||''}" placeholder="Master" style="width:64px"></td>
  <td><select data-tlmodo="${i}"><option value="">normale</option><option value="vantaggio"${v?' selected':''}>vantaggio</option><option value="svantaggio">svantaggio</option></select></td>
  <td><button data-tlroll="${i}">🎲 ${sg(tlBonus(o.car))}</button></td></tr>`;}).join('');
 tlApri(`<h2>${esc(T.n)}</h2><div data-tlid="${T.id}">
  ${ops?`<h3 class="tlh">Prove</h3><table class="tlt"><tr><th>Operazione</th><th>Car.</th><th>CD</th><th>Tiro</th><th></th></tr>${ops}</table>
   <p class="hint">Il tiro lo fa l'app: d20 + caratteristica + competenza (+${prof()}). Il risultato lo vedono giocatore e Master.</p>`:''}
  ${PROF_IDS.has(T.id)?`<p class="hint">Raccolta e fabbricazione: pulsante <b>⚒ Professione</b>.</p>`:''}
  <details><summary>Guida: come si usa, cosa si può e cosa non si può fare</summary>${tlGuida(T)}</details></div>`);}
let tlUltimo=null; /* etichetta e CD dell'ultimo tiro, per dire se è riuscito */
$('mTl').addEventListener('click',e=>{const b=e.target.closest('button');if(!b)return;
 if(b.id==='tlClose'){$('mTl').classList.remove('on');return;}
 const w=b.closest('[data-tlid]');const T=w&&TLS().find(x=>x.id===w.dataset.tlid);
 if(b.dataset.tlroll!=null&&T){if(!toolPronto(T)){toast('Servono la competenza e lo strumento nell\'equipaggiamento');return;}
  const i=+b.dataset.tlroll,o=T.ops[i],car=w.querySelector(`[data-tlcar="${i}"]`).value,cd=parseInt(w.querySelector(`[data-tlcd="${i}"]`).value,10)||0,modo=w.querySelector(`[data-tlmodo="${i}"]`).value;
  tlUltimo={op:o.n,cd};tiroScheda(o.n+(cd?' CD'+cd:''),tlBonus(car),modo);return;}
});
$('mTl').addEventListener('change',e=>{const t=e.target;if(t.dataset.tlcar==null)return;const b=t.closest('tr').querySelector('[data-tlroll]');b.textContent='🎲 '+sg(tlBonus(t.value));});
$('tlBox').addEventListener('click',e=>{const b=e.target.closest('button');if(!b)return;
 if(b.dataset.tlprof){if(typeof HOST!=='undefined'&&HOST.on)HOST.send({type:'scheda:professione',id:b.dataset.tlprof});else toast('La funzione Professione si apre dall\'app di Gaster Bless.');return;}
 if(b.dataset.tlu){const T=TLS().find(x=>x.id===b.dataset.tlu);if(T&&toolPronto(T))tlUso(T);return;}
 if(b.dataset.tlg){const T=TLS().find(x=>x.id===b.dataset.tlg);if(T&&(toolHa(T)||IS_MASTER))tlApri(`<h2>${esc(T.n)}</h2>${tlGuida(T)}`);return;}
 if(b.id==='tlAll'&&IS_MASTER){tlApri('<h2>Guida agli strumenti</h2>'+TLS().map(T=>`<details><summary><b>${esc(T.n)}</b></summary>${tlGuida(T)}</details>`).join(''));return;}
 if(b.id==='tlPick'){tlApri('<h2>Competenze negli strumenti</h2><p>Spunta gli strumenti in cui il personaggio ha competenza (quelli dati da classe, razza o background sono già spuntati). Per usarli scrivi lo strumento nell\'equipaggiamento.</p>'
  +TLS().map(T=>{const a=toolAuto(T);return `<label class="tlpk"><input type="checkbox" data-tlpk="${T.id}"${a||st.tl&&st.tl[T.id]?' checked':''}${a||window.SCHEDA_SOLA_LETTURA?' disabled':''}> ${esc(T.n)}${a?' <small class="hint">(da classe, razza o background)</small>':''}</label>`;}).join(''));}});
$('mTl').addEventListener('change',e=>{const t=e.target;if(!t.dataset.tlpk)return;st.tl=Object.assign({},st.tl||{});if(t.checked)st.tl[t.dataset.tlpk]=true;else delete st.tl[t.dataset.tlpk];save();render();});
function raceExtras(){const r=RD();if(!r)return[];const A=abil(),M=A.M,p=prof(),L=LT(),R=[];
 if(r.dragon){const d=DRAGONS.find(x=>x[0]===st.anc);const dice=(L>=16?5:L>=11?4:L>=6?3:2)+'d6';
  R.push(['Arma a Soffio',d?dice+' '+d[1]+', '+d[2]+', TS '+ABS[d[3]]+' CD '+(8+p+M.COS):'scegli la discendenza']);if(d)R.push(['Resistenza','danni da '+d[1]]);}
 if(r.tief)R.push(['Retaggio Infernale','taumaturgia'+(L>=3?'; intimorire infernale (2° liv.) 1/riposo lungo':'')+(L>=5?'; oscurità 1/riposo lungo':'')+' · Car']);
 if(r.drow)R.push(['Magia Drow','luci danzanti'+(L>=3?'; luminescenza 1/riposo lungo':'')+(L>=5?'; oscurità 1/riposo lungo':'')+' · Car']);
 if(r.inn&&r.inn.length){const x=r.inn.filter(i=>L>=i[0]).map(i=>i[1]);if(x.length)R.push([r.innN||'Magia innata',x.join('; ')+(r.innC?' · '+r.innC:'')]);}
 if(r.dv)R.push(['Scurovisione',r.dv+' m']);
 return R;}

/* ---------- costruzione statica ---------- */
function fillSel(el,opts,val,ph){el.innerHTML=(ph?'<option value="">'+ph+'</option>':'')+opts.map(o=>Array.isArray(o)?'<option value="'+esc(o[0])+'">'+esc(o[1])+'</option>':'<option>'+esc(o)+'</option>').join('');el.value=val||'';}
function initStatic(){
 fillSel($('sCls'),Object.entries(CLASSES).map(([k,c])=>[k,c.n+(c.src==='TCE'?' (supplemento)':'')]),st.cls,'— Classe —');
 fillSel($('sLvl'),Array.from({length:20},(_,i)=>[i+1,'Liv. '+(i+1)]),String(st.lvl));
 const SRC_RAZZA={TCE:' (supplemento)',VGM:' (Volo)',EEPC:' (Elemental Evil)',MTF:' (Mordenkainen)',VRGR:' (Corvonero)'};
 fillSel($('sRace'),Object.entries(RACES).map(([k,r])=>[k,r.n+(SRC_RAZZA[r.src]||'')]),st.race,'— Razza —');
 fillSel($('sBg'),Object.entries(BGS).map(([k,b])=>[k,b.n]),st.bg,'— Background —');
 fillSel($('sAl'),ALIGN,st.al,'—');
 $('abCol').innerHTML=AB.map(a=>`<div class="ab"><div class="an">${ABN[a].toUpperCase()}</div><div class="am" id="m_${a}">+0</div><span class="at" id="t_${a}">10</span><div class="ab-base">base <input type="number" min="1" max="30" data-base="${a}" value="${st.base[a]}"></div><div class="ab-bd" id="b_${a}"></div></div>`).join('');
 $('savesRows').innerHTML=AB.map(a=>`<div class="r" data-tiro="ts:${a}" title="Clic per tirare il tiro salvezza"><span class="dot" id="sd_${a}"></span><span class="v" id="sv_${a}">+0</span><span class="n">${ABN[a]}</span></div>`).join('')+'<div class="r note" id="saveNote"></div>';
 $('skRows').innerHTML=SKILLS.map(([n,a],i)=>`<div class="r" data-tiro="ab:${i}" title="Clic per tirare la prova"><button class="dot" id="kd_${i}" data-sk="${i}" title="Competenza"></button><button class="dex hid" id="ke_${i}" data-ex="${i}" title="Maestria"></button><span class="v" id="kv_${i}">+0</span><span class="n">${n} <small>(${ABS[a]})</small></span></div>`).join('');
 $('coins').innerHTML=['MR','MA','ME','MO','MP'].map(c=>`<label><span>${c}</span><input data-coin="${c}" value="${esc(st.coins[c])}" inputmode="numeric"></label>`).join('');
 $('atkBody').innerHTML='';
 document.querySelectorAll('[data-f]').forEach(el=>{el.value=st.f[el.dataset.f]||'';});
 $('cInsp').checked=!!st.insp;showExh();$('iHpCur').value=st.hpCur||'';$('iHpTmp').value=st.hpTmp||'';$('tEq').value=st.eq||'';
 document.querySelectorAll('[data-ds]').forEach(el=>el.checked=!!st.ds[el.dataset.ds]);
 setImg('portrait',st.img);setImg('symbol',st.sym);
}
function setImg(id,src){const el=$(id);const old=el.querySelector('img');if(old)old.remove();const sp=el.querySelector('span');if(src){const i=document.createElement('img');i.src=src;el.prepend(i);sp.style.display='none';}else sp.style.display='';}

/* ---------- selettori dinamici intestazione ---------- */
function refreshHeaderSel(){
 const c=C();const sub=$('sSub');
 if(c){fillSel(sub,c.subs.map(s=>[s[0],s[0]+' ('+s[1]+')']),st.sub,'— '+c.subT+' —');sub.disabled=false;$('lSub').textContent=c.subT+(st.lvl<c.subL?' (dal '+c.subL+'° liv.)':'');}
 else{fillSel(sub,[],'','—');sub.disabled=true;$('lSub').textContent='Sottoclasse';}
 const r=RACES[st.race];const sr=$('sSr');
 if(r&&r.subs){fillSel(sr,Object.entries(r.subs).map(([k,s])=>[k,s.n]),st.sr,'— Sottorazza —');sr.style.display='';}
 else{sr.innerHTML='';sr.style.display='none';}
}

/* ---------- pannello scelte ---------- */
function abOpts(sel,excl){return '<option value="">—</option>'+AB.filter(a=>!(excl||[]).includes(a)).map(a=>`<option value="${a}"${a===sel?' selected':''}>${ABN[a]}</option>`).join('');}
function buildCfg(){
 const c=C(),r=RD(),b=BG();const H=[];
 /* razza */
 let h='<div class="cfgc"><h3>Razza'+(r?' · '+esc(r.n):'')+'</h3>';
 if(!r)h+='<div class="hint">Seleziona una razza dall\'intestazione.</div>';
 else{
  if(RACES[st.race].subs&&!st.sr)h+='<div class="warn">Seleziona la sottorazza.</div>';
  const L=racialList();
  if(L.length){h+='<div class="hint">Bonus di caratteristica razziali'+(st.tOrig&&!r.noMove?' (riassegnabili – supplemento)':'')+':</div>';
   L.forEach((x,i)=>{h+=`<div class="fr"><label>${x.v>0?'+':''}${x.v}</label>`+(x.movable?`<select data-ra="${i}">${abOpts(x.ab,x.excl)}</select>`:`<b>${ABN[x.ab]}</b>`)+'</div>';});
   const used=L.map(x=>x.ab).filter(Boolean);if(new Set(used).size<used.length&&!r.noMove)h+='<div class="warn">Ogni bonus razziale deve andare a una caratteristica diversa.</div>';}
  if(r.dragon)h+=`<div class="fr"><label>Discendenza</label><select id="cAnc"><option value="">—</option>${DRAGONS.map(d=>`<option${d[0]===st.anc?' selected':''}>${d[0]}</option>`).join('')}</select></div>`;
  if(r.lineage)h+=`<div class="fr"><label>Retaggio variabile</label><select id="cLin"><option value="dv"${st.lin==='dv'?' selected':''}>Scurovisione 18 m</option><option value="sk"${st.lin==='sk'?' selected':''}>Competenza in un'abilità</option></select></div>`;
  if(r.feat)h+=`<div class="fr"><label>Talento</label><input type="text" id="cRFeat" list="featList" value="${esc(st.rFeat)}" placeholder="nome del talento" style="flex:1"></div><div class="fr"><label>+1 del talento</label><select id="cRFeatA">${abOpts(st.rFeatA)}</select><span class="hint">(solo mezzi talenti)</span></div>`;
 }
 h+=`<label class="tog"><input type="checkbox" id="cTOrig"${st.tOrig?' checked':''}> supplemento: personalizzare l'origine (riassegna i bonus razziali)</label></div>`;
 H.push(h);
 /* classe */
 h='<div class="cfgc"><h3>Classe'+(c?' · '+c.n+' '+st.lvl:'')+'</h3>';
 if(!c)h+='<div class="hint">Seleziona classe e livello dall\'intestazione.</div>';
 else{
  const nStyle=(c.style===1||(c.style===2&&st.lvl>=2))?1:0;const st2=SX().style;const champ2=SX().champ&&st.lvl>=10;
  const styl=[...STYLES,...(st.tOpt?STYLES_T:[])];
  if(nStyle||st2)h+=`<div class="fr"><label>Stile di combatt.</label><select id="cStyle"><option value="">—</option>${styl.map(s=>`<option${s[0]===st.style?' selected':''}>${s[0]}</option>`).join('')}</select></div>`;
  if(champ2)h+=`<div class="fr"><label>Secondo stile</label><select id="cStyle2"><option value="">—</option>${styl.map(s=>`<option${s[0]===st.style2?' selected':''}>${s[0]}</option>`).join('')}</select></div>`;
  if(subOn()&&SX().drac)h+=`<div class="fr"><label>Antenato drago</label><select id="cSAnc"><option value="">—</option>${DRAGONS.map(d=>`<option${d[0]===st.sAnc?' selected':''}>${d[0]} (${d[1]})</option>`).join('')}</select></div>`;
  if(st.lvl>=c.subL&&!st.sub)h+=`<div class="warn">Scegli ${c.subT.toLowerCase()} nell'intestazione.</div>`;
  if(typeof DifesePG!=='undefined')DifesePG.scelte(difeseCtx().classi).forEach(x=>{h+=`<div class="fr"><label>${esc(x.etichetta)}</label><select data-difs="${x.k}"><option value="">—</option>${x.opzioni.map(o=>`<option value="${o[0]}"${o[0]===st[x.k]?' selected':''}>${esc(o[1])}</option>`).join('')}</select></div>`;});
  const al=asiLevels();
  if(al.length){h+='<div class="hint" style="margin-top:8px"><b>Aumenti di caratteristica / talenti</b></div>';
   al.forEach(l=>{const x=st.asi[l]||{m:'2'};
    h+=`<div class="fr"><label>${esc(asiLabel(l))}</label><select data-asim="${l}"><option value="2"${x.m==='2'?' selected':''}>+2 a una</option><option value="11"${x.m==='11'?' selected':''}>+1 a due</option><option value="f"${x.m==='f'?' selected':''}>Talento</option></select>`;
    if(x.m==='2')h+=`<select data-asia="${l}">${abOpts(x.a)}</select>`;
    else if(x.m==='11')h+=`<select data-asia="${l}">${abOpts(x.a)}</select><select data-asib="${l}">${abOpts(x.b)}</select>`;
    else h+=`<input type="text" list="featList" data-asif="${l}" value="${esc(x.feat||'')}" placeholder="talento" style="width:120px"><select data-asifa="${l}" title="+1 dei mezzi talenti">${abOpts(x.fa)}</select>`;
    h+='</div>';});}
  h+=`<div class="fr"><label>Punti ferita</label><button class="btn" id="cHp">Media o valori manuali…</button></div>`;
 }
 if(c)h+=pannelloMulticlasse();
 h+=`<label class="tog"><input type="checkbox" id="cTOpt"${st.tOpt?' checked':''}> supplemento: privilegi di classe opzionali e nuovi stili</label></div>`;
 H.push(h);
 /* equip & caratteristiche */
 const A=abil();
 h='<div class="cfgc"><h3>Caratteristiche &amp; equipaggiamento</h3><table class="bd"><tr><th></th><th>BASE</th><th>RAZZA</th><th>ASI/TAL.</th><th>OGGETTI</th><th>TOT.</th></tr>'+
  AB.map(a=>`<tr><td>${ABN[a]}</td><td>${A.D[a].base}</td><td>${A.D[a].race?'+'+A.D[a].race:'—'}</td><td>${A.D[a].asi+A.D[a].x?'+'+(A.D[a].asi+A.D[a].x):'—'}</td><td>${A.D[a].item?sg(A.D[a].item):'—'}</td><td class="t">${A.T[a]}</td></tr>`).join('')+'</table>';
 const pb=pointBuy();h+=`<div class="hint">${pb}</div><div class="fr"><button class="btn pri" id="cPb">Calcolatore acquisto a punti…</button></div>`;
 const ap=armorProfs();
 h+=`<div class="fr"><label>Armatura</label><select id="cArm"><option value="">Nessuna</option>${['l','m','p'].map(t=>`<optgroup label="${{l:'Leggere',m:'Medie',p:'Pesanti'}[t]}">`+ARMORS.filter(a=>a[1]===t).map(a=>`<option${a[0]===st.armor?' selected':''}>${a[0]}</option>`).join('')+'</optgroup>').join('')}</select></div>`;
 h+=`<label class="tog"><input type="checkbox" id="cShield"${st.shield?' checked':''}> Scudo (+2 CA)</label>`;
 const ar=armorObj();if(ar&&!ap.has(ar[1]))h+='<div class="warn">Non hai competenza in questa armatura: svantaggio a prove, TS e attacchi con For/Des e niente incantesimi.</div>';
 if(armorMag())h+='<div class="hint">Indossi un\'armatura magica ('+esc(armorMag().nome)+'): sostituisce quella scelta qui sopra.</div>';
 if(st.shield&&!ap.has('sc'))h+='<div class="warn">Non hai competenza negli scudi.</div>';
 if(ar&&ar[4])h+='<div class="hint">Svantaggio alle prove di Furtività.</div>';
 h+='</div>';H.push(h);
 $('cfgBody').innerHTML=H.join('');
 $('cfgSum').textContent=[c?classiTxt():'',r?r.n:'',b?b.n:''].filter(Boolean).join(' · ');
}
function pointBuy(){const cost={8:0,9:1,10:2,11:3,12:4,13:5,14:7,15:9};let t=0,ok=true;AB.forEach(a=>{const v=+st.base[a];if(cost[v]==null)ok=false;else t+=cost[v];});
 return ok?`Acquisto a punti: <b>${t}</b>/27 punti spesi`:'Valori base fuori dall\'acquisto a punti (tiri di dado o serie standard: 15, 14, 13, 12, 10, 8)';}

/* ---------- render principale ---------- */
// Indebolimento: effetti cumulativi dei livelli (sintesi con parole nostre, regole 2014).
const EXH=['Nessun indebolimento.','Svantaggio alle prove di caratteristica.','Velocità dimezzata (più il livello 1).','Svantaggio a tiri per colpire e tiri salvezza (più i livelli precedenti).','Punti ferita massimi dimezzati (più i livelli precedenti).','Velocità 0 (più i livelli precedenti).','Morte.'];
function showExh(){const n=Math.max(0,Math.min(6,+st.exh||0));$('sExh').value=String(n);$('oExhD').textContent=EXH[n];$('sExh').closest('.exhbox').classList.toggle('attivo',n>0);}
function render(){
 const c=C(),r=RD(),b=BG(),A=abil(),M=A.M,p=prof(),L=LT(),sx=SX();const E=magE();
 /* icone delle classi (web/img/classi, fornite dall'utente): principale e classi del multiclasse */
 {const ic=$('oClsIco');if(ic)ic.innerHTML=KL().map(k=>`<img src="/img/classi/${k.id}_t.svg" alt="" title="${esc(k.c.n)} ${k.lvl}">`).join('');}
 AB.forEach(a=>{$('m_'+a).textContent=sg(M[a]);$('t_'+a).textContent=A.T[a];const d=A.D[a];const ex=d.race+d.asi+d.x+(d.item||0);$('b_'+a).textContent=ex?'bonus '+sg(ex):'';});
 $('oProf').textContent=sg(p);
 /* tiri salvezza */
 const sp=saveProfs();let aura=0;if(lv('paladino')>=6)aura=Math.max(1,M.CAR);
 AB.forEach(a=>{const on=sp.has(a);$('sd_'+a).className='dot auto'+(on?' on':'');const av=E.vantaggio('TS','TS_'+a);$('sv_'+a).textContent=sg(M[a]+(on?p:0)+aura+E.somma('TS','TS_'+a));$('sv_'+a).title=av==='vantaggio'||av==='svantaggio'?'Tiro con '+av+' ('+E.fontiVantaggio('TS','TS_'+a).join(', ')+')':'';});
 {const parti=[];if(aura)parti.push('Include Aura di Protezione (+'+aura+')');if(AB.some(a=>E.somma('TS','TS_'+a)))parti.push('include oggetti magici');
  AB.forEach(a=>{const av=E.vantaggio('TS','TS_'+a);if(av==='vantaggio'||av==='svantaggio')parti.push(av+' al TS di '+ABN[a]+' ('+E.fontiVantaggio('TS','TS_'+a).join(', ')+')');});
  $('saveNote').textContent=parti.join(' · ');}
 /* difese (2026-11-12) */
 {const box=$('oDif'),D=difesePG();if(box&&D){const R=DifesePG.righe(D,difeseOggetti(),DifesePG.ira?DifesePG.ira(difeseCtx()):null);if(EFF&&EFF.voci&&EFF.voci.length)R.unshift(['✨ Effetti in corso nello scontro',EFF.voci.map(String)]);
  box.innerHTML=R.length?R.map(([t,l])=>`<div class="dif-g"><b>${esc(t)}</b>${l.map(x=>`<div>${esc(x)}</div>`).join('')}</div>`).join(''):'<div class="hint">Nessuna resistenza o immunità.</div>';}}
 /* abilità */
 const au=autoSkills(),aex=autoExp();E.testi.comp.forEach(x=>{if(SKILLS.some(k=>k[0]===x.t))au.add(x.t);}); /* competenza data da un oggetto magico */const joat=lv('bardo')>=2;const kG=KL().find(k=>k.id==='guerriero');const ra=!!(kG&&kG.x.champ&&kG.lvl>=7);
 const exN=expAllowed();let manual=0,exUsed=0;
 SKILLS.forEach(([n,a],i)=>{const isA=au.has(n);const on=isA||!!st.sk[n];if(!isA&&st.sk[n])manual++;
  const ex=on&&(aex.has(n)||(!!st.ex[n]&&exN>0));if(on&&st.ex[n]&&!aex.has(n)&&exN>0)exUsed++;
  let v=M[a];if(on)v+=ex?2*p:p;else if(joat)v+=Math.floor(p/2);else if(ra&&['FOR','DES','COS'].includes(a))v+=Math.ceil(p/2);
  v+=E.somma('PROVE','ABIL:'+n);
  if(n==='Percezione')$('oPass').textContent=10+v+featSum('pass')+E.somma('PERC_PASSIVA');
  const d=$('kd_'+i);d.className='dot'+(isA?' auto on':on?' on click':' click')+(!on&&(joat||(ra&&['FOR','DES','COS'].includes(a)))?' half':'');
  d.title=isA?'Competenza automatica (razza/background/sottoclasse)':'Clicca per scegliere la competenza';
  const e=$('ke_'+i);e.className='dex'+(ex?' on':'')+((exN>0&&on)||aex.has(n)?' click':' hid');
  $('kv_'+i).textContent=sg(v);{const av=E.vantaggio('PROVE','ABIL:'+n);$('kv_'+i).title=av==='vantaggio'||av==='svantaggio'?'Prova con '+av+' ('+E.fontiVantaggio('PROVE','ABIL:'+n).join(', ')+')':'';}});
 const pk=skillPicks();const tot=pk.cls+pk.race+pk.sub+pk.bg;
 let cn='';if(c||r){const parts=[];if(pk.cls)parts.push(pk.cls+' classe');if(pk.race)parts.push(pk.race+' razza');if(pk.sub)parts.push(pk.sub+' sottoclasse');if(pk.bg)parts.push(pk.bg+' background');
  cn=`Competenze scelte <b class="${manual>tot?'bad':''}">${manual}/${tot}</b> (${parts.join(', ')||'—'})`;if(exN)cn+=` · Maestrie <b class="${exUsed>exN?'bad':''}">${exUsed}/${exN}</b>`;
  if(c&&c.sk!==ALL_SK)cn+=`<br>Abilità di classe: ${c.sk.join(', ')}`;else if(c)cn+='<br>Abilità di classe: tutte';}
 $('skCnt').innerHTML=cn;
 /* combattimento */
 const ac=acCalc();$('oAC').textContent=ac.ac;$('oACs').textContent=ac.src;
 /* Gaster Bless (2026-11-12): CA cambiata da un effetto in corso nello scontro, in un altro colore con l'originale visibile; la CA della scheda non cambia */
 $('oAC').classList.toggle('eff-mod',!!(EFF&&EFF.caOrig!=null&&EFF.ca!=null));
 if(EFF&&EFF.caOrig!=null&&EFF.ca!=null){$('oAC').textContent=EFF.ca;$('oAC').title='CA con gli effetti in corso; senza: '+ac.ac;$('oACs').textContent='senza effetti '+ac.ac+' · '+ac.src;}else $('oAC').title='';
 const ini=initCalc();$('oInit').textContent=sg(ini.v);$('oInits').textContent=ini.n;
 const spd=speedCalc();$('oSpd').textContent=fmtM(spd.s);$('oSpds').textContent=spd.n;
 const hp=hpCalc();
 if(hp){$('oHpMax').textContent=hp.max;$('oHpM').textContent=hp.meth+(hp.ex.length?' + '+hp.ex.join(', '):'');$('oHD').textContent=hp.dadi||L+'d'+hp.hd;$('iHpCur').placeholder=hp.max;}
 else{$('oHpMax').textContent='—';$('oHpM').textContent='—';$('oHD').textContent='—';}
 renderAttacks(A,p);
 /* competenze e lingue */
 const ap=armorProfs();const pr=[];
 if(c){const at={l:'leggere',m:'medie',p:'pesanti',sc:'scudi'};const arm=['l','m','p','sc'].filter(x=>ap.has(x));pr.push(['Armature',arm.length===4?'tutte le armature, scudi':arm.length?arm.map(x=>at[x]).join(', '):'nessuna']);
  let w=c.weapT;const extra=[...(sx.weapP||[]).includes('g')&&!c.weapP.includes('g')?['armi da guerra']:[],...(sx.weapN||[]).map(x=>x.toLowerCase())];if(r&&r.weapN.length)extra.push(...r.weapN.filter(n=>!weapProfClass(n)).map(x=>x.toLowerCase()));if(extra.length)w+=', '+[...new Set(extra)].join(', ');pr.push(['Armi',w]);}
 else if(r&&r.armorP.length)pr.push(['Armature',r.armorP.map(x=>({l:'leggere',m:'medie'}[x])).join(', ')]);
 const tools=[c&&c.tools,...(r?r.tools:[]),b&&b.tools].filter(Boolean);if(tools.length)pr.push(['Strumenti',tools.join('; ')]);
 const lang=[...(r?r.lang:[]),...(c&&c.lang||[])];const lx=(r?r.langX:0)+(b&&b.langX||0);
 if(lang.length||lx)pr.push(['Linguaggi',lang.join(', ')+(lx?(lang.length?', ':'')+'+'+lx+' a scelta':'')]);
 if(r)pr.push(['Taglia',r.size]);
 $('oProfs').innerHTML=pr.map(x=>`<p><b>${x[0]}</b><br>${esc(x[1])}</p>`).join('')||'<span class="hint" style="color:var(--mut)">Seleziona classe, razza e background.</span>';
 renderFeatures();
 renderTools();
 renderSpells(A,p);
 $('lXp').textContent='Punti esperienza'+(L<20?' · prossimo liv. a '+XP[L].toLocaleString('it-IT'):'');
 if(typeof renderMag==='function')renderMag(); /* pannello degli oggetti magici: web/scheda/oggetti_ui.js */
 save();
}
/* ===== ATTACCHI (Gaster Bless, 2026-11-11, Fase A del sistema di combattimento: docs/PIANO_COMBATTIMENTO.md) =====
 Ogni riga di st.wp è {w: arma del catalogo, a: caratteristica, m: bonus magico (storico, colpire e danni)} più campi
 facoltativi che, se presenti, sostituiscono i dati del catalogo: nome, mag (arma magica), cat (s/g), dist (m/d), prop,
 port (portata), g1/g2 (gittata), dado, tipo, comp (auto/si/no), bA/bD (bonus magico a colpire/ai danni), mA/mD (bonus
 aggiuntivi da talenti, capacità, effetti), extra [{dadi, tipo, mod}] (danni aggiuntivi di altro tipo), inv (oggetto
 dell'inventario collegato: la riga vale solo se è equipaggiato). Un campo assente = dato del catalogo: così cambiando
 l'arma i dati si aggiornano senza toccare le personalizzazioni. Un solo calcolo per tabella, anteprima e riepilogo. */
const ARMA_FL=pr=>{pr=(pr||'').toLowerCase();return(pr.includes('accurata')?'f':'')+(pr.includes('leggera')?'l':'')+(pr.includes('lancio')?'t':'')+(pr.includes('versatile')?'v':'')+(pr.includes('due mani')?'2':'')+(pr.includes('pesante')?'h':'')+(pr.includes('portata')?'r':'');};
const DADI_OK=d=>/^\d{1,2}d\d{1,3}$/.test(String(d||''))||d==='1'||d==='0';
const numOr=(v,d)=>v===''||v==null||!isFinite(+v)?d:Math.max(-30,Math.min(60,Math.round(+v)));
const gittataDa=pr=>{const m=String(pr||'').match(/(\d+(?:[.,]\d+)?)\s*\/\s*(\d+(?:[.,]\d+)?)\s*m/);return m?[m[1].replace(',','.'),m[2].replace(',','.')]:null;};
const TIPI_DANNO_IT=['acido','contundente','freddo','fuoco','forza','fulmine','necrotico','perforante','veleno','psichico','radioso','tagliente','tuono'];
/* dati effettivi di una riga: catalogo + personalizzazioni */
function wpEff(wp){wp=wp||{};const B=WEAPONS.find(x=>x[0]===wp.w)||null;
 const prop=wp.prop!=null?String(wp.prop):(B?B[5]:'');const dist=wp.dist==='d'||wp.dist==='m'?wp.dist:(B?B[2]:'m');
 const g=(wp.g1&&wp.g2)?[String(wp.g1),String(wp.g2)]:gittataDa(prop);
 const nome=String(wp.nome||wp.w||'').trim();const dado=DADI_OK(wp.dado)?String(wp.dado):(B?B[3]:'');
 const W=(B||(nome&&dado))?[B?B[0]:nome,wp.cat==='s'||wp.cat==='g'?wp.cat:(B?B[1]:'s'),dist,dado||'1',wp.tipo?String(wp.tipo):(B?B[4]:'contundente'),prop,wp.prop!=null||!B?ARMA_FL(prop):B[6]]:null;
 const leg=+wp.m||0;
 return{W,B,nome,mag:!!wp.mag||leg>0||+wp.bA>0||+wp.bD>0,bA:numOr(wp.bA,leg),bD:numOr(wp.bD,leg),mA:numOr(wp.mA,0),mD:numOr(wp.mD,0),
  comp:wp.comp==='si'||wp.comp==='no'?wp.comp:'auto',a:wp.a||'auto',arg:!!wp.arg,
  port:wp.port?String(wp.port):(dist==='m'?((W&&W[6].includes('r'))?'3 m':'1,5 m'):''),g,
  extra:(Array.isArray(wp.extra)?wp.extra:[]).filter(x=>x&&DADI_OK(x.dadi)&&x.dadi!=='0').slice(0,6).map(x=>({dadi:String(x.dadi),tipo:TIPI_DANNO_IT.includes(x.tipo)?x.tipo:'forza',mod:numOr(x.mod,0)}))};}
/* calcolatore legato allo stato attuale della scheda (caratteristiche, competenze, stili, oggetti magici) */
function calcolatoreAttacchi(){const A=abil(),M=A.M,p=prof();const lMo=lv('monaco');const monk=lMo>0;const md=lMo>=17?'1d10':lMo>=11?'1d8':lMo>=5?'1d6':'1d4';
 const arch=st.style==='Arcieria'||st.style2==='Arcieria';const duel=st.style==='Duellare'||st.style2==='Duellare';const thrownS=st.style==='Armi da Lancio';
 const EA=magE();const gA=EA.somma('ATTACCO'),gD=EA.somma('DANNO');
 /* o = {bA,bD,mA,mD,comp,extra,dado,tipo,sm,senzaComp,nome,portata,magica,argentata} */
 const calcRiga=(w,wp,o)=>{o=o||{};let a;const fl=w[6]||'';const monkW=monk&&(w[0]==='Spada corta'||(w[1]==='s'&&w[2]==='m'&&!/[h2]/.test(fl)));
  if(wp.a&&wp.a!=='auto'&&M[wp.a]!=null)a=wp.a;else if(w[2]==='d')a='DES';else if(fl.includes('f')||monkW)a=M.DES>M.FOR?'DES':'FOR';else a='FOR';
  const pf=o.senzaComp||o.comp==='si'?true:o.comp==='no'?false:weapProf(w);const legM=+wp.m||0;
  const magA=(o.bA||0)+legM,bonA=(o.mA||0)+gA+(arch&&w[2]==='d'?2:0);const ab2=M[a]+(pf?p:0)+magA+bonA;
  let die=o.dado||w[3];if(monkW&&!o.dado){const n=+md.slice(2),o2=+(w[3].split('d')[1]||1);if(n>o2||w[3]==='1')die=md;}
  const magD=(o.bD||0)+legM,bonD=(o.mD||0)+gD+(duel&&w[2]==='m'&&!fl.includes('2')?2:0)+(thrownS&&fl.includes('t')?2:0);
  const dm=M[a]+magD+bonD;const tipo=o.tipo!=null?o.tipo:w[4];const extra=o.extra||[];
  const ex=extra.map(x=>' + '+x.dadi+(x.mod?sg(x.mod):'')+' '+esc(x.tipo)).join('');
  const parti=[esc(die+' '+tipo),sg(M[a])+' '+ABS[a],magD?sg(magD)+' magico':'',bonD?sg(bonD)+' bonus':'',...extra.map(x=>esc(x.dadi+(x.mod?sg(x.mod):'')+' '+x.tipo)),o.argentata?'argentata':''].filter(Boolean).join(' · ');
  const partiAtt=[pf?sg(p)+' comp.':'',sg(M[a])+' '+ABS[a],magA?sg(magA)+' magico':'',bonA?sg(bonA)+' bonus':''].filter(Boolean).join(' · ');
  const profilo={nome:String(o.nome||w[0]),bonus:ab2,car:a,competente:pf,portata:o.portata||'',magica:!!o.magica||magA>0||magD>0,argentata:!!o.argentata,
   danni:[{dadi:String(die),mod:dm,tipo:String(tipo)},...extra.map(x=>({dadi:x.dadi,mod:x.mod,tipo:x.tipo}))]};
  return{atk:sg(ab2),dmg:esc(die)+(dm?(dm>0?'+':'')+dm:'')+' '+tipo+ex,sm:(o.sm!=null?o.sm:esc(w[5]))+(pf?'':' · <span class="warn">non competente</span>'),parti,partiAtt,profilo};};
 return{calcRiga,EA,gA,gD,M,p,monk,md};}
/* profilo di una riga di st.wp (anche non ancora salvata: anteprima della finestra di configurazione) */
function calcolaWp(wp,K){K=K||calcolatoreAttacchi();const E=wpEff(wp);if(!E.W)return null;
 const r=K.calcRiga(E.W,{a:E.a,m:0},{bA:E.bA,bD:E.bD,mA:E.mA,mD:E.mD,comp:E.comp,extra:E.extra,dado:E.W[3],tipo:esc(E.W[4]),nome:E.nome||E.W[0],magica:E.mag,argentata:E.arg,
  portata:E.W[2]==='d'?(E.g?E.g[0]+'/'+E.g[1]+' m':''):E.port+(E.g?' · lancio '+E.g[0]+'/'+E.g[1]+' m':''),
  sm:[esc(E.W[5]),E.W[2]==='m'?'portata '+esc(E.port):'',E.g?'gittata '+esc(E.g[0]+'/'+E.g[1])+' m':''].filter(Boolean).join(' · ')});
 r.profilo.danni[0].tipo=E.W[4];return r;}
const invEquip=id=>Array.isArray(st.inv)&&st.inv.some(x=>x&&x.id===id&&x.eq);
let PROFILI_ATTACCO=[];
function renderAttacks(A,p){
 const M=A.M;const opts='<option value="">—</option>'+[['s','m','Semplici da mischia'],['s','d','Semplici a distanza'],['g','m','Da guerra da mischia'],['g','d','Da guerra a distanza']].map(([c1,t,lab])=>`<optgroup label="${lab}">`+WEAPONS.filter(w=>w[1]===c1&&w[2]===t).map(w=>`<option>${w[0]}</option>`).join('')+'</optgroup>').join('');
 const abSel=v=>`<select data-wa style="width:46px">`+['auto',...AB].map(a=>`<option value="${a}"${a===v?' selected':''}>${a==='auto'?'auto':ABS[a]}</option>`).join('')+'</select>';
 let html='';const K=calcolatoreAttacchi();const{calcRiga,EA,gA,gD,monk,md}=K;const profili=[];const sx=SX();
 const arch=st.style==='Arcieria'||st.style2==='Arcieria';const duel=st.style==='Duellare'||st.style2==='Duellare'; /* per le note sotto la tabella */
 st.wp.forEach((wp,i)=>{if(!wp||typeof wp!=='object')return;if(wp.inv&&!invEquip(wp.inv))return; /* arma dell'inventario non equipaggiata */
  const E=wpEff(wp);const r=calcolaWp(wp,K);if(r)profili.push(r.profilo);
  const magTxt=E.bA===E.bD?(E.bA||''):E.bA+'/'+E.bD;const solo=E.bA!==E.bD||wp.bA!=null||wp.bD!=null;
  const titolo=wp.nome&&wp.nome!==wp.w?`<b class="wpn">${esc(wp.nome)}</b>${E.mag?' <span class="rar">magica</span>':''}`:(E.mag&&wp.w?'<span class="rar">magica</span>':'');
  html+=`<tr data-wi="${i}"${wp.inv?' class="wp-inv"':''}><td>${titolo}<div class="wprow"><select data-ww>${opts}</select><button class="linkbtn wpcfg solo-modifica" data-wcfg="${i}" title="Configura l'attacco: proprietà, competenza, bonus, danni aggiuntivi" aria-label="Configura l'attacco">⚙</button></div>`+
   `<div class="sm">${r?r.sm:''}</div>${r?`<div class="sm wpparti" title="Tiro per colpire: ${r.partiAtt}">${r.parti}</div>`:''}</td><td>${abSel(wp.a||'auto')}</td>`+
   `<td><input class="mg" data-wm value="${esc(magTxt)}" placeholder="0"${solo?' readonly title="Bonus magici diversi a colpire e ai danni: cambiali con ⚙"':''}></td><td class="num">${r?r.atk:''}</td><td class="num" style="text-align:left">${r?r.dmg:''}</td></tr>`;});
 /* armi magiche equipaggiate (e sintonizzate, se serve): righe non modificabili qui, i valori vengono dall'oggetto */
 OggettiMagici.armi(mag()).forEach(v=>{const o=v.o,ar=o.arma;let w=WEAPONS.find(x=>x[0]===ar.base);
  if(!w){w=[o.nome,ar.cat==='marziale'?'g':'s',ar.dist==='distanza'?'d':'m',ar.dado,ar.tipo,ar.prop||'—',ARMA_FL(ar.prop)];}
  const det=[ar.prop,ar.vers&&!/versatil/i.test(ar.prop||'')?'versatile ('+ar.vers+')':'',ar.g1?'gittata '+ar.g1+'/'+ar.g2+' m':'',ar.mun?'munizioni: '+ar.mun:''].filter(Boolean).map(esc).join(' · ');
  const extra=(Array.isArray(ar.extra)?ar.extra:[]).map(x=>({dadi:x.dadi,tipo:x.tipo,mod:+x.mod||0}));
  const r=calcRiga(w,{a:'auto',m:0},{bA:ar.bAtt,bD:ar.bDanno,dado:ar.dado,tipo:esc(ar.tipo),senzaComp:ar.comp==='no',sm:det,extra,nome:o.nome,magica:true,
   portata:ar.dist==='distanza'?(ar.g1?ar.g1+'/'+ar.g2+' m':''):'1,5 m'});
  r.profilo.danni[0].tipo=ar.tipo;r.profilo.oggetto=o.id;profili.push(r.profilo);
  html+=`<tr class="mag"><td colspan="3"><b>✨ ${esc(o.nome)}</b>${ar.bAtt||ar.bDanno?' <span class="rar">'+(ar.bAtt===ar.bDanno?sg(ar.bAtt):sg(ar.bAtt)+'/'+sg(ar.bDanno))+'</span>':''}<div class="sm">${r.sm}</div><div class="sm wpparti">${r.parti}</div></td><td class="num">${r.atk}</td><td class="num" style="text-align:left">${r.dmg}</td></tr>`;});
 html+=`<tr class="solo-modifica wpadd"><td colspan="5"><button class="linkbtn" id="wpAdd">＋ Aggiungi attacco</button></td></tr>`;
 /* righe automatiche */
 const auto=[];
 const ua=monk?Math.max(M.FOR,M.DES):M.FOR;const uaDie=monk?md:(st.style==='Combattimento Senz\'Armi'?'1d6':'1');
 auto.push(['Colpo senz\'armi',sg(ua+p),uaDie+(ua?sg(ua):'')+' contundente']);
 profili.push({nome:'Colpo senz\'armi',bonus:ua+p,car:monk&&M.DES>M.FOR?'DES':'FOR',competente:true,portata:'1,5 m',magica:false,danni:[{dadi:uaDie,mod:ua,tipo:'contundente'}],automatico:true});
 const cs=caster();if(cs&&cs.active)auto.push(['Attacco con incantesimo',sg(cs.atk),'CD TS '+cs.dc]);
 const r=RD();if(r&&r.dragon){const d=DRAGONS.find(x=>x[0]===st.anc);if(d)auto.push(['Arma a Soffio','CD '+(8+p+M.COS),(LT()>=16?5:LT()>=11?4:LT()>=6?3:2)+'d6 '+d[1]]);}
 html+=auto.map(x=>`<tr class="auto"><td colspan="3"><b>${x[0]}</b></td><td class="num">${x[1]}</td><td class="num" style="text-align:left">${x[2]}</td></tr>`).join('');
 PROFILI_ATTACCO=profili;
 const tb=$('atkBody');const fo=document.activeElement;const fw=fo&&fo.dataset&&fo.dataset.wm!=null&&tb.contains(fo)?fo.closest('tr').dataset.wi:null;tb.innerHTML=html;
 if(fw!=null){const ni=tb.querySelector(`tr[data-wi="${fw}"] [data-wm]`);if(ni){ni.focus();const v=ni.value.length;try{ni.setSelectionRange(v,v);}catch(e){}}}
 tb.querySelectorAll('tr[data-wi]').forEach(tr=>{tr.querySelector('[data-ww]').value=st.wp[+tr.dataset.wi].w||'';});
 const notes=[];{const av=EA.vantaggio('ATTACCO');if(gA||gD)notes.push('Oggetti magici: '+(gA?sg(gA)+' per colpire':'')+(gA&&gD?', ':'')+(gD?sg(gD)+' ai danni':'')+' (inclusi).');if(av==='vantaggio'||av==='svantaggio')notes.push(av[0].toUpperCase()+av.slice(1)+' ai tiri per colpire ('+EA.fontiVantaggio('ATTACCO').join(', ')+').');}
 if(arch)notes.push('Arcieria: +2 per colpire a distanza (incluso).');if(duel)notes.push('Duellare: +2 ai danni in mischia a una mano (incluso).');
 if(lv('barbaro'))notes.push('In ira: +'+(lv('barbaro')>=16?4:lv('barbaro')>=9?3:2)+' ai danni con attacchi in mischia basati sulla Forza.');
 if(lv('ladro'))notes.push('Attacco Furtivo: +'+Math.ceil(lv('ladro')/2)+'d6 una volta per turno.');
 if(sx.hex)notes.push('Guerriero del Sortilegio: imposta "Car" sull\'arma legata.');
 $('atkNotes').innerHTML=notes.join(' ');
}
/* ---------- PF: finestra ---------- */
let hpLevels=[];
let hpDest=null;/* {c, hp}: classe e oggetto dei PF a cui si riferisce la finestra */
function askHp(levels,why,k){const c=k?k.c:C();if(!c||!levels.length)return;hpDest={c,hp:k?k.hp:st.hp};hpLevels=levels;const avg=c.hd/2+1;
 $('mHpTxt').innerHTML=(why||'')+`Per ogni nuovo livello scegli se usare il <b>valore medio</b> del manuale (<b>${avg}</b> per il d${c.hd} del ${c.n.toLowerCase()}) oppure inserire il risultato del tuo tiro (1–${c.hd}). Il modificatore di Costituzione si aggiunge automaticamente.`;
 $('mHpRows').innerHTML=levels.map(l=>{const h=hpDest.hp[l];return `<div class="mrow">Liv. ${l}<input type="number" min="1" max="${c.hd}" data-hpl="${l}" value="${h&&h.m==='man'?h.v:''}" placeholder="${avg}"><button data-roll="${l}" title="Tira d${c.hd}">🎲</button></div>`;}).join('');
 $('mHp').classList.add('on');}
$('mHpAvg').onclick=()=>{hpLevels.forEach(l=>hpDest.hp[l]={m:'avg'});$('mHp').classList.remove('on');render();};
$('mHpMan').onclick=()=>{const c=hpDest.c;let bad=false;document.querySelectorAll('[data-hpl]').forEach(i=>{const l=+i.dataset.hpl,v=parseInt(i.value);if(v>=1&&v<=c.hd)hpDest.hp[l]={m:'man',v};else{if(i.value!=='')bad=true;hpDest.hp[l]={m:'avg'};}});
 $('mHp').classList.remove('on');render();if(bad)toast('Valori non validi sostituiti con la media');};
$('mHpRows').addEventListener('click',e=>{const b=e.target.closest('[data-roll]');if(!b)return;const c=hpDest.c;const i=document.querySelector(`[data-hpl="${b.dataset.roll}"]`);i.value=1+Math.floor(Math.random()*c.hd);});
$('bHp').onclick=()=>{if(st.lvl>1)askHp(range(2,st.lvl));else toast('Al 1° livello i PF sono il massimo del dado vita + Cos');};
const range=(a,b)=>{const o=[];for(let i=a;i<=b;i++)o.push(i);return o;};

/* ---------- equipaggiamento automatico ---------- */
function autoEq(){const c=C(),b=BG();const t=[];if(c)t.push('— '+c.n+' —\n'+c.eq);if(b)t.push('— '+b.n+' —\n'+b.eq);return t.join('\n\n');}
function syncEq(force){const a=autoEq();if(force||!st.eq||st.eq===st.eqAuto){st.eq=a;$('tEq').value=a;}st.eqAuto=a;}

/* ---------- eventi ---------- */
$('sCls').onchange=e=>{const old=st.cls;st.cls=e.target.value;st.sub='';st.sk={};st.ex={};st.asi={};st.style='';st.style2='';st.hp={};
 refreshHeaderSel();buildCfg();syncEq();render();if(st.cls&&st.lvl>1)askHp(range(2,st.lvl),'Hai scelto il '+C().n.toLowerCase()+' di livello '+st.lvl+'. ');};
$('sLvl').onchange=e=>{const old=st.lvl;st.lvl=+e.target.value;Object.keys(st.hp).forEach(l=>{if(+l>st.lvl)delete st.hp[l];});
 refreshHeaderSel();buildCfg();render();if(C()&&st.lvl>old)askHp(range(old+1,st.lvl),'Sei salito al livello '+st.lvl+'. ');};
$('sSub').onchange=e=>{st.sub=e.target.value;buildCfg();render();};
$('sRace').onchange=e=>{st.race=e.target.value;st.sr='';st.rA=[];st.anc='';st.rFeat='';st.rFeatA='';refreshHeaderSel();buildCfg();render();};
$('sSr').onchange=e=>{st.sr=e.target.value;st.rA=[];buildCfg();render();};
$('sBg').onchange=e=>{st.bg=e.target.value;const b=BG();if(b&&b.mo&&!st.coins.MO){st.coins.MO=String(b.mo);document.querySelector('[data-coin="MO"]').value=b.mo;}syncEq();buildCfg();render();};
$('sAl').onchange=e=>{st.al=e.target.value;save();};
$('bEq').onclick=()=>{syncEq(true);save();toast('Equipaggiamento iniziale ricaricato');};
/* Equipaggiamento acquistabile (web/regole/equipaggiamento.js): clic su «EQUIPAGGIAMENTO» o «aggiungi dall'elenco»;
   la scelta si aggiunge alle righe «N × oggetto» dell'equipaggiamento (se c'è già, cresce la quantità). Solo Master e
   proprietario aprono la scheda; le monete non cambiano. Richiesta dell'utente, 2026-11-06. */
const EQC=()=>typeof EQUIPAGGIAMENTO!=='undefined'?EQUIPAGGIAMENTO:[];
function eqAggiungi(nome,q){q=Math.max(1,Math.min(999,parseInt(q,10)||1));const righe=(st.eq||'').replace(/\s+$/,'').split('\n');
 const re=new RegExp('^\\s*(\\d+)\\s*[×x]\\s*'+nome.replace(/[.*+?^${}()|[\]\\]/g,'\\$&')+'\\s*$','i');let fatto=false;
 for(let i=0;i<righe.length;i++){const m=righe[i].match(re);if(m){righe[i]=(parseInt(m[1],10)+q)+' × '+nome;fatto=true;break;}}
 if(!fatto){if(righe.length===1&&righe[0]==='')righe.pop();righe.push(q+' × '+nome);}
 st.eq=righe.join('\n');$('tEq').value=st.eq;save();render();renderTools();toast('Aggiunto: '+q+' × '+nome);}
function eqDisegna(){const q=$('eqQ').value.trim().toLowerCase(),c=$('eqCat').value;let n=0;
 $('eqList').innerHTML=EQC().filter(k=>!c||k.id===c).map(k=>{const v=k.voci.filter(x=>!q||x[0].toLowerCase().includes(q)||(x[2]||'').toLowerCase().includes(q));n+=v.length;
  return v.length?`<div class="pki" style="background:var(--panel);font-weight:700">${esc(k.n)}</div>`+v.map(x=>`<div class="pki"><input class="eqq" type="number" min="1" max="999" value="1" aria-label="Quantità di ${esc(x[0])}"><button class="pkadd" data-eqadd="${esc(x[0])}" title="Aggiungi all'equipaggiamento">+</button><span class="pkn"><b>${esc(x[0])}</b>${x[2]?`<span class="pkm">${esc(x[2])}</span>`:''}</span><span class="eqp">${esc(x[1])}</span></div>`).join(''):'';}).join('')||'<div class="pki hint">Nessun oggetto trovato.</div>';
 $('eqCnt').textContent=n+' oggetti';}
function eqApri(){const sel=$('eqCat');if(!sel.options.length)sel.innerHTML='<option value="">Tutte le categorie</option>'+EQC().map(k=>`<option value="${k.id}">${esc(k.n)}</option>`).join('');
 $('eqQ').value='';eqDisegna();$('mEqC').classList.add('on');setTimeout(()=>$('eqQ').focus(),30);}
$('capEq').onclick=eqApri;$('bEqC').onclick=eqApri;$('eqClose').onclick=()=>$('mEqC').classList.remove('on');
$('eqQ').oninput=eqDisegna;$('eqCat').onchange=eqDisegna;
$('eqList').addEventListener('click',e=>{const b=e.target.closest('[data-eqadd]');if(!b)return;const q=b.parentElement.querySelector('.eqq');
 /* Gaster Bless, finanze: dentro l'app il server scala il prezzo dalle monete (gratis se lo aggiunge il Master) e aggiorna la scheda */
 if(typeof HOST!=='undefined'&&HOST.on){const v=EQC().flatMap(k=>k.voci).find(x=>x[0]===b.dataset.eqadd);HOST.send({type:'scheda:acquista',nome:b.dataset.eqadd,prezzo:v?v[1]:'',quantita:q&&q.value});if(q)q.value=1;return;}
 eqAggiungi(b.dataset.eqadd,q&&q.value);if(q)q.value=1;});
$('bTrasf').onclick=()=>{if(typeof HOST!=='undefined'&&HOST.on)HOST.send({type:'scheda:trasferisci'});else toast('Il trasferimento si fa dall\'app di Gaster Bless.');};
$('tEq').oninput=e=>{st.eq=e.target.value;save();renderTools();}; /* gli strumenti dipendono dall'equipaggiamento */
$('cInsp').onchange=e=>{st.insp=e.target.checked;save();};
$('sExh').onchange=e=>{st.exh=Math.max(0,Math.min(6,parseInt(e.target.value)||0));showExh();save();};
$('iHpCur').oninput=e=>{st.hpCur=e.target.value;save();};
$('iHpTmp').oninput=e=>{st.hpTmp=e.target.value;save();};
document.addEventListener('input',e=>{const t=e.target;
 if(t.dataset.f!=null){st.f[t.dataset.f]=t.value;document.querySelectorAll(`[data-f="${t.dataset.f}"]`).forEach(o=>{if(o!==t)o.value=t.value;});save();return;}
 if(t.dataset.base){st.base[t.dataset.base]=Math.max(1,Math.min(30,parseInt(t.value)||0));render();buildCfgKeep();return;}
 if(t.dataset.coin){st.coins[t.dataset.coin]=t.value;save();return;}
 if(t.dataset.sp){st.sp[t.dataset.sp]=t.value;save();return;}
 if(t.dataset.wm!=null){if(t.readOnly)return;const i=+t.closest('tr').dataset.wi;st.wp[i].m=t.value;delete st.wp[i].bA;delete st.wp[i].bD;clearTimeout(window._wt);window._wt=setTimeout(render,500);return;}
 if(t.dataset.asif){(st.asi[t.dataset.asif]=st.asi[t.dataset.asif]||{m:'f'}).feat=t.value;clearTimeout(window._ft);window._ft=setTimeout(()=>{render();},400);return;}
 if(t.id==='cRFeat'){st.rFeat=t.value;clearTimeout(window._ft);window._ft=setTimeout(render,400);return;}
});
function buildCfgKeep(){const tbl=document.querySelector('#cfgBody table.bd');if(!tbl)return;const A=abil();
 tbl.innerHTML='<tr><th></th><th>BASE</th><th>RAZZA</th><th>ASI/TAL.</th><th>OGGETTI</th><th>TOT.</th></tr>'+AB.map(a=>`<tr><td>${ABN[a]}</td><td>${A.D[a].base}</td><td>${A.D[a].race?'+'+A.D[a].race:'—'}</td><td>${A.D[a].asi+A.D[a].x?'+'+(A.D[a].asi+A.D[a].x):'—'}</td><td>${A.D[a].item?sg(A.D[a].item):'—'}</td><td class="t">${A.T[a]}</td></tr>`).join('');
 const h=tbl.nextElementSibling;if(h)h.innerHTML=pointBuy();}
document.addEventListener('change',e=>{const t=e.target;let reb=true;
 if(t.dataset.ra!=null){st.rA[+t.dataset.ra]=t.value;}
 else if(t.id==='cAnc')st.anc=t.value;
 else if(t.id==='cSAnc')st.sAnc=t.value.split(' ')[0];
 else if(t.id==='cLin')st.lin=t.value;
 else if(t.id==='cRFeatA')st.rFeatA=t.value;
 else if(t.id==='cRFeat'){st.rFeat=t.value;}
 else if(t.id==='cTOrig'){st.tOrig=t.checked;st.rA=[];}
 else if(t.id==='cTOpt')st.tOpt=t.checked;
 else if(t.id==='cStyle')st.style=t.value;
 else if(t.id==='cStyle2')st.style2=t.value;
 else if(t.id==='cArm')st.armor=t.value;
 else if(t.id==='cShield')st.shield=t.checked;
 else if(t.dataset.difs&&['amb','genio','resRip','totem'].includes(t.dataset.difs)){if(t.value)st[t.dataset.difs]=t.value;else delete st[t.dataset.difs];}
 else if(t.dataset.asim){const l=t.dataset.asim;st.asi[l]=Object.assign(st.asi[l]||{},{m:t.value});}
 else if(t.dataset.asia){(st.asi[t.dataset.asia]=st.asi[t.dataset.asia]||{m:'2'}).a=t.value;}
 else if(t.dataset.asib){(st.asi[t.dataset.asib]=st.asi[t.dataset.asib]||{m:'11'}).b=t.value;}
 else if(t.dataset.asifa){(st.asi[t.dataset.asifa]=st.asi[t.dataset.asifa]||{m:'f'}).fa=t.value;}
 else if(t.dataset.asif){(st.asi[t.dataset.asif]=st.asi[t.dataset.asif]||{m:'f'}).feat=t.value;}
 else if(t.dataset.ww!=null){st.wp[+t.closest('tr').dataset.wi].w=t.value;reb=false;}
 else if(t.dataset.wa!=null){st.wp[+t.closest('tr').dataset.wi].a=t.value;reb=false;}
 else if(t.dataset.spp){st.spp[t.dataset.spp]=t.checked;save();return;}
 else if(t.dataset.ds){st.ds[t.dataset.ds]=t.checked;save();return;}
 else return;
 if(reb)buildCfg();render();});
document.addEventListener('click',e=>{const t=e.target;
 if(t.dataset.sk!=null){const n=SKILLS[+t.dataset.sk][0];if(autoSkills().has(n))return;st.sk[n]=!st.sk[n];if(!st.sk[n])delete st.ex[n];render();return;}
 if(t.dataset.ex!=null){const n=SKILLS[+t.dataset.ex][0];if(!isProfSk(n)||autoExp().has(n)||!expAllowed())return;st.ex[n]=!st.ex[n];render();return;}
 if(t.dataset.slot){const lv=t.dataset.slot,i=+t.dataset.si;const u=st.slotU[lv]||0;st.slotU[lv]=i<u?i:i+1;render();return;}
 if(t.id==='cHp'){$('bHp').click();return;}
 if(t.classList.contains('tab')){document.querySelectorAll('.tab').forEach(b=>b.classList.toggle('on',b===t));document.querySelectorAll('.page').forEach(p=>p.classList.toggle('on',p.id===t.dataset.tab));return;}
 if(t.classList.contains('modal'))t.classList.remove('on');
});
function readImg(input,key,id){input.onchange=()=>{const f=input.files[0];if(!f)return;const rd=new FileReader();rd.onload=()=>{const im=new Image();im.onload=()=>{const cv=document.createElement('canvas');const sc=Math.min(1,600/Math.max(im.width,im.height));cv.width=im.width*sc;cv.height=im.height*sc;cv.getContext('2d').drawImage(im,0,0,cv.width,cv.height);st[key]=cv.toDataURL('image/jpeg',.82);setImg(id,st[key]);save();};im.src=rd.result;};rd.readAsDataURL(f);};}
readImg($('fImg'),'img','portrait');readImg($('fSym'),'sym','symbol');

/* esporta / importa / nuovo */
let ioMode='exp';
$('bExp').onclick=()=>{ioMode='exp';$('mIOt').textContent='Esporta personaggio';$('mIOp').textContent='Copia questo testo e conservalo: potrai reimportarlo con "Importa".';$('mIOtx').value=JSON.stringify(st);$('mIOok').textContent='Copia';$('mIOf').style.display='none';$('mIO').classList.add('on');};
$('bImp').onclick=()=>{ioMode='imp';$('mIOt').textContent='Importa personaggio';$('mIOp').textContent='Incolla il testo esportato oppure carica un file .json.';$('mIOtx').value='';$('mIOok').textContent='Importa';$('mIOf').style.display='';$('mIO').classList.add('on');};
$('mIOc').onclick=()=>$('mIO').classList.remove('on');
$('fJson').onchange=e=>{const f=e.target.files[0];if(!f)return;f.text().then(t=>{$('mIOtx').value=t;});};
$('mIOok').onclick=async()=>{if(ioMode==='exp'){try{await navigator.clipboard.writeText($('mIOtx').value);toast('Copiato negli appunti');}catch(e){$('mIOtx').select();toast('Seleziona il testo e copialo');}return;}
 try{const o=JSON.parse($('mIOtx').value);st=Object.assign(blank(),o);$('mIO').classList.remove('on');boot();toast('Personaggio importato');}catch(e){toast('Testo non valido');}};
$('bNew').onclick=()=>$('mNew').classList.add('on');
$('mNewNo').onclick=()=>$('mNew').classList.remove('on');
$('mNewOk').onclick=()=>{st=blank();$('mNew').classList.remove('on');boot();};

function boot(){migrateSpells();initStatic();refreshHeaderSel();buildCfg();render();}
const dl=document.createElement('datalist');dl.id='featList';dl.innerHTML=FEATS.map(f=>`<option value="${esc(f[0])}">${esc(f[1])} · ${esc(f[2])}</option>`).join('');document.body.appendChild(dl);
/* ===== INCANTESIMI: selettore e dettagli ===== */
const SCH={A:'Abiurazione',C:'Evocazione',D:'Divinazione',E:'Ammaliamento',V:'Invocazione',I:'Illusione',N:'Necromanzia',T:'Trasmutazione'};
const CLS_CODE={artefice:'a',bardo:'b',chierico:'c',druido:'d',paladino:'p',ranger:'r',stregone:'s',warlock:'w',mago:'m'};
const CODE_N={a:'Artefice',b:'Bardo',c:'Chierico',d:'Druido',p:'Paladino',r:'Ranger',s:'Stregone',w:'Warlock',m:'Mago'};
const SRC_N={PHB:'Manuale del Giocatore',XGE:'Guida Omnicomprensiva di supplemento',TCE:'Calderone Omnicomprensivo di supplemento'};
const SP=SPELLS.map(s=>({it:s[0],en:s[1],l:s[2],sc:s[3],t:s[4],r:s[5],cp:s[6],d:s[7],cl:s[8],src:s[9],ds:s[10],hi:s[11]||''}));
SP.sort((a,b)=>a.l-b.l||a.it.localeCompare(b.it,'it'));
const SPI={};SP.forEach(s=>SPI[s.en]=s);
/* Incantesimi importati dai manuali del Master (2026-11-11, «📥 Importa dai manuali»): arrivano con scheda:init. Aggiornano
   i valori (livello, scuola, tempo, gittata, componenti, durata; le classi lette si aggiungono) e aggiungono gli incantesimi
   nuovi. Il testo del manuale non arriva mai qui: il Master lo chiede a parte («📖 Testo del manuale»). */
SRC_N.MAN='Manuale del Master';
function applicaIncantesimiImportati(l){if(!Array.isArray(l))return;
 for(const x of l){if(!x||typeof x.voce!=='string'||!x.voce)continue;
  let s=SPI[x.voce];
  if(!s){if(!x.nuovo)continue;s={it:String(x.nome||x.voce).slice(0,80),en:x.voce,l:0,sc:'V',t:'a',r:'',cp:'',d:'I',cl:'',src:'MAN',
   ds:'Descrizione nel manuale del Master («'+String(x.fonte||'manuale').slice(0,80)+'», p. '+(+x.pagina||'?')+').',hi:''};SP.push(s);SPI[x.voce]=s;}
  if(Number.isInteger(x.l)&&x.l>=0&&x.l<=9)s.l=x.l;if(SCH[x.sc])s.sc=x.sc;
  for(const k of['t','r','d'])if(typeof x[k]==='string'&&x[k])s[k]=x[k].slice(0,60);
  if(typeof x.cp==='string'&&/^[VSM]{1,3}$/.test(x.cp))s.cp=x.cp;
  if(typeof x.cl==='string')for(const c of x.cl)if(CODE_N[c]&&!s.cl.includes(c))s.cl+=c;
  s.man={fonte:String(x.fonte||'').slice(0,80),pagina:+x.pagina||0};}
 SP.sort((a,b)=>a.l-b.l||a.it.localeCompare(b.it,'it'));}
const UNIT={m:['minuto','minuti'],h:['ora','ore'],r:['round','round'],g:['giorno','giorni'],d:['giorno','giorni']};
function fmtDur(d){if(d==='I')return'Istantanea';if(d==='P')return'Finché non viene dissolto';if(d==='S')return'Speciale';
 const m=/^(C?)(\d+)([mhrgd])$/.exec(d);if(!m)return d;const n=+m[2],u=UNIT[m[3]][n===1?0:1];return(m[1]?'Concentrazione, fino a ':'')+n+' '+u;}
function fmtCast(t){const rit=t[0]==='*';const x=rit?t.slice(1):t;const M={a:'1 azione',ab:'1 azione bonus',re:'1 reazione'};
 let s=M[x];if(!s){const m=/^(\d+)([mh])$/.exec(x);s=m?m[1]+' '+UNIT[m[2]][+m[1]===1?0:1]:x.replace('a o','1 azione o').replace(/(\d+)h/,'$1 ore');}return s+(rit?' (rituale)':'');}
const fmtComp=c=>c.split('').join(', ');
const isConc=s=>s.d[0]==='C';const isRit=s=>s.t[0]==='*';
function lvlName(l){return l===0?'Trucchetto':l+'° livello';}

let pickCls='';/* classe per cui si sceglie l'incantesimo (multiclasse) */
function spellCode(){if(pickCls==='extra')return null;const cs=caster();if(cs&&cs.mc){const x=cs.classi.find(y=>y.id===pickCls)||cs.classi[0];return x.codice;}const c=C();if(!c)return null;if(CLS_CODE[st.cls]&&c.cast)return CLS_CODE[st.cls];if(subOn()&&SX().cast==='third')return'm';return null;}
function maxSpellLvl(cs){if(!cs||!cs.active)return 0;return Regole5e.livelloMassimo(cs.inc);}
function spList(l){if(!st.spl)st.spl={};return st.spl[l]||(st.spl[l]=[]);}
/* incantesimi concessi da sottoclasse e razza (regole/incantesimi_auto.js, portato dal ramo main-ke8sy4):
   compaiono con ★ e non contano nei limiti; le sottoclassi valgono per ogni classe del personaggio */
function autoSpells(){const out=[],seen=new Set();
 const push=(n,why,modo)=>{const s=SPI[n];if(!s||seen.has(n))return;seen.add(n);out.push({k:n,l:s.l,why,modo});};
 const fromTable=(t,lvl,why,modo)=>{if(!t)return;Object.entries(t).forEach(([k,v])=>{if(/^\d+$/.test(k)&&lvl>=+k&&Array.isArray(v))v.forEach(n=>push(n,why,modo));});};
 KL().forEach(k=>{if(!k.subOn||!k.sub)return;const sp=SUB_SPELLS[k.sub.n];if(!sp)return;fromTable(sp,k.lvl,k.sub.n,sp.modo);fromTable(sp.tr,k.lvl,k.sub.n,'con');
  if(sp.terra&&st.terra&&LAND_SPELLS[st.terra])fromTable(LAND_SPELLS[st.terra],k.lvl,k.sub.n+' ('+st.terra+')','prep');});
 const r=RACE_SPELLS[st.race+'/'+(st.sr||'')];if(r)fromTable(r,LT(),(RD()||{}).n||'Razza','razza');
 return out;}
const subSpellTables=()=>KL().filter(k=>k.subOn&&k.sub&&SUB_SPELLS[k.sub.n]).map(k=>SUB_SPELLS[k.sub.n]);
function migrateSpells(){if(!st.spl)st.spl={};if(st.sp&&Object.keys(st.sp).length){Object.entries(st.sp).forEach(([k,v])=>{if(!v||!v.trim())return;const [l]=k.split('-');const f=SP.find(s=>s.it.toLowerCase()===v.trim().toLowerCase()||s.en.toLowerCase()===v.trim().toLowerCase());spList(l).push(f?{k:f.en,p:!!st.spp[k]}:{c:v.trim(),p:!!st.spp[k]});});st.sp={};st.spp={};}}

/* pagina incantesimi */
function renderSpells(A,p){
 const c=C(),cs=caster();const ab=cs?cs.ab:'';const code=spellCode();
 $('oSpCls').textContent=c?(cs&&cs.mc?cs.classi.map(x=>x.nome).join(' / '):cs?c.n+(c.cast?'':' ('+S().n+')'):c.n):'—';
 $('oSpAb').textContent=cs?ABN[ab]:'—';$('oSpDC').textContent=cs?cs.dc:'—';$('oSpAtk').textContent=cs?sg(cs.atk):'—';
 $('oSpN').textContent=cs&&cs.known!=null?cs.known:'—';$('lSpN').textContent=cs?cs.kLabel:'Incantesimi conosciuti';
 const mx=maxSpellLvl(cs);
 const AU=autoSpells(),AK=new Set(AU.map(x=>x.k));const nonConta=x=>x.o==='extra'||(x.k&&AK.has(x.k));
 let cnt0=spList(0).filter(x=>!nonConta(x)).length,cntL=0,cntP=0;for(let l=1;l<=9;l++){spList(l).forEach(x=>{if(nonConta(x))return;cntL++;if(x.p)cntP++;});}
 const info=[];
 if(c&&!cs&&!multi())info.push(c.n+(c.subs.some(s=>s[3]&&s[3].cast)?' non lancia incantesimi (a meno di una sottoclasse incantatrice dal 3° livello).':' non lancia incantesimi.'));
 else if(cs&&!cs.active)info.push('Gli incantesimi si ottengono al '+(cs.type==='half'?'2°':'3°')+' livello.');
 else if(cs){
  if(cs.cantrips)info.push(`Trucchetti: <b class="${cnt0>cs.cantrips?'bad':''}">${cnt0}/${cs.cantrips}</b>`);
  if(cs.known!=null){if(cs.kLabel==='Incantesimi preparati'){info.push(`Preparati: <b class="${cntP>cs.known?'bad':''}">${cntP}/${cs.known}</b> (spunta il cerchio accanto all'incantesimo)`);}
   else info.push(`Conosciuti: <b class="${cntL>cs.known?'bad':''}">${cntL}/${cs.known}</b>`);}
  info.push('Livello massimo lanciabile: '+mx+'°');
  if(cs.pact)info.push(`Magia del Patto: ${cs.pact.n} slot di ${cs.pact.lv}° livello (riposo breve)`);
  if(lv('mago'))info.push('Libro: almeno '+(6+2*(lv('mago')-1))+' incantesimi');
  if(cs.mc)infoMulticlasse(cs,info);}
 const SPS=subSpellTables();
 if(SPS.some(x=>x.terra))info.push(`Terreno del circolo: <select id="spTerra"><option value="">— scegli —</option>${Object.keys(LAND_SPELLS).map(t=>`<option${st.terra===t?' selected':''}>${t}</option>`).join('')}</select>`);
 if(AU.length)info.push('★ = concesso da sottoclasse o razza: non conta nei limiti');
 const notes=[...SPS.map(x=>x.nota),RACE_CANTRIP_CHOICE[st.race+'/'+(st.sr||'')]].filter(Boolean);
 if(notes.length)info.push(notes.map(esc).join(' '));
 $('spInfo').innerHTML=info.join(' · ');
 let h='';
 for(let lv=0;lv<=9;lv++){
  let tot=0,label='';
  if(lv===0)label='TRUCCHETTI';
  else if(cs){tot=cs.slots[lv-1]||0;if(cs.pact){if(lv===cs.pact.lv){tot+=cs.pact.n;label=cs.slots[lv-1]?'SLOT: '+tot+' (con il patto)':'';}if(cs.pact.arcanum.includes(lv)){tot=Math.max(tot,1);label='ARCANUM MISTICO';}}}
  const used=Math.min(st.slotU[lv]||0,tot);const L=spList(lv);const AL=AU.filter(x=>x.l===lv);
  const avail=lv===0?(cs&&cs.cantrips>0):lv<=mx;
  const off=lv>0&&!tot&&!L.length&&!AL.length&&!(cs&&cs.pact&&lv<=mx);
  let slots='';if(lv>0)slots='<span class="slots">'+(tot?'':'—')+Array.from({length:tot},(_,i)=>`<button class="slot${i<used?' used':''}" data-slot="${lv}" data-si="${i}" title="Slot ${i<used?'speso':'disponibile'}"></button>`).join('')+'</span>';
  h+=`<div class="splv${off?' off':''}"><div class="splh"><span class="n">${lv}</span><span class="t">${lv===0?label:label||'SLOT: '+tot}</span>${slots}</div>`;
  AL.forEach(x=>{const s=SPI[x.k];const tags=(isConc(s)?'<span class="tg" title="Concentrazione">C</span>':'')+(isRit(s)?'<span class="tg" title="Rituale">R</span>':'');
   const what=x.modo==='prep'?'sempre preparato':x.modo==='razza'?'tratto razziale':'conosciuto in più';
   h+=`<div class="spl auto" title="${esc(x.why+': '+what)}"><span class="spstar on" aria-label="Concesso">★</span><button class="spn" data-spinfok="${esc(x.k)}">${esc(s.it)}</button>${tags}<span class="spwhy">${esc(what)}</span></div>`;});
  L.forEach((x,i)=>{const s=x.k?SPI[x.k]:null;const nm=s?s.it:x.c;
   const tags=s?(isConc(s)?'<span class="tg" title="Concentrazione">C</span>':'')+(isRit(s)?'<span class="tg" title="Rituale">R</span>':''):'<span class="tg">pers.</span>';
   const og=x.o==='extra'?'<span class="tg" title="Bonus di razza, talento o sottoclasse: non conta nei limiti">bonus</span>':cs&&cs.mc&&x.o&&CLASSES[x.o]?`<span class="tg" title="Preso come ${esc(CLASSES[x.o].n)}">${esc(CLASSES[x.o].n.slice(0,3))}</span>`:'';
   h+=`<div class="spl">${lv?`<input type="checkbox" data-sprep="${lv}-${i}"${x.p?' checked':''} title="Preparato">`:''}<button class="spn" data-spinfo="${lv}-${i}">${esc(nm)}</button>${tags}${og}${x.k&&AK.has(x.k)?'<span class="spwhy">già concesso</span>':''}<button class="spx" data-sprm="${lv}-${i}" title="Rimuovi">×</button></div>`;});
  if(!L.length&&!AL.length)h+='<div class="spl empty">nessun incantesimo</div>';
  h+=`<button class="spadd${avail?'':' dim'}" data-spadd="${lv}">+ Aggiungi ${lv?'incantesimo':'trucchetto'}</button></div>`;}
 $('spGrid').innerHTML=h;
}

/* selettore */
let pickLv=0;
function openPicker(lv){pickLv=lv;const cs0=caster();if(pickCls!=='extra'&&!(cs0&&cs0.classi.some(y=>y.id===pickCls)))pickCls=cs0?cs0.classi[0].id:'';selettoreClassePicker(cs0);const code=spellCode();
 $('pkT').textContent='Aggiungi '+(lv?'incantesimo di '+lv+'° livello':'trucchetto');
 $('pkAll').checked=!code;$('pkAllL').style.display=code?'':'none';$('pkQ').value='';$('pkSch').value='';
 $('pkSub').textContent=code?'Lista del '+CODE_N[code].toLowerCase()+(code==='m'&&!CLS_CODE[st.cls]?' (sottoclasse incantatrice)':''):'Nessuna classe incantatrice: vengono mostrate tutte le liste';
 renderPicker();$('mSp').classList.add('on');setTimeout(()=>$('pkQ').focus(),50);}
function renderPicker(){const code=spellCode();const all=$('pkAll').checked||!code;const q=$('pkQ').value.trim().toLowerCase();const sch=$('pkSch').value;
 const have=new Set(spList(pickLv).map(x=>x.k).filter(Boolean));
 const R=SP.filter(s=>s.l===pickLv&&(all||s.cl.includes(code))&&(!sch||s.sc===sch)&&(!q||s.it.toLowerCase().includes(q)||s.en.toLowerCase().includes(q)));
 $('pkList').innerHTML=R.map(s=>`<div class="pki${have.has(s.en)?' on':''}"><button class="pkadd" data-pk="${esc(s.en)}" title="${have.has(s.en)?'Rimuovi':'Aggiungi'}">${have.has(s.en)?'✓':'+'}</button><button class="pkn" data-pkinfo="${esc(s.en)}"><b>${esc(s.it)}</b> <small>${esc(s.en)}</small><span class="pkm">${SCH[s.sc]} · ${fmtCast(s.t)}${isConc(s)?' · <span class="tg">C</span>':''}${isRit(s)?' <span class="tg">R</span>':''}${!s.cl.includes(code||'-')&&code?' · <span class="warn">altra lista</span>':''}<span class="src">${s.src}</span></span></button></div>`).join('')||'<div class="hint" style="padding:12px">Nessun incantesimo trovato.</div>';
 $('pkCnt').textContent=R.length+' incantesimi';}
function toggleSpell(en){const L=spList(pickLv);const i=L.findIndex(x=>x.k===en);if(i>=0)L.splice(i,1);else L.push(pickCls?{k:en,p:false,o:pickCls}:{k:en,p:false});L.sort((a,b)=>(a.k?SPI[a.k].it:a.c).localeCompare(b.k?SPI[b.k].it:b.c,'it'));render();renderPicker();}

/* dettaglio */
let infoCtx=null;
function openInfo(s,ctx){infoCtx=ctx;
 $('sdN').textContent=s.it;$('sdE').textContent=s.en;
 $('sdL').textContent=(s.l===0?'Trucchetto di '+SCH[s.sc].toLowerCase():SCH[s.sc]+' di '+s.l+'° livello')+(isRit(s)?' (rituale)':'');
 $('sdG').innerHTML=[['Tempo di lancio',fmtCast(s.t)],['Gittata',s.r],['Componenti',fmtComp(s.cp)],['Durata',fmtDur(s.d)]].map(x=>`<div><span class="lbl">${x[0]}</span><b>${esc(x[1])}</b></div>`).join('');
 const cs=caster();let extra='';
 if(cs&&cs.active){const dmg=/TS su (\w+)/.exec(s.ds);extra=`<div class="sdcalc">Con la tua scheda: ${/attacco (in mischia|a distanza) con incantesimo/i.test(s.ds)?'tiro per colpire <b>'+sg(cs.atk)+'</b> · ':''}${dmg?'CD del TS <b>'+cs.dc+'</b>':'CD incantesimi '+cs.dc}${s.ds.includes('mod. da incantatore')?' · mod. da incantatore <b>'+sg(cs.m)+'</b>':''}</div>`;}
 $('sdD').innerHTML=`<p>${esc(s.ds)}</p>`+(s.hi?`<p><b>${s.l===0?'Potenziamento dei trucchetti':'Ai livelli superiori'}.</b> ${esc(s.hi)}</p>`:'')+extra;
 $('sdC').innerHTML='<b>Classi:</b> '+s.cl.split('').map(x=>CODE_N[x]).join(', ')+' · <b>Fonte:</b> '+(s.man&&s.man.fonte?esc(s.man.fonte)+(s.man.pagina?', p. '+s.man.pagina:'')+' <span title="Valori aggiornati con «Importa dai manuali»">📥</span>':SRC_N[s.src]);
 /* Il testo del manuale lo vede solo il Master (lo chiede al server ogni volta: non resta nella scheda) */
 if(s.man&&typeof HOST!=='undefined'&&HOST.info&&HOST.info.role==='master'){const b=document.createElement('button');b.className='btn sm';b.textContent='📖 Testo del manuale';b.title='Solo per il Master';
  b.onclick=async()=>{b.disabled=true;try{const r=await fetch('/api/v1/elementi-manuali/testo?tipo=incantesimo&voce='+encodeURIComponent(s.en),{credentials:'same-origin'});if(!r.ok)throw new Error();const j=await r.json();
   const d=document.createElement('div');d.className='sdman';d.innerHTML='<p><b>Dal manuale («'+esc(j.manualeNome||'')+'», p. '+(+j.pagina||'?')+'):</b> '+esc(j.testo||'')+'</p>'+(j.materiali?'<p><b>Componenti materiali:</b> '+esc(j.materiali)+'</p>':'')+(j.superiori?'<p><b>'+(s.l===0?'Potenziamento dei trucchetti':'Ai livelli superiori')+'.</b> '+esc(j.superiori)+'</p>':'');
   b.replaceWith(d);}catch(er){b.disabled=false;b.textContent='📖 Testo non disponibile';}};
  $('sdC').appendChild(b);}
 /* Lancio dalla scheda (richiesta dell'utente, 2026-11-06): solo per chi lancia incantesimi */
 const daScheda=!!(ctx&&ctx.from==='sheet'&&cs&&cs.active);infoSp=s;
 $('sdCast').style.display=daScheda?'':'none';$('sdRit').style.display=daScheda&&isRit(s)&&s.l>0?'':'none';
 $('sdClose').textContent=daScheda?'Annulla':'Chiudi';
 const btn=$('sdAct');if(ctx&&ctx.from==='pick'){const have=spList(pickLv).some(x=>x.k===s.en);btn.style.display='';btn.textContent=have?'Rimuovi dalla scheda':'Aggiungi alla scheda';btn.dataset.en=s.en;}else btn.style.display='none';
 $('mSd').classList.add('on');}

document.addEventListener('click',e=>{const t=e.target.closest('[data-spadd],[data-spinfo],[data-sprm],[data-pk],[data-pkinfo],[data-spinfok]');if(!t)return;
 if(t.dataset.spinfok){const s=SPI[t.dataset.spinfok];if(s)openInfo(s,{from:'sheet'});return;}
 if(t.dataset.spadd!=null){openPicker(+t.dataset.spadd);return;}
 if(t.dataset.spinfo){const [l,i]=t.dataset.spinfo.split('-').map(Number);const x=spList(l)[i];if(x&&x.k)openInfo(SPI[x.k],{from:'sheet'});else if(x){toast('Incantesimo personalizzato: aggiungi le note in "Tratti & Privilegi aggiuntivi"');}return;}
 if(t.dataset.sprm){const [l,i]=t.dataset.sprm.split('-').map(Number);spList(l).splice(i,1);render();return;}
 if(t.dataset.pk){toggleSpell(t.dataset.pk);return;}
 if(t.dataset.pkinfo){openInfo(SPI[t.dataset.pkinfo],{from:'pick'});return;}});
document.addEventListener('change',e=>{const t=e.target;if(t.id==='spTerra'){st.terra=t.value;render();return;}if(t.dataset&&t.dataset.sprep){const [l,i]=t.dataset.sprep.split('-').map(Number);const x=spList(l)[i];if(x){x.p=t.checked;render();}}});
$('pkQ').oninput=renderPicker;$('pkSch').onchange=renderPicker;$('pkAll').onchange=renderPicker;
$('pkClose').onclick=()=>$('mSp').classList.remove('on');
$('pkCustomAdd').onclick=()=>{const v=$('pkCustom').value.trim();if(!v)return;spList(pickLv).push({c:v,p:false});$('pkCustom').value='';render();toast('Aggiunto: '+v);};
$('sdClose').onclick=()=>$('mSd').classList.remove('on');
/* slot totali di un livello (come nella pagina degli incantesimi: slot normali, Magia del Patto, Arcanum) */
let infoSp=null;
function slotTot(cs,lv){if(!cs||lv<1)return 0;let tot=cs.slots[lv-1]||0;if(cs.pact){if(lv===cs.pact.lv)tot+=cs.pact.n;if(cs.pact.arcanum.includes(lv))tot=Math.max(tot,1);}return tot;}
/* scelta = {slot: livello scelto} oppure {senzaSlot: motivo} (finestra della scelta dello slot, incantesimi_ui.js, 2026-11-11);
   senza scelta: il primo slot libero dal livello base in su (comportamento di prima). Lo slot si toglie solo qui, alla conferma. */
function lanciaIncantesimo(rituale,scelta){const s=infoSp,cs=caster();if(!s)return;let slot=0;
 if(!rituale&&s.l>0&&!(scelta&&scelta.senzaSlot)){
  if(scelta&&scelta.slot){slot=+scelta.slot;if(slot<s.l||slot>9||(st.slotU[slot]||0)>=slotTot(cs,slot)){toast('Lo slot di '+slot+'° livello non è più disponibile.',4000);return;}}
  else for(let lv=s.l;lv<=9;lv++){if((st.slotU[lv]||0)<slotTot(cs,lv)){slot=lv;break;}}
  if(!slot){toast('Nessuno slot libero di '+s.l+'° livello o superiore: recupera gli slot con un riposo.',4000);return;}
  st.slotU[slot]=(st.slotU[slot]||0)+1;save();render();}
 const inc={nome:s.it,en:s.en,livello:s.l,slot,rituale:!!rituale,intestazione:$('sdL').textContent,tempo:fmtCast(s.t)+(rituale?' + 10 minuti (rituale)':''),
  gittata:s.r,componenti:fmtComp(s.cp),durata:fmtDur(s.d),descrizione:s.ds+(scelta&&scelta.effetto?' — Al livello scelto: '+scelta.effetto+'.':''),superiori:s.hi||''};
 $('mSd').classList.remove('on');
 const nota=rituale?' come rituale (nessuno slot)':scelta&&scelta.senzaSlot?' senza slot ('+scelta.senzaSlot+')':slot?' · slot di '+slot+'° livello'+(!scelta&&slot>s.l?' (quelli di '+s.l+'° erano finiti)':''):'';
 if(typeof HOST!=='undefined'&&HOST.on){let effetto=null;try{if(!rituale&&typeof window.effettoScontro==='function')effetto=window.effettoScontro(s,slot);}catch(e){}
  HOST.send({type:'scheda:incantesimo',inc,effetto});toast('✨ '+s.it+nota+': descrizione condivisa con il tavolo',4000);}
 else toast('✨ '+s.it+nota+' (fuori dall\'app non si condivide)',4000);}
$('sdCast').onclick=()=>lanciaIncantesimo(false);
$('sdRit').onclick=()=>lanciaIncantesimo(true);
$('sdAct').onclick=()=>{toggleSpell($('sdAct').dataset.en);$('mSd').classList.remove('on');};
fillSel($('pkSch'),Object.entries(SCH).map(([k,v])=>[k,v]),'','Tutte le scuole');
// Esc: chiude la finestra interna in cima; se non ce n'è, passa il tasto a Gaster Bless (avvisi a tutto schermo, 2026-11-08).
document.addEventListener('keydown',e=>{if(e.key==='Escape'){const m=[...document.querySelectorAll('.modal.on')].pop();if(m)m.classList.remove('on');else if(window.parent!==window){try{window.parent.postMessage({type:'gasterbless:esc'},'*');}catch(_){}}}});

/* ===== CALCOLATORE ACQUISTO A PUNTI ===== */
const PB_COST_DEF={3:-9,4:-6,5:-4,6:-2,7:-1,8:0,9:1,10:2,11:3,12:4,13:5,14:7,15:9,16:12,17:15,18:19};
const PRIO={artefice:['INT','COS','DES'],barbaro:['FOR','COS','DES'],bardo:['CAR','DES','COS'],chierico:['SAG','COS','FOR'],druido:['SAG','COS','DES'],guerriero:['FOR','COS','DES'],ladro:['DES','COS','SAG'],mago:['INT','COS','DES'],monaco:['DES','SAG','COS'],paladino:['FOR','CAR','COS'],ranger:['DES','SAG','COS'],stregone:['CAR','COS','DES'],warlock:['CAR','COS','DES']};
let pb=null;
function pbCfg(){if(!st.pb)st.pb={budget:27,min:8,max:15,cost:{...PB_COST_DEF},saves:[]};return st.pb;}
function pbCost(v){const c=pbCfg().cost;return c[v]!=null?+c[v]:NaN;}
function pbAB(){return AB;} /* la Sanità mentale non è più nella scheda: è una funzione separata dell'app */
const pbBudget=()=>pbCfg().budget;
const pbNome=a=>ABN[a];
function openPb(){const c=pbCfg();pb={};pbAB().forEach(a=>{pb[a]=Math.max(c.min,Math.min(c.max,+st.base[a]||8));});
 $('pbB').value=c.budget;$('pbMin').value=c.min;$('pbMax').value=c.max;$('pbCustom').open=false;renderPbCost();renderPb();$('mPb').classList.add('on');}
function renderPb(){const c=pbCfg();const R={};racialList().forEach(b=>{if(b.ab)R[b.ab]=(R[b.ab]||0)+b.v;});let spent=0,bad=false;
 $('pbRows').innerHTML=pbAB().map(a=>{const v=pb[a];const k=pbCost(v);if(isNaN(k))bad=true;else spent+=k;const tot=v+(R[a]||0);const m=Math.floor((tot-10)/2);
  return `<tr><td>${pbNome(a)}</td><td class="pbsc"><button data-pbm="${a}"${v<=c.min?' disabled':''}>−</button><b>${v}</b><button data-pbp="${a}"${v>=c.max?' disabled':''}>+</button></td><td>${R[a]?'+'+R[a]:'—'}</td><td class="t">${tot}</td><td>${sg(m)}</td><td>${isNaN(k)?'?':k}</td></tr>`;}).join('');
 const B=pbBudget(),left=B-spent;$('pbSpent').innerHTML=`Punti spesi <b class="${left<0?'bad':''}">${spent}/${B}</b> · ${left>=0?'restanti '+left:'<span class="warn">'+(-left)+' oltre il limite</span>'}`;
 $('pbBar').style.width=Math.min(100,Math.max(0,spent/B*100))+'%';$('pbBar').style.background=left<0?'var(--acc2)':left===0?'var(--ok)':'var(--gold)';
 const r=RD();$('pbRace').innerHTML=r?'Bonus razziali di <b>'+esc(r.n)+'</b>'+(RACES[st.race].subs&&!st.sr?' — <span class="warn">scegli la sottorazza</span>':'')+(racialList().some(b=>!b.ab)?' — <span class="warn">assegna i bonus a scelta nelle Scelte del personaggio</span>':''):'Nessuna razza selezionata: i bonus razziali appariranno qui.';
 $('pbSaves').innerHTML=c.saves.map((s,i)=>`<span class="pbsv"><button data-pbload="${i}" title="Carica">${esc(s.n)}: ${AB.map(a=>s.v[a]).join('/')}</button><button data-pbdel="${i}" title="Elimina">×</button></span>`).join('')||'<span class="hint">Nessuna serie salvata.</span>';
 $('pbApply').disabled=left<0;}
function renderPbCost(){const c=pbCfg();let h='';for(let v=3;v<=18;v++)h+=`<label>${v}<input type="number" data-pbc="${v}" value="${c.cost[v]!=null?c.cost[v]:''}"></label>`;$('pbCost').innerHTML=h;}
$('mPb').addEventListener('click',e=>{const t=e.target.closest('button');if(!t)return;const c=pbCfg();
 if(t.dataset.pbm){pb[t.dataset.pbm]=Math.max(c.min,pb[t.dataset.pbm]-1);renderPb();}
 else if(t.dataset.pbp){const a=t.dataset.pbp;const nv=Math.min(c.max,pb[a]+1);pb[a]=nv;renderPb();}
 else if(t.dataset.pbload){const s=c.saves[+t.dataset.pbload];AB.forEach(a=>pb[a]=s.v[a]);renderPb();}
 else if(t.dataset.pbdel){c.saves.splice(+t.dataset.pbdel,1);save();renderPb();}});
['pbB','pbMin','pbMax'].forEach(id=>$(id).oninput=()=>{const c=pbCfg();c.budget=+$('pbB').value||0;c.min=Math.max(3,+$('pbMin').value||3);c.max=Math.min(18,Math.max(c.min,+$('pbMax').value||18));AB.forEach(a=>pb[a]=Math.max(c.min,Math.min(c.max,pb[a])));save();renderPb();});
$('pbCost').addEventListener('input',e=>{const t=e.target;if(!t.dataset.pbc)return;const c=pbCfg();if(t.value==='')delete c.cost[t.dataset.pbc];else c.cost[t.dataset.pbc]=+t.value;save();renderPb();});
$('pbResetRules').onclick=()=>{st.pb.budget=27;st.pb.min=8;st.pb.max=15;st.pb.cost={...PB_COST_DEF};openPb();};
$('pbReset').onclick=()=>{AB.forEach(a=>pb[a]=Math.max(pbCfg().min,Math.min(pbCfg().max,8)));renderPb();};
$('pbStd').onclick=()=>{const arr=[15,14,13,12,10,8];const pr=PRIO[st.cls]||[];const order=[...pr,...['COS','DES','SAG','FOR','INT','CAR'].filter(a=>!pr.includes(a))];
 /* le restanti in ordine utile: Cos/Des prima di quelle meno usate */
 order.forEach((a,i)=>pb[a]=Math.max(pbCfg().min,Math.min(pbCfg().max,arr[i])));renderPb();toast(pr.length?'Serie standard ordinata per il '+C().n.toLowerCase():'Serie standard applicata in ordine');};
$('pbSave').onclick=()=>{const c=pbCfg();const n='Serie '+(c.saves.length+1);c.saves.push({n,v:{...pb}});save();renderPb();};
$('pbApply').onclick=()=>{AB.forEach(a=>st.base[a]=pb[a]);document.querySelectorAll('[data-base]').forEach(i=>i.value=st.base[i.dataset.base]);$('mPb').classList.remove('on');buildCfg();render();toast('Punteggi applicati alla scheda');};
$('pbClose').onclick=()=>$('mPb').classList.remove('on');
$('bPb').onclick=openPb;
document.addEventListener('click',e=>{if(e.target.id==='cPb')openPb();});
/* ===== PRIVILEGI CLICCABILI CON SPIEGAZIONE ===== */
const FEAT_D={'allerta':'+5 all\'iniziativa (già calcolato), non puoi essere sorpreso da coscienti e chi è nascosto non ha vantaggio contro di te.','mobile':'+3 m di velocità (già calcolato); lo Scatto ignora il terreno difficile e chi attacchi in mischia non può fare attacchi di opportunità contro di te.','osservatore':'+1 a Intelligenza o Saggezza, leggi le labbra e +5 a Percezione e Indagare passive (già calcolato).','resiliente':'+1 alla caratteristica scelta e competenza nei TS di quella caratteristica (già calcolato).','robusto':'Il massimo dei PF aumenta di 2 per ogni livello (già calcolato).'};
let FI=[];
function fiAdd(t,d,sub){FI.push({t,d,sub});return FI.length-1;}
function fiRow(lv,t,d,sub){const i=fiAdd(t,d,sub);return `<button class="fi" data-fi="${i}">${lv!=null?`<span class="lv">${lv}</span>`:''}<span class="ft">${esc(t)}</span>${d?'<span class="fq">i</span>':''}</button>`;}
function renderFeatures(){
 FI=[];const c=C(),r=RD(),b=BG(),L=st.lvl,s=subOn()?S():null;let h='';const mc=multi();
 const res=[...resources(),...raceExtras()];
 if(res.length)h+='<h4>RISORSE</h4>'+res.map(x=>{const i=fiAdd(x[0],(FD_RES[x[0]]||'')+'\n\nValore attuale: '+x[1],'Risorsa');return `<button class="res" data-fi="${i}"><span>${esc(x[0])}</span><b>${esc(x[1])}</b></button>`;}).join('');
 if(r)h+=`<h4>${esc(r.n.toUpperCase())}<span class="src">${r.src}</span></h4>`+r.t.map(t=>fiRow(null,t,featDesc(null,null,t),'Tratto razziale · '+r.n)).join('');
 if(c){const fl=parseF(c.f).filter(x=>x[0]<=L).map(x=>[x[0],x[1],featDesc(st.cls,null,x[1])]);
  (c.asi||ASI_STD).filter(l=>l<=L).forEach(l=>fl.push(rigaAumento(l,String(l))));
  if(st.tOpt&&c.topt)parseF(c.topt).filter(x=>x[0]<=L).forEach(x=>fl.push([x[0],x[1]+' ✦',featDesc(st.cls,null,x[1])]));
  if(st.style){const sd=[...STYLES,...STYLES_T].find(x=>x[0]===st.style);fl.push([c.style===2?2:c.style===1?1:3,'Stile: '+st.style,sd?sd[1]+'.':'']);}
  fl.sort((a,b)=>a[0]-b[0]);
  h+=`<h4>${c.n.toUpperCase()} ${L}${mc?' (classe principale)':''}<span class="src">${c.src}</span></h4>`+fl.map(x=>fiRow(x[0]+'°',x[1],x[2],'Privilegio di classe · '+c.n+' '+x[0]+'° livello')).join('');
  if(st.tOpt&&c.topt)h+='<div class="hint" style="font-size:10.5px;color:var(--mut)">✦ privilegio opzionale di supplemento (con il permesso del DM)</div>';}
 if(s){const sf=parseF(s.f).map((x,i)=>[x[0],x[1],featDesc(st.cls,s.n,x[1],i)]).filter(x=>x[0]<=L);
  if(st.style2){const sd=[...STYLES,...STYLES_T].find(x=>x[0]===st.style2);sf.push([10,'Stile: '+st.style2,sd?sd[1]+'.':'']);}
  if(SX().drac&&st.sAnc){const d=DRAGONS.find(x=>x[0]===st.sAnc);sf.unshift([1,'Antenato: drago '+st.sAnc,d?'Il tuo antenato è un drago '+st.sAnc.toLowerCase()+': il tipo di danno associato è '+d[1]+'.':'']);}
  h+=`<h4>${esc(s.n.toUpperCase())}<span class="src">${s.src}</span></h4>`+sf.map(x=>fiRow(x[0]+'°',x[1],x[2],(C().subT)+' · '+s.n+' '+x[0]+'° livello')).join('');}
 h+=privilegiClassiAggiunte();
 if(b)h+=`<h4>BACKGROUND: ${esc(b.n.toUpperCase())}</h4>`+fiRow(null,'Privilegio: '+b.feat,FD_BG[b.feat]||'','Privilegio di background · '+b.n);
 const F=feats();if(F.length)h+='<h4>TALENTI</h4>'+F.map(f=>fiRow(f.race?'razza':f.l+'°',f.n+(f.a?' (+1 '+ABS[f.a]+')':''),FEAT_D[f.n.toLowerCase()]||'Talento scelto dal giocatore: consulta il manuale per il testo completo.','Talento')).join('');
 $('oFeat').innerHTML=h||'<span style="color:var(--mut)">I privilegi compariranno qui dopo aver scelto classe, razza e background.</span>';
}
function openFeat(i){const f=FI[i];if(!f)return;$('fiT').textContent=f.t.replace(' ✦','');$('fiS').textContent=f.sub||'';
 $('fiD').innerHTML=(f.d||'Nessuna spiegazione disponibile per questa voce.').split('\n\n').map(p=>`<p>${esc(p)}</p>`).join('');$('mFi').classList.add('on');
 testoManualeRegola(f.t);}
/* Regole dai manuali (2026-11-11, «📥 Importa dai manuali», M3.4): al Master, sotto la sintesi di Gaster Bless, il testo del manuale
 se l'ha importato (talenti, privilegi, tratti, background, condizioni). Ai giocatori niente: il server lo dà solo al Master. */
function testoManualeRegola(t){if(!(typeof HOST!=='undefined'&&HOST.info&&HOST.info.role==='master'))return;
 const nome=String(t||'').replace(/^(Privilegio|Stile|Antenato)\s*:\s*/i,'').replace(/\s*\(.*?\)\s*/g,' ').replace(/✦/g,'').trim(),id=++testoManualeRegola.n;if(!nome)return;
 /* l'elenco delle regole importate si chiede una volta: niente richieste per le voci che non ci sono */
 if(!testoManualeRegola.elenco)testoManualeRegola.elenco=fetch('/api/v1/elementi-manuali?tipo=regola',{credentials:'same-origin'}).then(r=>r.ok?r.json():[]).then(l=>new Set((Array.isArray(l)?l:[]).map(x=>_fnorm(x.nome)))).catch(()=>new Set());
 testoManualeRegola.elenco.then(S=>S.has(_fnorm(nome))?fetch('/api/v1/elementi-manuali/testo?tipo=regola&nome='+encodeURIComponent(nome),{credentials:'same-origin'}).then(r=>r.ok?r.json():null):null).then(x=>{
  if(!x||!x.testo||id!==testoManualeRegola.n||!$('mFi').classList.contains('on'))return;
  const d=document.createElement('details');d.className='sdman';
  d.innerHTML='<summary>📖 Testo del manuale (solo tu) · '+esc(x.manualeNome||'')+(x.pagina?', p. '+esc(x.pagina):'')+'</summary><p>'+esc(x.testo)+'</p>';$('fiD').appendChild(d);}).catch(()=>{});}
testoManualeRegola.n=0;
document.addEventListener('click',e=>{const t=e.target.closest('[data-fi]');if(t)openFeat(+t.dataset.fi);});
$('fiClose').onclick=()=>$('mFi').classList.remove('on');

/* ===== RITRATTO NELLA PAGINA 1 ===== */
const _setImg=setImg;
setImg=function(id,src){_setImg(id,src);if(id==='portrait'){_setImg('portrait1',src);}};
readImg($('fImg1'),'img','portrait');
$('bImgDel').onclick=e=>{e.preventDefault();st.img='';setImg('portrait','');save();};

/* ===== BARRA DI RIEPILOGO ===== */
function renderQuick(){const c=C();const hp=hpCalc();const cs=caster();
 const items=[['CA',$('oAC').textContent],['PF',(st.hpCur||(hp?hp.max:'—'))+(hp?' / '+hp.max:'')],['Iniz.',$('oInit').textContent],['Vel.',$('oSpd').textContent],['Perc. pass.',$('oPass').textContent],['Comp.',$('oProf').textContent]];
 if(cs&&cs.active)items.push(['CD inc.',cs.dc],['Att. inc.',sg(cs.atk)]);
 $('qName').textContent=st.f.nome||'Senza nome';$('qSub').textContent=[c?classiTxt():'',RD()?RD().n:''].filter(Boolean).join(' · ');
 $('qStats').innerHTML=items.map(x=>`<span><small>${x[0]}</small><b>${esc(x[1])}</b></span>`).join('');}
const _render=render;render=function(){_render();renderQuick();};
window.addEventListener('scroll',()=>{$('quick').classList.toggle('on',window.scrollY>260);},{passive:true});
$('quick').onclick=()=>window.scrollTo({top:0,behavior:'smooth'});

/* ===== BACKGROUND PRIVATO (giocatore + master) =====
 Due archivi possibili: il database della scheda su claude.ai, oppure l'app ospite
 (mappa di Gaster Bless) tramite postMessage — vedi INTEGRAZIONE.md */
const BG_MAX=15*1024*1024,CHUNK=180000;
let STORE=null,UID=null,IS_MASTER=false,myMeta=null,allBg=[];
function bgSet(html){$('bgBox').innerHTML=html;}
function fmtSize(n){return n>1048576?(n/1048576).toFixed(1).replace('.',',')+' MB':Math.round(n/1024)+' KB';}
function fmtDate(t){try{return new Date(t).toLocaleString('it-IT',{dateStyle:'short',timeStyle:'short'});}catch(e){return'';}}
function b64(buf){let s='';const u=new Uint8Array(buf);for(let i=0;i<u.length;i+=32768)s+=String.fromCharCode.apply(null,u.subarray(i,i+32768));return btoa(s);}
function unb64(s){const bin=atob(s);const u=new Uint8Array(bin.length);for(let i=0;i<bin.length;i++)u[i]=bin.charCodeAt(i);return u;}
/* archivio claude.ai */
function claudeStore(DB,USR){return{
 watchMine(cb,err){return DB.doc('bg/'+UID).onSnapshot(s=>cb(s.exists?s.data():null),err);},
 watchAll(cb,err){return DB.collection('bg').onSnapshot(s=>cb(s.docs.filter(d=>d.exists).map(d=>({id:d.id,...d.data()}))),err);},
 async put(meta,data){const n=Math.ceil(data.length/CHUNK);const old=myMeta?myMeta.n:0;
  for(let i=0;i<n;i++){bgProg(`Caricamento ${Math.round((i+1)/n*100)}%…`);await DB.doc(`bg/${UID}/parts/p${i}`).set({d:data.slice(i*CHUNK,(i+1)*CHUNK)});}
  await DB.doc('bg/'+UID).set({...meta,n});for(let i=n;i<old;i++){try{await DB.doc(`bg/${UID}/parts/p${i}`).delete();}catch(e){}}},
 async del(){const n=myMeta.n;await DB.doc('bg/'+UID).delete();for(let i=0;i<n;i++)await DB.doc(`bg/${UID}/parts/p${i}`).delete();},
 async get(uid,meta){let s='';for(let i=0;i<meta.n;i++){const d=await DB.doc(`bg/${uid}/parts/p${i}`).get();if(!d.exists)throw new Error('parte mancante');s+=d.data().d;}return s;},
 async names(ids){try{const p=await USR.profiles(ids);const o={};ids.forEach(i=>o[i]=p[i]&&p[i].name);return o;}catch(e){return{};}}};}
/* archivio dell'app ospite */
function hostStore(){let mineCb=null,allCb=null;
 const refresh=async()=>{try{if(mineCb)mineCb((await HOST.request('bg.get',{playerId:UID,metaOnly:true})).meta||null);if(allCb)allCb(await HOST.request('bg.list',{}));}catch(e){}};
 return{watchMine(cb,err){mineCb=cb;refresh().catch(err);return()=>{};},watchAll(cb,err){allCb=cb;refresh().catch(err);return()=>{};},
  async put(meta,data){bgProg('Caricamento…');await HOST.request('bg.put',{playerId:UID,meta,data});await refresh();},
  async del(){await HOST.request('bg.delete',{playerId:UID});await refresh();},
  async get(uid){return(await HOST.request('bg.get',{playerId:uid})).data;},
  async names(ids){const o={};allBg.forEach(b=>o[b.id]=b.player);return o;}};}
function bgProg(t){const p=$('bgProg');if(p)p.innerHTML=t;}
async function initBg(){
 bgSet('<div class="hint">Connessione all\'archivio privato…</div>');
 if(HOST.on){const ok=await HOST.ready;if(ok&&HOST.info.playerId){UID=String(HOST.info.playerId);IS_MASTER=HOST.info.role==='master';renderTools();if(typeof renderMag==='function')renderMag();STORE=hostStore();startBg();return;}}
 let DB=null,USR=null;
 try{[DB,USR]=await Promise.all([window.claude?claude.use('db'):null,window.claude?claude.use('user'):null]);}catch(e){DB=null;}
 if(!DB||!USR){bgSet('<div class="warn">Il caricamento del background è disponibile nella scheda pubblicata su claude.ai o all\'interno dell\'app della campagna.</div>');return;}
 try{UID=await USR.id();}catch(e){UID=null;}
 IS_MASTER=!!(USR.isOwner&&USR.isOwner());
 if(!UID){bgSet('<div class="warn">Accedi per caricare il tuo background.</div>');return;}
 STORE=claudeStore(DB,USR);startBg();}
function startBg(){let done=false;const t=setTimeout(()=>{if(!done)bgSet('<div class="warn">L\'archivio non risponde ancora: riprova tra poco.</div>');},10000);
 STORE.watchMine(m=>{done=true;clearTimeout(t);myMeta=m;renderMine();},e=>{done=true;clearTimeout(t);bgSet('<div class="warn">Impossibile leggere l\'archivio.</div>');});
 if(IS_MASTER){$('bgMaster').style.display='';STORE.watchAll(l=>{allBg=l||[];renderAll();},()=>{$('bgList').innerHTML='<div class="warn">Impossibile leggere i background dei giocatori.</div>';});}}
function renderMine(){
 let h='<div class="bgpriv">🔒 Visibile solo a te e al Master.</div>';
 if(myMeta)h+=`<div class="bgfile"><span class="bgic">${/pdf/.test(myMeta.type)?'PDF':'DOC'}</span><div><b>${esc(myMeta.name)}</b><small>${fmtSize(myMeta.size)} · caricato il ${fmtDate(myMeta.at)}</small></div><button class="btn" data-bgopen="${esc(UID)}">Apri</button><button class="btn" id="bgDel">Elimina</button></div>`;
 h+=`<label class="btn pri bgup">${myMeta?'Sostituisci file':'Carica PDF o Word'}<input type="file" id="bgFile" accept=".pdf,.docx,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document" hidden></label><div class="hint">Formati: PDF o Word (.docx), massimo 15 MB. I vecchi file .doc vanno salvati come .docx.</div><div id="bgProg" class="hint"></div>`;
 bgSet(h);$('bgFile').onchange=e=>uploadBg(e.target.files[0]);if(myMeta)$('bgDel').onclick=deleteBg;}
async function uploadBg(f){if(!f)return;const isPdf=/pdf$/i.test(f.name)||f.type==='application/pdf';const isDocx=/\.docx$/i.test(f.name);
 if(!isPdf&&!isDocx){toast('Formato non supportato: usa PDF o .docx');return;}
 if(f.size>BG_MAX){toast('File troppo grande (massimo 15 MB)');return;}
 bgProg('Caricamento in corso…');
 try{const data=b64(await f.arrayBuffer());
  await STORE.put({name:f.name,type:isPdf?'application/pdf':'docx',size:f.size,at:Date.now(),char:st.f.nome||'',player:st.f.giocatore||'',cls:C()?classiTxt():''},data);toast('Background caricato');}
 catch(e){bgProg('<span class="warn">Caricamento non riuscito'+(e&&e.code==='quota_exceeded'?': spazio dell\'archivio esaurito':e&&e.code==='invalid_argument'?': non hai i permessi di scrittura (chiedi al Master di condividerti la scheda come Collaboratore)':'')+'.</span>');}}
async function deleteBg(){if(!myMeta)return;try{await STORE.del();toast('Background eliminato');}catch(e){toast('Eliminazione non riuscita');}}
async function renderAll(){if(!allBg.length){$('bgList').innerHTML='<div class="hint">Nessun giocatore ha ancora caricato un background.</div>';return;}
 const names=await STORE.names(allBg.map(b=>b.id));allBg.sort((a,b)=>(b.at||0)-(a.at||0));
 $('bgList').innerHTML=allBg.map(b=>`<div class="bgfile"><span class="bgic">${/pdf/.test(b.type)?'PDF':'DOC'}</span><div><b>${esc(b.char||'Personaggio senza nome')}</b> <small>${esc(b.cls||'')}</small><small>${esc(names[b.id]||b.player||'Giocatore')} · ${esc(b.name)} · ${fmtDate(b.at)}</small></div><button class="btn" data-bgopen="${esc(b.id)}">Apri</button></div>`).join('');}
function loadScript(src){return new Promise((res,rej)=>{if(document.querySelector(`script[src="${src}"]`))return res();const s=document.createElement('script');s.src=src;s.onload=res;s.onerror=rej;document.head.appendChild(s);});}
const PDFJS='/vendor/pdfjs/';
async function openBg(uid){const meta=uid===UID?myMeta:allBg.find(b=>b.id===uid);if(!meta)return;
 $('bvT').textContent=meta.char||meta.name;$('bvS').textContent=meta.name+' · '+fmtSize(meta.size);const body=$('bvB');body.innerHTML='<div class="hint">Apertura del documento…</div>';$('mBv').classList.add('on');
 try{const bytes=unb64(await STORE.get(uid,meta));
  if(/pdf/.test(meta.type)){await loadScript(PDFJS+'pdf.worker.min.js');await loadScript(PDFJS+'pdf.min.js');
   const doc=await window.pdfjsLib.getDocument({data:bytes,isEvalSupported:false}).promise;body.innerHTML='';
   for(let p=1;p<=doc.numPages;p++){const pg=await doc.getPage(p);const w=Math.min(body.clientWidth-8,900);const vp0=pg.getViewport({scale:1});const vp=pg.getViewport({scale:(w/vp0.width)*(window.devicePixelRatio||1)});
    const cv=document.createElement('canvas');cv.width=vp.width;cv.height=vp.height;cv.style.width=w+'px';cv.className='bvpage';body.appendChild(cv);await pg.render({canvasContext:cv.getContext('2d'),viewport:vp}).promise;}}
  else{await loadScript('/vendor/mammoth/mammoth.browser.min.js');await loadScript('/vendor/dompurify/purify.min.js');
   const r=await window.mammoth.convertToHtml({arrayBuffer:bytes.buffer});const div=document.createElement('div');div.className='bvdoc';
   div.innerHTML=window.DOMPurify.sanitize(r.value,{FORBID_TAGS:['style','form','input','iframe'],FORBID_ATTR:['style']});div.querySelectorAll('a').forEach(a=>a.removeAttribute('href'));body.innerHTML='';body.appendChild(div);}}
 catch(e){body.innerHTML='<div class="warn">Impossibile aprire il documento.</div>';}}
document.addEventListener('click',e=>{const t=e.target.closest('[data-bgopen]');if(t)openBg(t.dataset.bgopen);});
$('bvClose').onclick=()=>$('mBv').classList.remove('on');
/* ===== INTEGRAZIONE CON L'APP OSPITE (mappa di Gaster Bless / tracker iniziativa) =====
 La scheda può essere incorporata in un <iframe>. Il protocollo è descritto in INTEGRAZIONE.md. */
const HOST=(()=>{
 const qs=new URLSearchParams(location.search);
 let origin=qs.get('host')||'';
 if(!origin&&window.parent!==window&&document.referrer){try{origin=new URL(document.referrer).origin;}catch(e){}}
 const on=window.parent!==window&&!!origin&&!window.claude&&!/claude\.ai|claudeusercontent/.test(origin);
 const pend={};let n=0,readyRes;
 const H={on,origin,info:{},ready:new Promise(r=>readyRes=r),
  send(msg){if(on)window.parent.postMessage(msg,origin);},
  request(action,payload){return new Promise((res,rej)=>{if(!on)return rej(new Error('nessun ospite'));const id='r'+(++n);pend[id]={res,rej};
   H.send({type:'scheda:request',id,action,payload});setTimeout(()=>{if(pend[id]){delete pend[id];rej(new Error('timeout'));}},20000);});}};
 if(on){
  window.addEventListener('message',e=>{if(e.source!==window.parent||e.origin!==origin)return;const m=e.data||{};
   switch(m.type){
    case'scheda:init':H.info=m;IS_MASTER=m.role==='master'; /* subito: il primo disegno (oggetti magici, strumenti) deve già sapere chi è il Master (2026-11-11) */if(m.theme)setTheme(m.theme,true);if(m.incantesimi)applicaIncantesimiImportati(m.incantesimi);if(m.state)loadState(m.state);readyRes(true);break;
    case'scheda:load':if(m.state)loadState(m.state);break;
    case'scheda:theme':setTheme(m.theme,true);break;
    case'scheda:effetti':EFF=m.effetti&&typeof m.effetti==='object'?m.effetti:null;render();break; /* effetti in corso nello scontro (Gaster Bless) */
    case'scheda:setExh':st.exh=Math.max(0,Math.min(6,+m.livello||0));showExh();break;
    case'scheda:setHp':if(m.cur!=null){st.hpCur=String(m.cur);$('iHpCur').value=st.hpCur;}if(m.temp!=null){st.hpTmp=String(m.temp);$('iHpTmp').value=st.hpTmp;}render();break;
    case'scheda:print':doPrint();break;
    case'scheda:tiroRisultato':{const r=m.r||{};const d0=r.dadi&&r.dadi[0]||{};const v=(d0.valori||[])[0];const sc=d0.scarto&&d0.scarto.length?', scartato '+d0.scarto.join(', '):'';
     let esito='';if(tlUltimo&&tlUltimo.cd&&String(r.etichetta||'').includes(tlUltimo.op.slice(0,20))){esito=r.totale>=tlUltimo.cd?' · RIUSCITO ✔':' · FALLITO ✖';tlUltimo=null;}
     toast(`🎲 ${r.etichetta||''}: d20 (${v}${sc}) ${sg(r.mod||0)} = ${r.totale}${v===20?' · 20 naturale!':v===1?' · 1 naturale':''}${esito}`,6000);break;}
    case'scheda:response':{const p=pend[m.id];if(p){delete pend[m.id];m.ok?p.res(m.data):p.rej(new Error(m.error||'errore'));}break;}
   }});
  setTimeout(()=>readyRes(false),8000);
 }else readyRes(false);
 return H;})();
/* Gaster Bless (Fase 1, S13): i dati in arrivo si controllano prima di usarli. Si tengono solo le chiavi
 previste da blank() e con un tipo compatibile; il ritratto deve essere un'immagine. */
function validaStato(x){const b=blank(),o={};if(!x||typeof x!=='object'||Array.isArray(x))throw new Error('scheda non valida');
 const prim=v=>typeof v==='string'||typeof v==='number';
 for(const k of Object.keys(b)){if(!(k in x))continue;const v=x[k],d=b[k];
  if(Array.isArray(d)){if(Array.isArray(v))o[k]=v.filter(e=>e&&typeof e==='object'&&!Array.isArray(e)||prim(e)).slice(0,200);}
  else if(d&&typeof d==='object'){if(v&&typeof v==='object'&&!Array.isArray(v))o[k]=v;}
  else if(typeof d==='boolean'){if(typeof v==='boolean')o[k]=v;}
  else if(prim(d)&&prim(v))o[k]=v;}

 if(Array.isArray(o.mag))o.mag=OggettiMagici.sanificaVoci(o.mag); /* oggetti magici: ogni campo controllato, mai trattato come istruzione */
 if(typeof o.img==='string'&&o.img&&!/^data:image\/(jpeg|png|webp);base64,[A-Za-z0-9+/=]+$/.test(o.img))o.img='';
 return o;}
function loadState(s){try{st=Object.assign(blank(),validaStato(typeof s==='string'?JSON.parse(s):s));
 /* scheda bloccata (modifica manuale): i dati validi appena arrivati diventano subito il punto fermo, prima del disegno; se no la
    guardia del blocco li prenderebbe per una modifica e rimetterebbe la bozza (2026-11-12) */
 try{if(BLOCCO)approvatoSt=clona(st);}catch(e){/* blocco non ancora pronto all'avvio */}
 boot();}catch(e){toast('Dati della scheda non validi');}}
/* riepilogo per tracker iniziativa e mappa */
function summary(){const A=abil(),c=C(),r=RD(),hp=hpCalc(),cs=caster(),ac=acCalc(),ini=initCalc(),spd=speedCalc();
 return{name:st.f.nome||'',player:st.f.giocatore||'',class:c?(multi()?classiTxt():c.n):'',subclass:subOn()?S().n:'',level:LT(),race:r?r.n:'',
  ac:ac.ac,hpMax:hp?hp.max:0,hpCur:st.hpCur!==''&&st.hpCur!=null?+st.hpCur:(hp?hp.max:0),hpTemp:+st.hpTmp||0,
  initiative:ini.v,dex:A.T.DES,speed:spd.s,passivePerception:+$('oPass').textContent||10,
  spellDC:cs&&cs.active?cs.dc:null,spellAttack:cs&&cs.active?cs.atk:null,
  abilities:A.T,mods:A.M,proficiency:prof(),deathSaves:st.ds,portrait:st.img||'',
  saveProfs:[...saveProfs()],
  /* Gaster Bless, professioni: strumenti pronti (competenza + strumento nell'equipaggiamento) e abilità con competenza */
  tools:TLS().filter(toolPronto).map(T=>T.id),skillProfs:SKILLS.filter(([n])=>skProf(n)).map(([n])=>n),
  /* Gaster Bless, sistema di combattimento (Fase A, 2026-11-11): profili d'attacco calcolati dalla scheda, attacchi per azione
   (Attacco Extra) e difese date dagli oggetti magici. Li userà il motore degli attacchi del tracker (Fase B). */
  attacks:PROFILI_ATTACCO.slice(0,30),attacksPerAction:+((resources().find(x=>x[0]==='Attacchi per azione')||[0,1])[1])||1,
  /* riposi (2026-11-11): Dadi Vita per tipo («5d10 + 2d6»), modificatore di Costituzione, slot della Magia del Patto */
  hitDice:(()=>{const m={};String((hp&&hp.dadi)||'').split('+').forEach(x=>{const r=/(\d+)d(\d+)/.exec(x);if(r)m[r[2]]=(m[r[2]]||0)+(+r[1]);});return Object.entries(m).map(([f,n])=>({f:+f,n}));})(),
  conMod:A.M.COS,pactSlots:cs&&cs.pact?{lv:cs.pact.lv,n:cs.pact.n}:null,
  /* difese complete (2026-11-12): razza, classe, talenti e oggetti; TS calcolati; immunità alle condizioni; vantaggi ai TS */
  ...(()=>{const D=difesePG(),o=difeseOggetti();if(!D)return{defenses:o};
   const ira=DifesePG.ira?DifesePG.ira(difeseCtx()):null; /* ira del barbaro: tasto «😡 Ira» nel tracker (effetti_valori.go) */
   return{defenses:DifesePG.perRiepilogo(D,o),saves:tsTotali(),condImm:D.immCond.map(x=>x.c),saveAdv:D.vant.map(x=>x.c),magicRes:D.magia.car,...(ira?{rage:ira}:{})};})(),
  /* incantesimi automatici e slot per lanciarli senza la scheda (gioco da remoto, scheda/incantesimi_ui.js, 2026-11-12) */
  ...(()=>{try{return typeof window.incantesimiAutomatici==='function'?window.incantesimiAutomatici():{};}catch(e){return{};}})()};}
let hostT=null;
function notifyHost(){if(!HOST.on)return;clearTimeout(hostT);hostT=setTimeout(()=>{HOST.send({type:'scheda:state',state:st,summary:summary()});},700);}
const _save=save;save=function(){_save();notifyHost();};
window.SchedaAPI={getState:()=>JSON.parse(JSON.stringify(st)),setState:loadState,getSummary:summary,setTheme:t=>setTheme(t),print:()=>doPrint(),
 setHp:(cur,temp)=>{if(cur!=null)st.hpCur=String(cur);if(temp!=null)st.hpTmp=String(temp);initStatic();render();}};

/* ===== TEMA CHIARO / SCURO ===== */
const TKEY='dnd5e-scheda-tema';
function setTheme(t,fromHost){if(t!=='light'&&t!=='dark')return;document.documentElement.dataset.theme=t;$('bTheme').textContent=t==='light'?'☾':'☀';$('bTheme').title=t==='light'?'Passa al tema scuro':'Passa al tema chiaro';
 if(!fromHost){try{localStorage.setItem(TKEY,t);}catch(e){}HOST.send({type:'scheda:theme',theme:t});}}
(()=>{let t='dark';try{t=localStorage.getItem(TKEY)||t;}catch(e){}setTheme(t,true);})();
$('bTheme').onclick=()=>setTheme(document.documentElement.dataset.theme==='light'?'dark':'light');

/* ===== STAMPA ===== */
function txtToDiv(on){document.querySelectorAll('.ptxt').forEach(d=>d.remove());if(!on)return;
 document.querySelectorAll('.page textarea').forEach(t=>{const d=document.createElement('div');d.className='ptxt';d.textContent=t.value||' ';d.style.minHeight=Math.min(t.offsetHeight||40,400)+'px';t.after(d);});}
window.addEventListener('beforeprint',()=>txtToDiv(true));window.addEventListener('afterprint',()=>txtToDiv(false));
async function doPrint(){
 if(!window.claude){txtToDiv(true);setTimeout(()=>{try{window.print();}catch(e){}},50);return;}
 /* in claude.ai la stampa diretta è bloccata: si crea un PDF da stampare */
 $('mPr').classList.add('on');}
async function makePdf(){const btn=$('prPdf');btn.disabled=true;$('prMsg').textContent='Preparazione del PDF…';
 try{await loadScript('/vendor/html2canvas/html2canvas.min.js');await loadScript('/vendor/jspdf/jspdf.umd.min.js');
  const {jsPDF}=window.jspdf;const pdf=new jsPDF({unit:'mm',format:'a4',orientation:'portrait'});
  const pages=['p1','p2','p3'].filter(id=>$('pr_'+id).checked);if(!pages.length)throw new Error('nessuna pagina');
  const root=document.documentElement;const prevTheme=root.dataset.theme;root.classList.add('printing');root.dataset.theme='light';txtToDiv(true);
  const wrap=document.querySelector('.wrap');const oldW=wrap.style.width;wrap.style.width='1120px';
  for(let i=0;i<pages.length;i++){const pg=$(pages[i]);const was=pg.classList.contains('on');pg.classList.add('on');document.querySelectorAll('.page').forEach(p=>{if(p!==pg)p.style.display='none';});
   $('prMsg').textContent=`Pagina ${i+1} di ${pages.length}…`;
   const cv=await html2canvas(pg,{scale:2,backgroundColor:'#ffffff',useCORS:true,logging:false,windowWidth:1180});
   document.querySelectorAll('.page').forEach(p=>p.style.display='');if(!was)pg.classList.remove('on');
   const W=200,H=287;const r=Math.min(W/cv.width,H/cv.height);const w=cv.width*r,h=cv.height*r;
   if(i)pdf.addPage();pdf.addImage(cv.toDataURL('image/jpeg',.92),'JPEG',(210-w)/2,5,w,h);}
  wrap.style.width=oldW;root.classList.remove('printing');root.dataset.theme=prevTheme;txtToDiv(false);
  const blob=pdf.output('blob');const name=((st.f.nome||'personaggio').replace(/[^\w\- àèéìòù]/gi,'').trim()||'personaggio')+' - scheda.pdf';
  const dl=window.claude?await claude.use('downloads'):null;
  if(dl){await dl.save({filename:name,data:blob});$('prMsg').textContent='PDF salvato: aprilo e stampalo.';}
  else{const a=document.createElement('a');a.href=URL.createObjectURL(blob);a.download=name;document.body.appendChild(a);a.click();a.remove();$('prMsg').textContent='PDF scaricato: aprilo e stampalo.';}}
 catch(e){document.documentElement.classList.remove('printing');txtToDiv(false);document.querySelectorAll('.page').forEach(p=>p.style.display='');
  $('prMsg').textContent=e&&e.code==='declined'?'Salvataggio annullato.':'Non è stato possibile creare il PDF.';}
 btn.disabled=false;}
function printNow(){$('mPr').classList.remove('on');txtToDiv(true);setTimeout(()=>{try{window.print();}catch(e){}},80);}
$('bPrint').onclick=()=>{$('prNow').style.display=window.claude?'none':'';$('prNote').style.display=window.claude?'':'none';$('prMsg').textContent='';$('mPr').classList.add('on');};$('prNow').onclick=printNow;$('prPdf').onclick=makePdf;$('prClose').onclick=()=>$('mPr').classList.remove('on');

/* ===== MODIFICA MANUALE (Gaster Bless, richiesta dell'utente 2026-11-11) =====
 Con l'approvazione del Master attiva, l'app manda {type:'scheda:blocco', bloccata:true} finché il giocatore non accende
 «✏ Modifica manuale» (tasto nella barra della pagina, web/sezioni/schede.js). Da bloccata si cambiano solo i dati di
 gioco: PF, PF temporanei, tiri contro la morte, slot, ispirazione, indebolimento, incantesimi preparati, monete (che
 diventano una richiesta al Master) e stato degli oggetti magici (equipaggiato, sintonia, attivato, cariche).
 I campi di testo e le scelte degli altri dati sono disattivati; qualsiasi altra modifica (pulsanti, finestre,
 importazione) viene annullata dalla guardia in save(). Le stesse regole le applicano l'app e il server
 (internal/server/modifiche_manuali.go): qui servono solo a dare subito la risposta giusta al giocatore. */
let BLOCCO=false,approvatoSt=null,bloccoAvv=0,bloccoRipristino=false;
const GIOCO_TESTE=new Set(['hpCur','hpTmp','ds','slotU','insp','exh','v','eqAuto','san','spp','coins','armor','shield','resRip']); /* armatura e scudo indossati = equipaggiamento (2026-11-11) */
const GIOCO_SEL='[data-f="hdUsati"],#iHpCur,#iHpTmp,[data-ds],#cInsp,#sExh,[data-spp],[data-coin],[data-mg],#bgFile,#cArm,#cShield';
const clona=o=>JSON.parse(JSON.stringify(o));
const stabileJ=v=>Array.isArray(v)?'['+v.map(stabileJ).join(',')+']':v&&typeof v==='object'?'{'+Object.keys(v).sort().map(k=>JSON.stringify(k)+':'+stabileJ(v[k])).join(',')+'}':JSON.stringify(v===undefined?null:v);
function piattoSt(o){const out={};for(const [k,v] of Object.entries(o||{})){if(v&&typeof v==='object'&&!Array.isArray(v)&&Object.keys(v).length){for(const [k2,v2] of Object.entries(v))out[k+'.'+k2]=stabileJ(v2);}else out[k]=stabileJ(v);}return out;}
/* stessi oggetti magici, cambiano solo equipaggiato/sintonia/attivato/cariche */
/* elenco degli oggetti: stessi oggetti, cambia solo «equipaggiato» */
function invSoloStatoSt(a,b){a=Array.isArray(a)?a:[];b=Array.isArray(b)?b:[];if(a.length!==b.length)return false;const d=v=>{const c={...v};delete c.eq;return stabileJ(c);};return a.every((v,i)=>d(v)===d(b[i]));}
function magSoloStatoSt(a,b){a=Array.isArray(a)?a:[];b=Array.isArray(b)?b:[];if(a.length!==b.length)return false;
 const dati=v=>{const c={...v};delete c.eq;delete c.sint;delete c.on;delete c.car;return stabileJ(c);};return a.every((v,i)=>dati(v)===dati(b[i]));}
function campiBloccatiCambiati(){if(!approvatoSt)return[];const A=piattoSt(approvatoSt),N=piattoSt(st),out=[];
 for(const k of new Set([...Object.keys(A),...Object.keys(N)])){if(A[k]===N[k])continue;const t=k.split('.')[0];if(GIOCO_TESTE.has(t)||k==='f.hdUsati')continue;if(k==='mag'&&magSoloStatoSt(approvatoSt.mag,st.mag))continue;if(k==='inv'&&invSoloStatoSt(approvatoSt.inv,st.inv))continue;out.push(k);}
 return out;}
/* true = la modifica appena fatta tocca dati bloccati: si annulla (subito dopo il disegno in corso) e non si salva */
function bloccoGuardia(){if(!BLOCCO||!approvatoSt||bloccoRipristino)return false;
 if(!campiBloccatiCambiati().length){approvatoSt=clona(st);return false;} /* solo dati di gioco: diventano il nuovo punto fermo */
 bloccoRipristino=true;
 setTimeout(()=>{const n=clona(approvatoSt);GIOCO_TESTE.forEach(t=>{if(t in st)n[t]=clona(st[t]);});if(st.f&&n.f&&st.f.hdUsati!=null)n.f.hdUsati=st.f.hdUsati;
  if(magSoloStatoSt(approvatoSt.mag,st.mag))n.mag=clona(st.mag);if(invSoloStatoSt(approvatoSt.inv,st.inv))n.inv=clona(st.inv);
  st=Object.assign(blank(),n);bloccoRipristino=false;boot();approvatoSt=clona(st);applicaBlocco();
  if(Date.now()-bloccoAvv>3000){bloccoAvv=Date.now();toast('🔒 Per cambiare questo dato attiva «✏ Modifica manuale» in alto: le modifiche andranno al Master.',5000);}},0);
 return true;}
const _saveBlocco=save;save=function(){if(bloccoGuardia())return;_saveBlocco();};
function applicaBlocco(){document.documentElement.classList.toggle('bloccata',BLOCCO);
 document.querySelectorAll('.page input,.page select,.page textarea,.page button[data-bl],#magBox input,#magBox button').forEach(e=>{
  const gioco=e.matches(GIOCO_SEL)||e.closest('.mgo-btns')&&/^(card|exp)$/.test(e.dataset.mgv||'')||e.matches('[data-mgc]');
  if(BLOCCO&&!gioco&&!e.disabled){e.disabled=true;e.dataset.bl='1';}
  else if((!BLOCCO||gioco)&&e.dataset.bl){e.disabled=false;delete e.dataset.bl;}});
 ['mgoNew','mgoImp'].forEach(id=>{const b=$(id);if(b)b.style.display=BLOCCO?'none':'';});}
let bloccoObs=null;
function impostaBlocco(on){BLOCCO=!!on;approvatoSt=BLOCCO?clona(st):null;applicaBlocco();
 if(BLOCCO&&!bloccoObs){let t=0;bloccoObs=new MutationObserver(()=>{if(t)return;t=requestAnimationFrame(()=>{t=0;if(BLOCCO)applicaBlocco();});});
  bloccoObs.observe(document.querySelector('.wrap')||document.body,{childList:true,subtree:true});}}
window.addEventListener('message',e=>{if(!HOST.on||e.source!==window.parent||e.origin!==HOST.origin)return;const m=e.data||{};
 if(m.type==='scheda:blocco')impostaBlocco(m.bloccata);
 else if(m.type==='scheda:load'&&BLOCCO)setTimeout(()=>{approvatoSt=clona(st);applicaBlocco();},0);});

HOST.send({type:'scheda:ready',version:1});
setTimeout(initBg,0);

/* ===== GASTER BLESS: MULTICLASSE ===== */
/* Classi aggiunte in st.mc = [{cls, sub, lvl, hp}]; i calcoli sono in /regole/motore.js. */
function mcLiberi(i){const usati=st.lvl+(st.mc||[]).reduce((a,m,j)=>a+(j===i?0:(+m.lvl||0)),0);return Math.max(0,20-usati);}
function pannelloMulticlasse(){if(!Array.isArray(st.mc))st.mc=[];
 const A=abil();let h='<div class="hint" style="margin-top:10px"><b>Multiclasse</b></div>';
 st.mc.forEach((m,i)=>{const presi=new Set([st.cls,...st.mc.filter((_,j)=>j!==i).map(x=>x.cls)]);const d=CLASSES[m.cls];const max=Math.max(1,mcLiberi(i));
  h+=`<div class="fr"><select data-mcc="${i}" aria-label="Classe aggiunta"><option value="">— Classe —</option>${Object.entries(CLASSES).filter(([k])=>!presi.has(k)||k===m.cls).map(([k,c])=>`<option value="${k}"${k===m.cls?' selected':''}>${esc(c.n)}</option>`).join('')}</select>`+
   `<select data-mcl="${i}" aria-label="Livelli" style="width:70px">${Array.from({length:max},(_,j)=>`<option value="${j+1}"${j+1===+m.lvl?' selected':''}>Liv. ${j+1}</option>`).join('')}</select>`+
   (d?`<select data-mcs="${i}" aria-label="${esc(d.subT)}"${+m.lvl<d.subL?' disabled':''}><option value="">— ${esc(d.subT)}${+m.lvl<d.subL?' (dal '+d.subL+'°)':''} —</option>${d.subs.map(s=>`<option value="${esc(s[0])}"${s[0]===m.sub?' selected':''}>${esc(s[0])}</option>`).join('')}</select>`:'')+
   `${d?`<button class="btn" data-mchp="${i}" title="Punti ferita di questa classe">PF…</button>`:''}<button class="btn" data-mcx="${i}" title="Togli questa classe">×</button></div>`;
  if(d){const cp=Regole5e.COMPETENZE_MC[m.cls];if(cp)h+=`<div class="hint">Entrando nella classe ottieni: ${esc(cp.testo)}.</div>`;}});
 Regole5e.avvisiMulticlasse(st,A.T).forEach(a=>h+=`<div class="warn">${esc(a)}</div>`);
 if(LT()<20)h+=`<div class="fr"><button class="btn" id="mcAdd">＋ Aggiungi una classe</button></div>`;
 if(st.mc.length)h+=`<div class="hint">Livello del personaggio: ${LT()} (${esc(classiTxt())}). Il bonus di competenza dipende dal livello del personaggio, i privilegi dal livello di ogni classe. I tiri salvezza sono solo quelli della classe principale.</div>`;
 return h;}
function mcVoce(i){return KL().find(k=>!k.primaria&&k.idx===i);}
document.addEventListener('change',e=>{const t=e.target;if(!t.dataset)return;
 if(t.dataset.mcc!=null){const i=+t.dataset.mcc;st.mc[i]={cls:t.value,sub:'',lvl:Math.min(+st.mc[i].lvl||1,Math.max(1,mcLiberi(i))),hp:{}};buildCfg();render();
  const k=mcVoce(i);if(k)askHp(range(1,k.lvl),'Hai aggiunto '+k.c.n.toLowerCase()+' al personaggio. ',k);return;}
 if(t.dataset.mcl!=null){const i=+t.dataset.mcl,m=st.mc[i];const old=+m.lvl||1;m.lvl=+t.value;if(!m.hp)m.hp={};Object.keys(m.hp).forEach(l=>{if(+l>m.lvl)delete m.hp[l];});
  buildCfg();render();const k=mcVoce(i);if(k&&m.lvl>old)askHp(range(old+1,m.lvl),'Sei salito al livello '+m.lvl+' da '+k.c.n.toLowerCase()+'. ',k);return;}
 if(t.dataset.mcs!=null){st.mc[+t.dataset.mcs].sub=t.value;buildCfg();render();return;}});
document.addEventListener('click',e=>{const t=e.target.closest('#mcAdd,[data-mcx],[data-mchp]');if(!t)return;
 if(t.id==='mcAdd'){if(!Array.isArray(st.mc))st.mc=[];if(LT()<20){st.mc.push({cls:'',sub:'',lvl:1,hp:{}});buildCfg();render();}return;}
 if(t.dataset.mcx!=null){st.mc.splice(+t.dataset.mcx,1);buildCfg();render();toast('Classe tolta');return;}
 if(t.dataset.mchp!=null){const k=mcVoce(+t.dataset.mchp);if(k){if(!st.mc[k.idx].hp)st.mc[k.idx].hp={};k.hp=st.mc[k.idx].hp;askHp(range(1,k.lvl),'',k);}}});

/* Aumento di caratteristica: al posto di "+2 ?" si segnala chiaramente che manca la scelta. */
function rigaAumento(l,key){const a=st.asi[key]||{m:'2'};let d,ok;
 if(a.m==='f'){d='Talento: '+(a.feat||'—');ok=!!a.feat;}
 else if(a.m==='11'){d='+1 '+(a.a?ABS[a.a]:'?')+', +1 '+(a.b?ABS[a.b]:'?');ok=!!(a.a&&a.b);}
 else{d='+2 '+(a.a?ABS[a.a]:'?');ok=!!a.a;}
 const base=FD_GEN['Aumento dei Punteggi di Caratteristica'];
 if(!ok)return[l,'⚠ '+ASI_NAME+': da scegliere',base+' Non hai ancora completato la scelta: apri «Scelte del personaggio», riquadro Classe, voce '+asiLabel(key)+'.'];
 return[l,ASI_NAME+' ('+d+')',base+' Scelta attuale: '+d+'.'];}

/* Privilegi delle classi aggiunte, ognuna con il proprio livello. */
function privilegiClassiAggiunte(){let h='';
 KL().filter(k=>!k.primaria).forEach(k=>{const c=k.c,L=k.lvl;
  const fl=parseF(c.f).filter(x=>x[0]<=L).map(x=>[x[0],x[1],featDesc(k.id,null,x[1])]);
  (c.asi||ASI_STD).filter(l=>l<=L).forEach(l=>fl.push(rigaAumento(l,k.id+':'+l)));
  if(st.tOpt&&c.topt)parseF(c.topt).filter(x=>x[0]<=L).forEach(x=>fl.push([x[0],x[1]+' ✦',featDesc(k.id,null,x[1])]));
  fl.sort((a,b)=>a[0]-b[0]);
  h+=`<h4>${esc(c.n.toUpperCase())} ${L}<span class="src">${c.src}</span></h4>`+fl.map(x=>fiRow(x[0]+'°',x[1],x[2],'Privilegio di classe · '+c.n+' '+x[0]+'° livello')).join('');
  if(k.subOn){const s=k.sub;const sf=parseF(s.f).map((x,i)=>[x[0],x[1],featDesc(k.id,s.n,x[1],i)]).filter(x=>x[0]<=L);
   h+=`<h4>${esc(s.n.toUpperCase())}<span class="src">${s.src}</span></h4>`+sf.map(x=>fiRow(x[0]+'°',x[1],x[2],c.subT+' · '+s.n+' '+x[0]+'° livello')).join('');}});
 return h;}

/* Incantesimi con il multiclasse: CD, attacco e conteggi per ogni classe. Gli incantesimi presi prima
   del multiclasse (senza "o") contano per la prima classe incantatrice. */
function infoMulticlasse(cs,info){const prima=cs.classi[0].id;
 cs.classi.forEach(x=>{let t0=0,n=0,np=0;for(let l=0;l<=9;l++)spList(l).forEach(y=>{if(y.o==='extra'||(y.o||prima)!==x.id)return;if(l===0)t0++;else{n++;if(y.p)np++;}});
  const parti=[`<b>${esc(x.nome)}</b> ${x.lvl}: CD ${x.cd}, attacco ${sg(x.atk)}`];
  if(x.trucchetti)parti.push(`trucchetti <b class="${t0>x.trucchetti?'bad':''}">${t0}/${x.trucchetti}</b>`);
  if(x.known!=null)parti.push(x.kLabel==='Incantesimi preparati'?`preparati <b class="${np>x.known?'bad':''}">${np}/${x.known}</b>`:`conosciuti <b class="${n>x.known?'bad':''}">${n}/${x.known}</b>`);
  if(!x.attivo)parti.push('incantesimi dal '+(x.tipo==='half'?'2°':'3°')+' livello');
  info.push(parti.join(', '));});
 if(cs.inc.multiclasse)info.push('Slot condivisi da incantatore multiclasse di '+cs.inc.livello+'° livello');}

/* Nel selettore degli incantesimi, con più classi incantatrici si sceglie per quale classe. */
function selettoreClassePicker(cs){let sel=$('pkCls');
 if(!sel){sel=document.createElement('select');sel.id='pkCls';sel.setAttribute('aria-label','Per quale classe');sel.style.margin='6px 0';$('pkSub').after(sel);
  sel.onchange=()=>{pickCls=sel.value;const code=spellCode();$('pkSub').textContent=code?'Lista del '+CODE_N[code].toLowerCase():'Tutte le liste: l\'incantesimo non conterà nei limiti di classe';renderPicker();};}
 /* "Bonus": trucchetti razziali (alto elfo, tiefling, drow...), incantesimi dei talenti e quelli sempre preparati della sottoclasse. */
 sel.innerHTML=(cs?cs.classi:[]).map(x=>`<option value="${x.id}"${x.id===pickCls?' selected':''}>Per: ${esc(x.nome)}</option>`).join('')+
  `<option value="extra"${pickCls==='extra'?' selected':''}>Bonus: razza, talento o sottoclasse (non conta nei limiti)</option>`;
 if(!cs)pickCls='extra';} /* chi non lancia incantesimi li ha solo come bonus */
