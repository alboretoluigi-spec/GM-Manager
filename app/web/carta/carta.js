/* Mappa di Gaster Bless: copia di 02_mappa_originale/estratto_da_exe/web/index.html (script), adattata alla nuova app.
   Modifiche elencate in web/carta/LEGGIMI.md. */
"use strict";
const W=1618,H=1048,N=W*H,HEX=22.15,$=s=>document.querySelector(s),NS="http://www.w3.org/2000/svg";
// HEX = distanza in pixel tra i centri di due esagoni adiacenti, misurata sulla mappa (legenda: 1 esagono = ¼ di miglio)
const PACE={2:"Andatura lenta: 18 miglia al giorno; il gruppo può muoversi furtivamente.",3:"Andatura normale: 24 miglia al giorno.",4:"Andatura veloce: 30 miglia al giorno; −5 alla Saggezza (Percezione) passiva."};
const img=$("#bg"),ov=$("#ov"),map=$("#map"),vp=$("#vp"),PJ=location.hash==="#proj";
let LOCS=[],wps=[],legs=[],zoom=1,miHex=.25,ME={},role=null,tool="route",painting=false,raf=0,saving=false,pend=false,es=null;
let CONTENT={},G=null,GROUP=null,PST=null,PROJ={kind:"none"},PVL=null;
const BP=window.GasterBlessProiettore; // regolazioni e disegno della proiezione pulita (proiettore.js)
const el=(t,a,p)=>{const e=document.createElementNS(NS,t);for(const k in a)e.setAttribute(k,a[k]);(p||ov).appendChild(e);return e};
const esc=s=>String(s==null?"":s).replace(/[&<>"']/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c]));
function toast(t,ms){if(PJ)return;/* proiezione pulita: niente avvisi sul proiettore */const e=$("#toast");e.textContent=t;e.style.display="block";clearTimeout(toast.t);toast.t=setTimeout(()=>e.style.display="none",ms||2800)}
const pxMi=px=>px/HEX*miHex;
/* Gaster Bless (nuova app): le chiamate /api/... vanno al nuovo server, con la sessione e il token anti-CSRF. */
const SID=new URLSearchParams(location.search).get("sessione")||"";let CONSULTA=false; /* giocatore fuori dalle sessioni («g:…»): mappa solo da guardare (2026-11-08) */
let CSRF="",LONTANO=false; // LONTANO: giocatore collegato da fuori casa (Internet o VPN)
const compat=p=>{const[a,q]=p.split("?");return"/api/v1/mappa/"+a.replace(/^\/api\//,"")+"?sessione="+encodeURIComponent(SID)+(q?"&"+q:"")};
async function api(path,body,method){path=compat(path);const o={method:method||(body!==undefined?"POST":"GET"),credentials:"same-origin",headers:{"X-CSRF":CSRF}};
  if(body!==undefined){o.headers["Content-Type"]="application/json";o.body=JSON.stringify(body)}
  const r=await fetch(path,o);let j=null;try{j=await r.json()}catch(e){}
  if(!r.ok){const e=new Error((j&&(j.errore||j.error))||("Errore "+r.status));e.status=r.status;if(r.status===401)lost();throw e}return j}

/* ---------- Nebbia (celle 20 px) ---------- */
const CW=20,CC=Math.ceil(W/CW),CR=Math.ceil(H/CW);let fog=new Uint8Array(CC*CR);
const cell=(x,y)=>fog[Math.min(CR-1,Math.max(0,(y/CW)|0))*CC+Math.min(CC-1,Math.max(0,(x/CW)|0))];
const pass=n=>role!=="player"||cell(n%W,(n/W)|0);
const pack=()=>{const b=new Uint8Array(Math.ceil(fog.length/8));fog.forEach((v,i)=>{if(v)b[i>>3]|=1<<(i&7)});let s="";b.forEach(v=>s+=String.fromCharCode(v));return btoa(s)};
const unpack=s=>{const f=new Uint8Array(CC*CR);try{const r=atob(s||"");for(let i=0;i<f.length;i++)f[i]=((r.charCodeAt(i>>3)||0)>>(i&7))&1}catch(e){}return f};

/* ---------- Strade e percorsi: calcolati sul PC (P1) ----------
   Prima ogni dispositivo scaricava la rete stradale (img/mask.png) e teneva in memoria circa 35 MB di
   tabelle per l'A*. Ora lo fa il server con lo stesso algoritmo (internal/server/mappa_strade.go) e qui
   arriva solo il tracciato. I giocatori ottengono solo strade fuori dalla nebbia, come prima. */
async function snap(x,y,R){try{return(await api("/api/strada",{op:"aggancia",x:Math.round(x),y:Math.round(y),r:R})).i}catch(e){toast(e.message);return -1}}
async function astar(a,b){try{const r=await api("/api/strada",{op:"percorso",a,b});return r.nessuno?null:{pts:r.pts,px:r.px,off:false}}catch(e){toast(e.message);return null}}
const pt=i=>[i%W,(i/W)|0];
const straight=(a,b)=>({pts:[a,b],px:Math.hypot(b[0]-a[0],b[1]-a[1]),off:true});
/* ---------- Terreno (2026-11-07): zone lette sulla mappa, in terreno.json ----------
   Sulle strade e sui sentieri si viaggia a velocità piena. Fuori strada (regole 2014) foresta, collina, montagna e
   acqua sono terreno difficile: velocità dimezzata; in pianura la velocità resta piena. Ogni tratto fuori strada si
   campiona ogni 3 pixel e si divide nei pezzi di terreno che attraversa. */
let TERR={predefinito:{tipo:"foresta",nome:"Selvascura Woods"},zone:[]};
const DIFF={foresta:true,collina:true,montagna:true,acqua:true,gelo:true};
/* Fattore di velocità per tipo di pezzo. Fuori strada: terreno difficile 1/2, pianura 1. Su strada (regola della casa,
   richiesta dell'utente 2026-11-07): collina 3/4, montagna 1/2, foresta e pianura 1. Tratto del Gelo 1/2. */
const FATT={strada:1,pianura:1,foresta:.5,collina:.5,montagna:.5,acqua:.5,gelo:.5,stradaCollina:.75,stradaMontagna:.5};
const fatt=t=>FATT[t]==null?1:FATT[t];
const NOMI_T={foresta:"foresta",collina:"collina",montagna:"montagna",acqua:"acqua",pianura:"pianura",gelo:"salita ghiacciata",strada:"strada",stradaCollina:"strada in collina",stradaMontagna:"strada in montagna"};
/* ---------- Monte Gelo (2026-11-07): tratto Pendici → Passo del Vento con regole speciali (internal/server/gelo.go) ----------
   Sul tratto anche la strada è terreno difficile (salita ripida, ghiaccio, neve): metà velocità. Dalla metà del tratto
   (quota ≥ 5.900 m) vale l'alta quota: ogni ora conta doppia per la marcia forzata. */
let GK=null,GKD=null;
function gkPrepara(){GKD=null;if(!GK||!GK.attivo||!GK.pts||GK.pts.length<2)return;const p=[],c=[];let tot=0;
  for(let i=0;i<GK.pts.length;i++){if(i){const[a,b]=[GK.pts[i-1],GK.pts[i]],L=Math.hypot(b[0]-a[0],b[1]-a[1]),n=Math.max(1,Math.ceil(L/2));
      for(let k=1;k<=n;k++){p.push([a[0]+(b[0]-a[0])*k/n,a[1]+(b[1]-a[1])*k/n]);tot+=L/n;c.push(tot)}}else{p.push(GK.pts[0]);c.push(0)}}
  const xs=p.map(q=>q[0]),ys=p.map(q=>q[1]),r=GK.regole.corridoio;GKD={p,c,tot,r,bb:[Math.min(...xs)-r,Math.min(...ys)-r,Math.max(...xs)+r,Math.max(...ys)+r]}}
/* frazione del tratto (0 = Pendici, 1 = Passo) se il punto è sulla strada del tratto, altrimenti -1 */
function gkF(x,y){const g=GKD;if(!g||x<g.bb[0]||y<g.bb[1]||x>g.bb[2]||y>g.bb[3])return -1;let b=1e9,bi=0;
  for(let i=0;i<g.p.length;i++){const d=(g.p[i][0]-x)**2+(g.p[i][1]-y)**2;if(d<b){b=d;bi=i}}return Math.sqrt(b)<=g.r?g.c[bi]/g.tot:-1}
const gkQuota=f=>GK.regole.quotaBase+f*(GK.regole.quotaPasso-GK.regole.quotaBase);
const m0=x=>String(Math.round(x)).replace(/\B(?=(\d{3})+(?!\d))/g,"."); // 5.900
function dentro(x,y,P){let c=false;for(let i=0,j=P.length-1;i<P.length;j=i++){const[xi,yi]=P[i],[xj,yj]=P[j];
  if((yi>y)!==(yj>y)&&x<(xj-xi)*(y-yi)/(yj-yi)+xi)c=!c}return c}
function terrenoIn(x,y){for(let i=TERR.zone.length-1;i>=0;i--){const z=TERR.zone[i];if(dentro(x,y,z.punti))return z}return TERR.predefinito}
/* Pezzi di terreno di un tratto fuori strada: [{tipo,nome,px}] nell'ordine del cammino (calcolati una volta). */
function pezzi(s){if(s.pz&&s.pzGK===GKD)return s.pz;const out=[];
  for(let k=0;k<s.pts.length-1;k++){const[a,b]=[s.pts[k],s.pts[k+1]],L=Math.hypot(b[0]-a[0],b[1]-a[1]),n=Math.max(1,Math.ceil(L/3));
    for(let i=0;i<n;i++){const f=(i+.5)/n,x=a[0]+(b[0]-a[0])*f,y=a[1]+(b[1]-a[1])*f,g=gkF(x,y);
      // Sul tratto del Gelo valgono le sue regole (anche su strada); altrimenti strada o terreno della mappa.
      const z=g>=0?{tipo:"gelo",nome:g>=.5?"Monte Gelo, alta quota":"Monte Gelo"}:s.off?terrenoIn(x,y):stradaIn(x,y),u=out[out.length-1];
      const fine=[a[0]+(b[0]-a[0])*(i+1)/n,a[1]+(b[1]-a[1])*(i+1)/n];
      if(u&&u.tipo===z.tipo&&u.nome===z.nome){u.px+=L/n;u.b=fine;u.pts.push(fine);if(g>=0){u.f0=Math.min(u.f0,g);u.f1=Math.max(u.f1,g)}}
      else{const ini=[a[0]+(b[0]-a[0])*i/n,a[1]+(b[1]-a[1])*i/n];out.push({tipo:z.tipo,nome:z.nome,px:L/n,f0:g,f1:g,a:ini,b:fine,pts:[ini,fine]})}}}
  s.pzGK=GKD;return s.pz=out}
/* [miglia su strada, miglia fuori strada a velocità piena, miglia fuori strada su terreno difficile] */
/* Su strada conta il rilievo: collina e montagna rallentano, foresta, pianura e acqua (ponti) no. */
function stradaIn(x,y){const z=terrenoIn(x,y);return z.tipo==="collina"?{tipo:"stradaCollina",nome:z.nome}:z.tipo==="montagna"?{tipo:"stradaMontagna",nome:z.nome}:{tipo:"strada",nome:""}}
/* [miglia su strada a velocità piena, fuori strada a velocità piena, a metà velocità (terreno difficile o tratto del Gelo)] */
const legMi3=l=>{let r=0,p=0,d=0,c=0;l.segs.forEach(s=>pezzi(s).forEach(q=>{const m=pxMi(q.px),f=fatt(q.tipo);if(q.tipo==="strada")r+=m;else if(f===1)p+=m;else if(f===.5)d+=m;else c+=m}));return[r,p,d,c]};
const legMi=l=>{let r=0,o=0;l.segs.forEach(s=>s.off?o+=pxMi(s.px):r+=pxMi(s.px));return[r,o]};
/* tratto del Gelo in una tappa: miglia, di cui in alta quota, frazioni minima e massima */
function legGk(l){let mi=0,alta=0,f0=1,f1=0;l.segs.forEach(s=>pezzi(s).forEach(q=>{if(q.tipo!=="gelo")return;const m=pxMi(q.px);mi+=m;if(q.nome.includes("alta"))alta+=m;f0=Math.min(f0,q.f0);f1=Math.max(f1,q.f1)}));return{mi,alta,f0,f1}}
const legH=(l,mph)=>{let h=0;l.segs.forEach(s=>pezzi(s).forEach(q=>{h+=pxMi(q.px)/(mph*fatt(q.tipo))}));return h};
/* Tratti a velocità dimezzata di una tappa, raggruppati per terreno e nome della zona. */
function lenti(l){const m=new Map();l.segs.forEach(s=>{pezzi(s).forEach(q=>{if(fatt(q.tipo)===1||q.tipo==="gelo")return;const k=q.tipo+"|"+q.nome;m.set(k,(m.get(k)||0)+pxMi(q.px))})});
  return[...m.entries()].map(([k,mi])=>{const[tipo,nome]=k.split("|");return{tipo,nome,mi}}).filter(x=>x.mi>=0.05)}
async function mkWp(x,y,loc){const off=$("#off").checked,r=await snap(x,y,loc||!off?42:12);
  if(r>=0){const[px,py]=pt(r);return{x:px,y:py,i:r,loc}}
  if(!off){toast(loc?"Questa località non è su una strada: attiva «Fuori strada» per raggiungerla.":"Clicca su una strada o un sentiero (o attiva «Fuori strada»).");return null}
  return{x:Math.round(x),y:Math.round(y),i:-1,loc}}
async function legBetween(A,B){
  if(A.i>=0&&B.i>=0){const p=await astar(A.i,B.i);return p?{segs:[p]}:null}
  const opts=[{segs:[straight([A.x,A.y],[B.x,B.y])]}];
  const ra=A.i>=0?A.i:await snap(A.x,A.y,500),rb=B.i>=0?B.i:await snap(B.x,B.y,500);
  if(ra>=0&&rb>=0){const segs=[];let ok=true;if(A.i<0)segs.push(straight([A.x,A.y],pt(ra)));
    if(ra!==rb){const p=await astar(ra,rb);if(p)segs.push(p);else ok=false}
    if(B.i<0)segs.push(straight(pt(rb),[B.x,B.y]));if(ok)opts.push({segs})}
  const mph=+$("#pace").value;return opts.reduce((a,b)=>legH(b,mph)<legH(a,mph)?b:a)}
async function addWp(w){if(!w)return false;
  if(!wps.length){wps.push(w);render();return true}
  const l=await legBetween(wps[wps.length-1],w);if(!l){toast("Nessun collegamento via strada o sentiero conosciuto.");return false}
  if(l.segs.reduce((s,x)=>s+x.px,0)<1)return false;legs.push(l);wps.push(w);render();return true}
// Le tappe si aggiungono una alla volta, nell'ordine dei clic (le risposte del server arrivano dopo).
let RQ=Promise.resolve();const inCoda=f=>(RQ=RQ.then(f,f));
function add(x,y){if(role==="player"&&!cell(x,y)){toast("Quest’area è avvolta dalla nebbia.");return}inCoda(async()=>addWp(await mkWp(x,y,null)))}
function addLoc(l){if(role==="player"&&!cell(l.x,l.y)){toast("Quest’area è avvolta dalla nebbia.");return Promise.resolve(false)}return inCoda(async()=>addWp(await mkWp(l.x,l.y,l)))}

/* ---------- Dati della località (Master: tutto; Giocatore: solo ciò che è stato svelato) ---------- */
function info(l){
  if(role==="master"){const c=CONTENT[l.k]||{},g=G||{pubNames:{},pubItems:{},hideBuiltin:{}};
    return{pub:!!g.pubNames[l.k],desc:c.desc||"",notes:c.notes||"",bpub:!g.hideBuiltin[l.k],items:(c.items||[]).map(it=>Object.assign({},it,{pub:!!g.pubItems[it.id]}))}}
  const p=(PST&&PST.locs[l.k])||{};return{pub:!!p.name,desc:p.desc||"",bpub:!!p.builtin,items:(p.items||[]).map(it=>Object.assign({},it,{pub:true}))}}
const known=l=>role==="master"||(!!(PST&&PST.locs[l.k]&&PST.locs[l.k].name)&&!!cell(l.x,l.y));
const disp=l=>known(l)?l.n:"Località sconosciuta";
function nameOf(w){if(w.loc)return known(w.loc)?w.loc.n:null;let b=null,bd=45;for(const l of LOCS){const d=Math.hypot(l.x-w.x,l.y-w.y);if(d<bd&&known(l)){bd=d;b=l}}return b?b.n:null}
const mURL=f=>"/media/"+String(f).split("/").map(encodeURIComponent).join("/");
const mediaSrc=it=>it.k==="builtin"?it.src:it.file?mURL(it.file):it.url;
function itemsOf(l){const d=info(l),out=[];
  if(l.b&&(role==="master"||d.bpub))out.push({k:"builtin",src:"img/lm/"+l.k+".webp",cap:"Mappa locale",pub:d.bpub});
  d.items.forEach(it=>out.push(it));return out}

/* ---------- Disegno ---------- */
/* ---------- Gaster Bless (nuova app): segnalini del gruppo (ritratti e nomi dei PG dalle schede) ----------
   Si trascinano (Master e giocatori della sessione); tasto destro o pressione lunga: dividi, riunisci,
   personalizza (solo Master: nome, colore, simbolo), togli. Server: internal/server/mappa_segnalino.go. */
let PARTY=null,partyMode=false,PDRAG=false;
const SIMBOLI=["⚑","🐺","🦇","🛡","⚔","🏹","🔥","💀","🌙","🐎","🕯","👁"];
function drawParty(g){if(!PARTY||!PARTY.segnalini)return;const col=PARTY.colore||"#d9a441",div=PARTY.segnalini.length>1;
  PARTY.segnalini.forEach((s,si)=>{const k=el("g",{class:"party","data-id":s.id},g),pg=s.pg||[],n=Math.min(pg.length,4);
    const tt=el("title",{},k);tt.textContent="Trascina per spostare · tasto destro (o tocco lungo): dividi, riunisci"+(role==="master"?", personalizza":"");
    el("circle",{cx:s.x,cy:s.y,r:22,fill:"rgba(26,18,10,.75)",stroke:col,"stroke-width":3},k);
    if(!n){const t=el("text",{x:s.x,y:s.y+7,"text-anchor":"middle",style:"font:bold 20px sans-serif;fill:"+col},k);t.textContent=PARTY.simbolo||"⚑"}
    pg.slice(0,4).forEach((p,i)=>{const a=n>1?2*Math.PI*i/n:0,r=n>1?11:0,cx=s.x+r*Math.cos(a),cy=s.y+r*Math.sin(a),R=n>1?10:18;
      if(p.ritratto){const id="cp"+si+"_"+i;const cp=el("clipPath",{id},k);el("circle",{cx,cy,r:R},cp);const im=el("image",{x:cx-R,y:cy-R,width:2*R,height:2*R,"clip-path":"url(#"+id+")",preserveAspectRatio:"xMidYMid slice"},k);im.setAttribute("href","/api/v1/personaggi/"+encodeURIComponent(p.id)+"/ritratto")}
      else{el("circle",{cx,cy,r:R,fill:"#8a1c1c",stroke:"#fff","stroke-width":1.5},k);const t=el("text",{x:cx,y:cy+4,"text-anchor":"middle",style:"font:bold 11px sans-serif;fill:#fff"},k);t.textContent=(p.nome||"?")[0]}});
    if(n&&PARTY.simbolo){el("circle",{cx:s.x+18,cy:s.y-18,r:10,fill:col,stroke:"#1a120a","stroke-width":1.5},k);const t=el("text",{x:s.x+18,y:s.y-13.5,"text-anchor":"middle",style:"font:12px sans-serif"},k);t.textContent=PARTY.simbolo}
    const nomi=pg.map(p=>p.nome).join(", "),t=el("text",{x:s.x,y:s.y+40,"text-anchor":"middle",class:"nm","data-m":1},k);
    t.textContent=s.principale?(PARTY.nome?(div&&nomi?PARTY.nome+" ("+nomi+")":PARTY.nome):(nomi||"Il gruppo")):nomi;
    if(CONSULTA){k.style.cursor="default";return}partyDrag(k,s)})}
function partyDrag(k,s){let st=null,lt=0;
  const fine=()=>{clearTimeout(lt);st=null;PDRAG=false;if(markers.pend){markers.pend=0;markers()}};
  k.addEventListener("pointerdown",e=>{if(e.button)return;e.stopPropagation();e.preventDefault();try{k.setPointerCapture(e.pointerId)}catch(_){}
    st={x:e.clientX,y:e.clientY,moved:false};PDRAG=true;clearTimeout(lt);if(e.pointerType==="touch")lt=setTimeout(()=>{fine();partyMenu(s)},550)});
  k.addEventListener("pointermove",e=>{if(!st)return;const dx=e.clientX-st.x,dy=e.clientY-st.y;if(!st.moved&&Math.hypot(dx,dy)<5)return;
    st.moved=true;clearTimeout(lt);const r=map.getBoundingClientRect();k.setAttribute("transform","translate("+dx/r.width*W+" "+dy/r.height*H+")");k.classList.add("trascina")});
  k.addEventListener("pointerup",e=>{if(!st){fine();return}const m=st.moved,r=map.getBoundingClientRect();
    const x=Math.max(0,Math.min(W,s.x+(e.clientX-st.x)/r.width*W)),y=Math.max(0,Math.min(H,s.y+(e.clientY-st.y)/r.height*H));fine();if(!m)return;
    const da=[s.x,s.y];s.x=x;s.y=y;inviaMossa({azione:"muovi",id:s.id,x,y},da)});
  k.addEventListener("pointercancel",()=>{fine();k.removeAttribute("transform")});
  k.addEventListener("click",e=>e.stopPropagation());
  k.addEventListener("contextmenu",e=>{e.preventDefault();e.stopPropagation();fine();partyMenu(s)})}
function partyMenu(s){const pg=s.pg||[],ma=role==="master";
  const o=dlgX(`<h2>${esc(s.principale?(PARTY.nome||"Il gruppo"):"Parte del gruppo")}</h2><p class="mut">${pg.length?esc(pg.map(p=>p.nome).join(", ")):"Nessun personaggio nella sessione."}</p>
  <p>${pg.length>1?'<button data-a="div">✂ Dividi il gruppo…</button> ':""}${!s.principale?'<button data-a="unisci">🔗 Riunisci al gruppo</button> ':""}${ma?'<button data-a="asp">🎨 Personalizza…</button> <button data-a="togli">✕ Togli dalla mappa</button>':""}</p>
  <p class="mut">Per spostarlo basta trascinarlo.</p><p style="text-align:right"><button data-a="x">Chiudi</button></p>`);
  const fai=async(p,ok)=>{try{await api("/api/party",p);o.remove();await refresh();if(ok)toast(ok)}catch(er){toast(er.message)}};
  o.onclick=e=>{if(e.target===o){o.remove();return}const b=e.target.closest("[data-a]");if(!b)return;const a=b.dataset.a;
    if(a==="x")o.remove();
    else if(a==="unisci")fai({azione:"riunisci",id:s.id},"Il gruppo si è riunito.");
    else if(a==="togli")fai({azione:"togli"},"Segnalino tolto dalla mappa.");
    else if(a==="div"){o.querySelector(".bx").innerHTML=`<h2>✂ Chi si separa?</h2><p class="mut">I personaggi scelti formano un nuovo segnalino accanto a questo: poi trascinalo dove vanno.</p>
      ${pg.map(p=>`<label style="display:block;margin:6px 0"><input type="checkbox" value="${esc(p.id)}"> ${esc(p.nome)}</label>`).join("")}
      <p style="text-align:right"><button data-a="x">Annulla</button> <button data-a="ok">Dividi</button></p>`;
      o.onclick=e2=>{const b2=e2.target.closest("[data-a]");if(e2.target===o||b2&&b2.dataset.a==="x"){o.remove();return}if(!b2)return;
        const sc=[...o.querySelectorAll("input:checked")].map(i=>i.value);if(!sc.length||sc.length===pg.length){toast("Scegli chi si separa: almeno uno, ma non tutti.");return}
        fai({azione:"dividi",id:s.id,pg:sc,x:Math.min(W,s.x+45),y:Math.min(H,s.y+45)},"Gruppo diviso: trascina il nuovo segnalino.")}}
    else if(a==="asp"){o.querySelector(".bx").innerHTML=`<h2>🎨 Segnalino del gruppo</h2>
      <p><label>Nome <input id="pnome" maxlength="40" value="${esc(PARTY.nome||"")}" placeholder="Es. Persi nella nebbia"></label></p>
      <p><label>Colore <input id="pcol" type="color" value="${esc(PARTY.colore||"#d9a441")}"></label></p>
      <p><label>Simbolo <input id="psim" maxlength="4" size="4" value="${esc(PARTY.simbolo||"")}" placeholder="⚑"></label><br>${SIMBOLI.map(x=>`<button data-s="${x}" style="font-size:18px;min-width:40px">${x}</button>`).join(" ")}</p>
      <p class="mut">Vale per tutto il gruppo e lo vedono anche i giocatori.</p><p style="text-align:right"><button data-a="x">Annulla</button> <button data-a="ok">Salva</button></p>`;
      o.onclick=e2=>{const b2=e2.target.closest("[data-a],[data-s]");if(e2.target===o||b2&&b2.dataset.a==="x"){o.remove();return}if(!b2)return;
        if(b2.dataset.s){$("#psim").value=b2.dataset.s;return}
        fai({azione:"aspetto",nome:$("#pnome").value.trim(),colore:$("#pcol").value,simbolo:$("#psim").value.trim()},"Segnalino aggiornato.")}}}}
function markers(){if(PDRAG){markers.pend=1;return}const g0=$("#ov2");g0.innerHTML="";const g=el("g",{id:"mk"},g0);
  LOCS.forEach(l=>{const k=el("g",{class:"mkg","data-x":l.x,"data-y":l.y,style:"cursor:pointer"},g);
    let lp=0,lt=null;
    k.addEventListener("click",e=>{e.stopPropagation();if(lp){lp=0;return}if(drag.moved||tool!=="route")return;addLoc(l);preview(l)});
    k.addEventListener("contextmenu",e=>{e.preventDefault();e.stopPropagation();sheet(l)});
    k.addEventListener("pointerdown",e=>{if(e.pointerType!=="touch")return;clearTimeout(lt);lt=setTimeout(()=>{lp=1;sheet(l)},550)});
    ["pointerup","pointercancel","pointerleave"].forEach(t=>k.addEventListener(t,()=>clearTimeout(lt)));
    const tt=el("title",{},k);tt.textContent=role==="master"?l.c+". "+l.n+(l.ch?" (cap. "+l.ch+")":""):disp(l);
    el("circle",{cx:l.x,cy:l.y,r:l.mj?13:11,fill:l.mj?"rgba(217,164,65,.35)":"rgba(95,211,200,.25)",stroke:l.mj?"#d9a441":"#5fd3c8","stroke-width":l.mj?3:2},k);
    if(known(l)){const tx=el("text",{x:l.x,y:l.y-18,"text-anchor":"middle",class:"nm","data-m":l.mj?1:0},k);tx.textContent=l.n}});
  drawParty(g);labels();updMk()}
function labels(){document.querySelectorAll("#ov2 .nm").forEach(t=>t.style.display=(role==="player"||$("#lb").checked)&&(t.getAttribute("data-m")==="1"||zoom>=2||role==="player")?"":"none")}
function updMk(){document.querySelectorAll("#ov2 .mkg").forEach(k=>{k.style.display=role==="player"&&!cell(+k.getAttribute("data-x"),+k.getAttribute("data-y"))?"none":""})}
function render(){
  const o=$("#rt");if(o)o.remove();const g=el("g",{id:"rt","pointer-events":"none"});
  legs.forEach(l=>l.segs.forEach(s=>{const p=s.pts.map(q=>q.join(",")).join(" ");
    el("polyline",{points:p,fill:"none",stroke:"#1a120a","stroke-width":8,"stroke-linecap":"round","stroke-linejoin":"round"},g);
    if(!s.off){el("polyline",{points:p,fill:"none",stroke:"#ffd23f","stroke-width":4.5,"stroke-linecap":"round","stroke-linejoin":"round"},g);
      // Dove il rilievo rallenta, la strada stessa cambia colore e ne segue le curve (niente linee aggiuntive).
      pezzi(s).forEach(q=>{const col=q.tipo==="stradaCollina"?"#ff8c1a":q.tipo==="stradaMontagna"?"#d93a2b":q.tipo==="gelo"?"#d8f1ff":null;if(!col)return;
        const ln=el("polyline",{points:q.pts.map(v=>v[0].toFixed(1)+","+v[1].toFixed(1)).join(" "),fill:"none",stroke:col,"stroke-width":4.5,"stroke-linecap":"round","stroke-linejoin":"round"},g);
        el("title",{},ln).textContent=q.tipo==="gelo"?"Monte Gelo: salita ghiacciata, metà velocità"+(q.nome.includes("alta")?"; alta quota":""):NOMI_T[q.tipo]+" ("+q.nome+"): "+(q.tipo==="stradaCollina"?"3/4 della velocità":"metà velocità")});return}
    // fuori strada: arancio = terreno difficile (metà velocità), azzurro = pianura (velocità piena)
    pezzi(s).forEach(q=>{const ln=el("line",{x1:q.a[0],y1:q.a[1],x2:q.b[0],y2:q.b[1],stroke:q.tipo==="gelo"?"#d8f1ff":DIFF[q.tipo]?"#ff8a3d":"#9fd0ff","stroke-width":4.5,"stroke-linecap":"round","stroke-dasharray":"9 7"},g);
      const tt=el("title",{},ln);tt.textContent=NOMI_T[q.tipo]+" ("+q.nome+")"+(DIFF[q.tipo]?": terreno difficile, metà velocità":": velocità piena")})}));
  wps.forEach((w,i)=>{el("circle",{cx:w.x,cy:w.y,r:10,fill:"#8a1c1c",stroke:"#fff","stroke-width":2.5},g);
    const t=el("text",{x:w.x,y:w.y+4.5,"text-anchor":"middle",style:"font:bold 13px sans-serif;fill:#fff"},g);t.textContent=i+1});
  panel()}
/* Riquadro delle regole speciali del Monte Gelo, se il percorso passa sul tratto. */
function avvisoGelo(mph){if(!GKD)return{html:"",extra:0};let mi=0,alta=0,f0=1,f1=0;legs.forEach(l=>{const g=legGk(l);mi+=g.mi;alta+=g.alta;if(g.mi){f0=Math.min(f0,g.f0);f1=Math.max(f1,g.f1)}});
  if(mi<0.05)return{html:"",extra:0};const R=GK.regole,ore=mi/(mph/2),extra=alta/(mph/2),ts=Math.max(1,Math.ceil(ore-1e-9));
  return{extra,html:`<div class="avv-gelo" role="note"><b>❄ Regole speciali: Monte Gelo</b>
    <p>${mi1(mi)} mi sulla salita tra le Pendici del Monte Gelo e il Passo del Vento, da ${m0(gkQuota(f0))} a ${m0(gkQuota(f1))} m di quota. <b>Terreno impervio</b>: salita ripida, ghiaccio e neve rendono difficile anche la strada → <b>metà velocità</b> (${mph/2} mph): ${fmt(ore)}.</p>
    ${alta>=0.05?`<p><b>Alta quota</b> dalla metà del tratto (oltre ${m0(gkQuota(.5))} m): ${mi1(alta)} mi in cui ogni ora conta doppia per la fatica (+${fmt(extra)}).</p>`:""}
    <ul><li><b>${R.temperatura} °C</b>, raffiche di vento gelato: ogni ora TS su <b>Costituzione CD ${R.cdFreddo}</b> (abiti in pelliccia e mantello pesante = riuscita automatica, uno solo = tiro normale, nessuno = svantaggio); se fallisce, 1 livello di indebolimento.</li>
    <li>Ogni ora TS su <b>Forza o Destrezza CD ${R.cdVento}</b> contro il vento${alta>=0.05?" (con svantaggio dalla metà del tratto)":""}; armatura pesante, scarponi o stivali da scalatore danno vantaggio (con lo svantaggio si annullano): se fallisce cade prono e il gruppo perde 10 minuti${f1>=.5?"; con un 1 naturale, dalla metà del tratto, precipita nel vuoto":""}.</li>
    <li>Vento: svantaggio agli attacchi a distanza e alla Percezione con l'udito; torce e fuochi si spengono; chi vola atterra o cade.</li>
    <li>Neve: area leggermente oscurata (svantaggio alla Percezione con la vista). Ghiaccio: in combattimento Acrobazia CD 10 o prono. Cibo: Sopravvivenza CD 20.</li>
    <li>Tiri salvezza da fare in questo tratto: <b>${ts}</b> (uno per ogni ora).</li></ul></div>`}}
/* In cima al pannello: perché la velocità è dimezzata, per quanto e su quali tratti (2026-11-07). */
const mi1=x=>x.toFixed(1).replace(".",",");
function avvisoTerreno(mph){const righe=[];let extra=0,acqua=false,fs=false,st=false;
  legs.forEach((l,i)=>{const tr=lenti(l);if(!tr.length)return;let prima=0,dopo=0;
    tr.forEach(x=>{prima+=x.mi/mph;dopo+=x.mi/(mph*fatt(x.tipo));if(x.tipo==="acqua")acqua=true;if(x.tipo.startsWith("strada"))st=true;else fs=true});extra+=dopo-prima;
    righe.push(`<li><b>Tappa ${i+1}</b> (${esc(nameOf(wps[i])||"punto "+(i+1))} → ${esc(nameOf(wps[i+1])||"punto "+(i+2))}): `+
      tr.map(x=>`${mi1(x.mi)} mi di ${NOMI_T[x.tipo]} (${esc(x.nome)}, ${fatt(x.tipo)===.75?"3/4":"metà"} velocità)`).join(", ")+` · da ${fmt(prima)} a <b>${fmt(dopo)}</b></li>`)});
  if(!righe.length)return"";
  return`<div class="avv-terreno" role="note"><b>⚠ Velocità ridotta dal terreno su ${righe.length===1?"una tappa":righe.length+" tappe"}</b>`+
    (st?`<p><b>Su strada</b> il rilievo rallenta: in <b>collina</b> si va a 3/4 della velocità (${String(mph*.75).replace(".",",")} mph), in <b>montagna</b> a metà (${String(mph/2).replace(".",",")} mph). Foresta e pianura non rallentano chi resta sulla strada. Sul percorso la strada diventa <span style="color:#ff8c1a">arancio</span> in collina e <span style="color:#d93a2b">rossa</span> in montagna.</p>`:"")+
    (fs?`<p><b>Fuori strada</b> foresta, collina, montagna e acqua sono <b>terreno difficile</b>: metà velocità (${String(mph/2).replace(".",",")} mph invece di ${mph}). In pianura la velocità resta piena.</p>`:"")+
    `<ul>${righe.join("")}</ul><p>Tempo in più per il terreno: <b>${fmt(extra)}</b>.${acqua?" Il percorso attraversa un lago o un fiume: serve una barca o un guado.":""}</p></div>`}
function fmt(h){let H=Math.floor(h),m=Math.round((h-H)*60);if(m===60){H++;m=0}return H+" h "+String(m).padStart(2,"0")+" min"}
const mhs=()=>String(miHex).replace(".",",");
function panel(){
  const mph=+$("#pace").value,side=$("#res");$("#mhp").textContent="Scala: 1 esagono = "+mhs()+" mi";
  let h=`<h2>Percorso</h2>`;
  if(!wps.length){side.innerHTML=h+`<p class="mut">Clicca su una strada, un sentiero o una località per fissare la partenza; ogni clic successivo aggiunge una tappa. Il tracciato segue strade e sentieri; con <b>Fuori strada</b> puoi raggiungere anche luoghi isolati: tratteggio <span style="color:#ff8a3d">arancio</span> = terreno difficile (foresta, collina, montagna, acqua: metà velocità), <span class="off">azzurro</span> = pianura (velocità piena). Sulle strade il rilievo rallenta e la strada cambia colore: <span style="color:#ff8c1a">arancio</span> = collina (3/4 della velocità), <span style="color:#d93a2b">rosso</span> = montagna (metà).</p><p class="mut">Scala: 1 esagono = ${mhs()} mi. Un giorno di viaggio = 8 ore. Trascina per spostare la mappa, rotellina o due dita per lo zoom.</p><p class="mut">Tasto destro o pressione prolungata su una località: apre la sua galleria.</p>`;return}
  let R=0,O=0,hrs=0;legs.forEach(l=>{const[r,o]=legMi(l);R+=r;O+=o;hrs+=legH(l,mph)});const tot=R+O,days=Math.floor(hrs/8),rem=hrs-days*8;
  const gk=avvisoGelo(mph);
  h+=gk.html+avvisoTerreno(mph);
  h+=`<div class="big">${fmt(hrs)}</div><div>${mi1(tot)} miglia (${mi1(tot*1.609)} km)${O>0.05?` <span class="off">· di cui ${mi1(O)} fuori strada</span>`:""}</div>`;
  if(gk.extra>0){const F=hrs+gk.extra,dF=Math.floor(F/8);
    h+=`<div>${dF?dF+" giorn"+(dF>1?"i":"o")+" e ":""}${fmt(F-dF*8)} <small>di marcia ai fini della fatica (alta quota: +${fmt(gk.extra)}; 8 h al giorno)</small></div>`}
  else h+=`<div>${days?days+" giorn"+(days>1?"i":"o")+" e ":""}${fmt(rem)} <small>(8 h di marcia al giorno)</small></div>`;
  h+=`<p class="mut">${PACE[mph]} Oltre 8 ore al giorno è marcia forzata: TS Costituzione CD 10 + 1 per ogni ora extra, altrimenti un livello di indebolimento.</p>`;
  h+=`<b>Tappe</b><ol>`+legs.map((l,i)=>{const[r,o]=legMi(l);return`<li>${esc(nameOf(wps[i])||"punto "+(i+1))} → ${esc(nameOf(wps[i+1])||"punto "+(i+2))}: ${mi1(r+o)} mi${o>0.05?` <span class="off">(${mi1(o)} f.s.)</span>`:""}, ${fmt(legH(l,mph))}</li>`}).join("")+`</ol>`;
  if(!legs.length)h+=`<p class="mut">Partenza fissata: ${esc(nameOf(wps[0])||"punto 1")}. Scegli la prossima tappa.</p>`;
  side.innerHTML=h}
// Anteprima della località cliccata, nel pannello laterale
function preview(l){PVL=l||PVL;const p=$("#pv");if(!PVL||(role==="player"&&!cell(PVL.x,PVL.y))){p.hidden=true;return}
  const d=info(PVL),its=itemsOf(PVL).filter(x=>x.k==="img"||x.k==="builtin"),first=its.find(x=>x.k==="img")||its[0];
  p.innerHTML=`<h2>${esc(disp(PVL))}</h2>${first?`<img src="${esc(mediaSrc(first))}" alt="" data-a="g">`:`<p class="mut">Nessuna immagine disponibile.</p>`}<div class="txt mut" id="pvd"></div><p style="margin:6px 0 0"><button data-a="g">🖼 Galleria${itemsOf(PVL).length>1?" ("+itemsOf(PVL).length+")":""}</button> <button data-a="x">✕</button></p>`;
  $("#pvd").textContent=d.desc.length>220?d.desc.slice(0,220)+"…":d.desc;p.hidden=false;
  p.onclick=e=>{const b=e.target.closest("[data-a]");if(!b)return;if(b.dataset.a==="g")sheet(PVL);else{PVL=null;p.hidden=true}}}

/* ---------- Interazione mappa: clic, trascinamento, zoom ---------- */
const drag={on:false,moved:false,x:0,y:0,sl:0,st:0};let pinch=null;
map.addEventListener("click",e=>{if(tool!=="route"||drag.moved)return;const r=map.getBoundingClientRect();
  if(partyMode){partyMode=false;$("#party").classList.remove("on");map.style.cursor="crosshair";const p0=PARTY&&PARTY.segnalini&&PARTY.segnalini[0];inviaMossa({azione:"posiziona",x:(e.clientX-r.left)/r.width*W,y:(e.clientY-r.top)/r.height*H},p0?[p0.x,p0.y]:null);return}add((e.clientX-r.left)/r.width*W,(e.clientY-r.top)/r.height*H)});
vp.addEventListener("pointerdown",e=>{if(tool!=="route"||e.button)return;drag.moved=false;if(e.pointerType==="mouse"){drag.on=true;drag.x=e.clientX;drag.y=e.clientY;drag.sl=vp.scrollLeft;drag.st=vp.scrollTop}});
vp.addEventListener("pointermove",e=>{if(!drag.on)return;const dx=e.clientX-drag.x,dy=e.clientY-drag.y;if(!drag.moved&&Math.hypot(dx,dy)<5)return;drag.moved=true;vp.scrollLeft=drag.sl-dx;vp.scrollTop=drag.st-dy;map.style.cursor="grabbing"});
const endPtr=()=>{if(drag.on){drag.on=false;map.style.cursor=tool==="route"?"crosshair":"cell";setTimeout(()=>drag.moved=false,0)}};
["pointerup","pointercancel"].forEach(t=>vp.addEventListener(t,endPtr));
const tdist=t=>Math.hypot(t[0].clientX-t[1].clientX,t[0].clientY-t[1].clientY);
vp.addEventListener("touchstart",e=>{if(e.touches.length===2&&tool==="route"){pinch={d:tdist(e.touches),z:zoom};e.preventDefault()}},{passive:false});
vp.addEventListener("touchmove",e=>{if(pinch&&e.touches.length===2){e.preventDefault();const t=e.touches;zoomAt(pinch.z*tdist(t)/pinch.d/zoom,(t[0].clientX+t[1].clientX)/2,(t[0].clientY+t[1].clientY)/2)}},{passive:false});
vp.addEventListener("touchend",e=>{if(e.touches.length<2)pinch=null});
// iPhone: Safari ingrandirebbe tutta la pagina con due dita (ignora user-scalable): sulla mappa lo zoom è quello della mappa.
vp.addEventListener("gesturestart",e=>e.preventDefault());vp.addEventListener("gesturechange",e=>e.preventDefault());
function zoomAt(f,cx,cy){const r=map.getBoundingClientRect(),fx=(cx-r.left)/r.width,fy=(cy-r.top)/r.height,old=zoom;
  zoom=Math.min(6,Math.max(1,zoom*f));if(zoom===old)return;map.style.width=zoom*100+"%";labels();
  const r2=map.getBoundingClientRect();vp.scrollLeft+=r2.left+fx*r2.width-cx;vp.scrollTop+=r2.top+fy*r2.height-cy}
vp.addEventListener("wheel",e=>{e.preventDefault();zoomAt(e.deltaY<0?1.15:1/1.15,e.clientX,e.clientY)},{passive:false});
const zc=f=>{const r=vp.getBoundingClientRect();zoomAt(f,r.left+r.width/2,r.top+r.height/2)};
$("#zi").onclick=()=>zc(1.3);$("#zo").onclick=()=>zc(1/1.3);
$("#undo").onclick=()=>{if(legs.length)legs.pop();wps.pop();render()};
$("#clr").onclick=()=>{wps=[];legs=[];render()};
$("#pace").onchange=panel;$("#lb").onchange=labels;$("#off").onchange=()=>{try{localStorage.setItem("brOff",$("#off").checked?"1":"")}catch(e){}};
try{$("#off").checked=!!localStorage.getItem("brOff")}catch(e){}

/* ---------- Da … a … ---------- */
function fillSel(){const vis=LOCS.map((l,i)=>i).filter(i=>known(LOCS[i]));
  const ord=vis.sort((a,b)=>LOCS[a].n.localeCompare(LOCS[b].n,"it"));
  const opts=ord.map(i=>`<option value="${i}">${esc(LOCS[i].n)}</option>`).join("");
  const pa=$("#fa").value,pb=$("#fb").value;$("#fa").innerHTML=opts;$("#fb").innerHTML=opts;
  const ix=n=>String(LOCS.findIndex(l=>l.n===n));
  $("#fa").value=pa!==""&&vis.includes(+pa)?pa:ix("Villaggio di Nebbiavalle");$("#fb").value=pb!==""&&vis.includes(+pb)?pb:ix("Borgofosco");
  if($("#fa").selectedIndex<0)$("#fa").selectedIndex=0;if($("#fb").selectedIndex<0)$("#fb").selectedIndex=Math.min(1,ord.length-1);
  $("#ft").hidden=ord.length<2}
$("#sw").onclick=()=>{const t=$("#fa").value;$("#fa").value=$("#fb").value;$("#fb").value=t};
$("#ia").onclick=()=>{const l=LOCS[+$("#fa").value];if(l)sheet(l)};$("#ib").onclick=()=>{const l=LOCS[+$("#fb").value];if(l)sheet(l)};
$("#go").onclick=async()=>{const ia=+$("#fa").value,ib=+$("#fb").value,a=LOCS[ia],b=LOCS[ib];
  if(!a||!b)return;if(ia===ib){toast("Scegli due località diverse.");return}
  wps=[];legs=[];render();if(!await addLoc(a)){wps=[];legs=[];render();return}
  if(!await addLoc(b)){wps=[];legs=[];render()}};

/* ---------- Codice della campagna (fase L2, 2026-11-12; solo Master) ----------
   Nella scheda della località: le schede del Codice che si chiamano come il luogo (o hanno il suo nome tra gli alias),
   e quelle che lo hanno nei campi Luogo/Dove/Regione o tra i collegamenti (PNG che ci vivono, eventi, segreti).
   Il pulsante apre la scheda nell'app (messaggio «gasterbless:vai» alla pagina che contiene la mappa). */
const normC=s=>String(s||"").normalize("NFD").replace(/[\u0300-\u036f]/g,"").toLowerCase().replace(/[«»"“”'’`]/g,"").replace(/\s+/g," ").trim();
const ICOC={luogo:"🏰",png:"🧑",fazione:"⚜",evento:"⚡",trama:"🧵",oggetto:"🗡",segreto:"🔒",cronologia:"📅"};
let CODICE=null,CODICE_T=0;
async function codiceLoc(l){const box=$("#cdx");if(!box)return;
  try{if(!CODICE||Date.now()-CODICE_T>20000){const r=await fetch("/api/v1/lore",{credentials:"same-origin"});if(!r.ok)return;CODICE=(await r.json()).schede||[];CODICE_T=Date.now()}}catch(e){return}
  const n=normC(l.n),nomi=x=>[x.nome,...Object.entries(x.campi||{}).filter(([k])=>/^(alias|altri nomi)$/i.test(k.trim())).flatMap(([,v])=>String(v).split(/[;,]/))].map(normC);
  const dove=x=>Object.entries(x.campi||{}).filter(([k])=>/^(luogo|dove|regione)$/i.test(k.trim())).some(([,v])=>String(v).split(/[;,]/).map(normC).includes(n))||(x.collegamenti||[]).some(c=>normC(c)===n);
  const qui=CODICE.filter(x=>nomi(x).includes(n)),vicine=CODICE.filter(x=>!qui.includes(x)&&dove(x));
  const b=$("#cdx");if(!b||!SH||SH.l!==l)return;
  if(!qui.length&&!vicine.length){b.innerHTML="";return}
  const bt=x=>`<button data-cdx="${esc(x.id)}" title="Apri nel Codice della campagna">${ICOC[x.tipo]||"🗝"} ${esc(x.nome)}</button>`;
  b.innerHTML=`<h3>🗝 Codice della campagna</h3>`+(qui.length?`<p>${qui.map(bt).join(" ")}</p>`:"")+(vicine.length?`<p class="mut">Legati a questo luogo:</p><p>${vicine.map(bt).join(" ")}</p>`:"");
  b.onclick=e=>{const t=e.target.closest("[data-cdx]");if(!t)return;try{window.parent.postMessage({type:"gasterbless:vai",percorso:"/codice?scheda="+encodeURIComponent(t.dataset.cdx)},location.origin)}catch(_){}}}
/* apertura diretta di una località: /carta/?sessione=…&loc=<chiave> (dal Codice della campagna) */
(()=>{const k=new URLSearchParams(location.search).get("loc");if(!k)return;const t=setInterval(()=>{if(role&&LOCS.length&&RSIG){clearInterval(t);const l=LOCS.find(x=>x.k===k);if(l)sheet(l)}},250);setTimeout(()=>clearInterval(t),20000)})();

/* ---------- Scheda località: galleria (tutti) + gestione contenuti e proiezione (Master) ---------- */
let SH=null;
let LOOSE=[];
async function loadLoose(scan){try{const r=await api("/api/folders",scan?{}:undefined);LOOSE=r.loose||[];return r}catch(e){LOOSE=[];return{}}}
function sheet(l){if(role==="player"&&!cell(l.x,l.y))return;closeSheet();if(role==="master")loadLoose().then(()=>{if(SH&&SH.l===l&&!SH.o.contains(document.activeElement))drawSheet()});const o=document.createElement("div");o.className="dlg";o.innerHTML='<div class="bx" role="dialog" aria-modal="true"></div>';
  document.body.appendChild(o);o.addEventListener("click",e=>{if(e.target===o)closeSheet()});SH={l,o,sel:0,busy:""};drawSheet()}
function closeSheet(){if(SH){SH.o.querySelectorAll("video").forEach(v=>v.pause());SH.o.remove();SH=null}}
/* Esc chiude sempre ciò che è in primo piano (richiesta dell'utente, 2026-11-08): video dei canti, proiezione, immagini
   mostrate dal Master, galleria, finestre (.dlg) e scheda della località. Se qui non c'è nulla, il tasto passa all'app
   (avvisi a tutto schermo sopra la mappa). */
document.addEventListener("keydown",e=>{if(e.key!=="Escape")return;
  if($("#songv")){songVideoOff();return}
  if($("#pjov")){const o=$("#pjov"),v=o.querySelector("video");if(v)v.pause();o.remove();return}
  if($("#shw")){$("#shw").remove();return}
  if($("#lbx")){$("#lbx").remove();return}
  const d=[...document.querySelectorAll(".dlg")].pop();if(d){if(SH&&d===SH.o)closeSheet();else{d.querySelectorAll("video").forEach(v=>v.pause());d.remove()}return}
  if(SH){closeSheet();return}
  if(window.parent!==window)try{window.parent.postMessage({type:"gasterbless:esc"},location.origin)}catch(_){}});
document.addEventListener("keydown",e=>{ // frecce: luogo precedente/successivo nella scheda aperta
  if(SH&&!/INPUT|TEXTAREA|SELECT/.test((document.activeElement||{}).tagName||"")){if(e.key==="ArrowRight")go(1);if(e.key==="ArrowLeft")go(-1)}});
function go(d){const n=itemsOf(SH.l).length;if(!n)return;SH.sel=(SH.sel+d+n)%n;drawSheet()}
function drawSheet(){if(!SH)return;const{l}=SH,d=info(l),its=itemsOf(l),ma=role==="master";if(SH.sel>=its.length)SH.sel=Math.max(0,its.length-1);if(SH.sel<0)SH.sel=0;/* B5 */const it=its[SH.sel];
  let stage;
  if(!it)stage=`<div class="empty">${ma?"Nessun contenuto per questa località.<br>Aggiungi immagini, video o link qui sotto.":"Nessun contenuto disponibile."}</div>`;
  else if(it.k==="vid")stage=`<video src="${esc(mediaSrc(it))}" controls playsinline preload="metadata"></video>`;
  else if(it.k==="link")stage=`<div class="lk">🔗 ${esc(it.cap||"Collegamento esterno")}<br><a href="${esc(it.url)}" target="_blank" rel="noopener noreferrer">Apri ${/youtu\.?be/.test(it.url)?"su YouTube":"il collegamento"} ↗</a><br><small>${esc(it.url)}</small></div>`;
  else stage=`<img src="${esc(mediaSrc(it))}" alt="${esc(it.cap||disp(l))}" data-z="1">`;
  if(its.length>1)stage+=`<button class="nav l" data-a="prev" aria-label="Precedente">‹</button><button class="nav r" data-a="next" aria-label="Successivo">›</button>`;
  const th=its.length>1||ma?`<div class="th">`+its.map((x,i)=>`<button data-s="${i}" class="${i===SH.sel?"on":""}" title="${esc(x.cap||"")}">${x.k==="vid"?`<video src="${esc(mediaSrc(x))}#t=0.5" preload="metadata" muted></video><span class="bdg">▶</span>`:x.k==="link"?"🔗":`<img src="${esc(mediaSrc(x))}" loading="lazy" alt="">`}${ma&&!x.pub?`<span class="hid">solo master</span>`:""}${ma&&isProj(l,x)?`<span class="bdg" style="left:3px;right:auto">📺</span>`:""}</button>`).join("")+`</div>`:"";
  let ed="";
  if(ma){
    if(it)ed+=`<div class="ed"><b>Elemento ${SH.sel+1}</b>${it.k==="builtin"?`<span class="mut">Mappa locale inclusa</span>`:`<input type="text" id="ecap" value="${esc(it.cap||"")}" placeholder="Didascalia" maxlength="200">`}
      <label><input type="checkbox" id="epub"${it.pub?" checked":""}> Visibile ai giocatori</label>
      ${it.k!=="link"?`<button data-a="proj">📺 ${isProj(l,it)?"In proiezione":"Proietta"}</button>`:""}
      ${it.k==="builtin"?"":`<button data-a="ml" title="Sposta a sinistra">◀</button><button data-a="mr" title="Sposta a destra">▶</button><button data-a="del">🗑 Elimina</button>`}</div>`;
    ed+=`<div class="ed"><input type="file" id="up" accept="image/*,video/mp4,video/webm,video/quicktime" multiple hidden><button data-a="up" ${SH.busy?"disabled":""}>＋ Immagini / video</button>
      <input type="text" id="lku" placeholder="https://… (YouTube, sito, documento)"><input type="text" id="lkt" placeholder="Titolo del link" style="flex:.6;min-width:120px"><button data-a="lk">＋ Link</button>
      <select id="lfo" title="Immagini e video già presenti nelle cartelle Video e Immagini"><option value="">📂 Dalla cartella… (${LOOSE.length})</option>${LOOSE.map(f=>`<option value="${esc(f)}">${esc(f)}</option>`).join("")}</select><button data-a="scan" title="Collega i file messi nelle sottocartelle con il nome di una località (es. Video\\Borgofosco)">🔄 Cerca nelle cartelle</button>
      ${SH.busy?`<span class="mut" id="bz">${esc(SH.busy)}</span><span class="pbar"><i id="pb"></i></span>`:""}</div>`}
  const txt=ma?`<div class="cols"><div><h3>Descrizione per i giocatori</h3><textarea id="tdesc" placeholder="Visibile ai giocatori quando il nome della località è svelato.">${esc(d.desc)}</textarea></div><div><h3>Note del Master <small class="mut">(solo per te)</small></h3><textarea id="tnot" placeholder="Segreti, PNG, incontri, tesori…">${esc(d.notes)}</textarea></div></div><p><button data-a="savet">Salva testi</button> <span class="mut" id="st"></span><br><small class="mut">Immagini, video e testi valgono per tutti i gruppi; la visibilità ai giocatori è separata per ogni gruppo.</small></p>`
    :(d.desc?`<div class="txt"></div>`:"");
  SH.o.querySelector(".bx").innerHTML=`<div class="sh"><h2>${esc(disp(l))}${ma&&l.ch?` <small class="mut">· cap. ${l.ch}</small>`:""}</h2><div>${ma?`<label title="Mostra il nome sulla mappa dei giocatori e nell'elenco Da … a …"><input type="checkbox" id="pubn"${d.pub?" checked":""}> Nome e descrizione visibili ai giocatori</label>`:""}</div><div>${ma?`<label title="Parte da sola quando apri o proietti un video di questa località">🎵 <select id="lmus"><option value="">nessuna musica</option>${MUS.files.map(f=>`<option value="${esc(f)}"${(MUS.locs||{})[l.k]===f?" selected":""}>${esc(musName(f))}</option>`).join("")}</select></label> `:""}<button data-a="route">＋ Aggiungi al percorso</button> <button data-a="close">Chiudi</button></div></div>
    <div class="stage">${stage}</div><p class="cap">${it&&it.k!=="link"?esc(it.cap||""):""}</p>${th}${ed}${txt}${ma?'<div id="cdx" class="cdx"></div>':""}`;
  const tx=SH.o.querySelector(".txt");if(tx)tx.textContent=d.desc;if(ma)codiceLoc(l);
  SH.o.querySelector(".bx").onclick=e=>{const b=e.target.closest("[data-a],[data-s],[data-z]");if(!b)return;
    if(b.dataset.s!=null){SH.sel=+b.dataset.s;drawSheet();return}
    if(b.dataset.z){const lb=document.createElement("div");lb.id="lbx";lb.innerHTML=`<img src="${esc(b.src)}" alt="">`;lb.onclick=()=>lb.remove();document.body.appendChild(lb);return}
    const a=b.dataset.a;
    if(a==="close")closeSheet();else if(a==="prev")go(-1);else if(a==="next")go(1);
    else if(a==="route"){const L=SH.l;closeSheet();setToolRoute();addLoc(L);preview(L)}
    else if(a==="up")$("#up").click();
    else if(a==="lk")addLink();
    else if(a==="scan"){loadLoose(true).then(r=>{toast(r.linked?`Collegati ${r.linked} file dalle cartelle delle località.`:"Nessun nuovo file nelle sottocartelle con il nome di una località.");refresh()})}
    else if(a==="savet")saveTexts();
    else if(a==="proj")sceltaProiezione(it.k==="builtin"?{kind:"builtin",k:l.k}:{kind:"item",id:it.id},(it.k==="vid"?"il video":"l’immagine")+(it.cap?" «"+it.cap+"»":"")+" di "+(l.n||"questa località"));
    else if(a==="ml"||a==="mr"){SH.sel+=a==="ml"?-1:1;act(api("/api/item",{id:it.id,move:a==="ml"?-1:1}))}
    else if(a==="del"){if(b.dataset.c)act(api("/api/item?id="+encodeURIComponent(it.id),undefined,"DELETE"));else{b.dataset.c=1;b.textContent="Conferma eliminazione";setTimeout(()=>{if(b.isConnected){delete b.dataset.c;b.textContent="🗑 Elimina"}},4000)}}};
  if(ma&&it&&it.k==="vid"&&SH.vidMus!==it.id){SH.vidMus=it.id;if((MUS.locs||{})[l.k])mcmd({op:"loc",k:l.k})}
  if(ma){const lf=$("#lfo");if(lf)lf.onchange=async()=>{const f=lf.value;if(!f)return;try{await api("/api/item",{k:l.k,file:f});await loadLoose();await refresh();SH.sel=itemsOf(l).length-1;drawSheet();toast("File collegato alla località. È visibile solo a te finché non spunti «Visibile ai giocatori».")}catch(e){toast(e.message)}};
    $("#lmus").onchange=async e=>{await mcmd({op:"setloc",k:l.k,file:e.target.value});await musLoad();toast(e.target.value?"Musica della località impostata.":"Musica della località rimossa.")};
    $("#up").onchange=e=>uploadFiles([...e.target.files]);
    $("#pubn").onchange=e=>act(api("/api/group",{kind:"name",key:l.k,val:e.target.checked}));
    const ep=$("#epub");if(ep)ep.onchange=()=>act(api("/api/group",it.k==="builtin"?{kind:"builtin",key:l.k,val:ep.checked}:{kind:"item",key:it.id,val:ep.checked}));
    const ec=$("#ecap");if(ec)ec.onchange=()=>act(api("/api/item",{id:it.id,cap:ec.value}))}}
function setToolRoute(){if(tool!=="route")setFog(false)}
async function act(p){try{await p;await refresh()}catch(e){toast(e.message)}}
async function saveTexts(){const l=SH.l;$("#st").textContent="Salvataggio…";try{await api("/api/loc",{k:l.k,desc:$("#tdesc").value,notes:$("#tnot").value});await refresh();const s=$("#st");if(s)s.textContent="Salvato ✓"}catch(e){toast(e.message)}}
function addLink(){let u=$("#lku").value.trim();const t=$("#lkt").value.trim();if(!u)return;if(!/^https?:\/\//i.test(u))u="https://"+u;
  try{new URL(u)}catch(e){toast("Indirizzo non valido.");return}
  const l=SH.l;act(api("/api/item",{k:l.k,url:u,cap:t}).then(()=>{if(SH)SH.sel=itemsOf(l).length}))}
async function prepImg(f){if(/gif/.test(f.type))return f;
  try{const bmp=await createImageBitmap(f);const s=Math.min(1,2400/Math.max(bmp.width,bmp.height));const c=document.createElement("canvas");c.width=Math.round(bmp.width*s);c.height=Math.round(bmp.height*s);c.getContext("2d").drawImage(bmp,0,0,c.width,c.height);
    const b=await new Promise(r=>c.toBlob(r,"image/webp",.85));return b&&(b.size<f.size||!/png|jpeg|webp/.test(f.type))?b:f}catch(e){return f}}
function xhrUpload(url,blob,onp,method){return new Promise((res,rej)=>{const x=new XMLHttpRequest();x.open(method||"POST",compat(url));x.setRequestHeader("Content-Type",blob.type);x.setRequestHeader("X-CSRF",CSRF);
  x.upload.onprogress=e=>{if(e.lengthComputable)onp(e.loaded/e.total)};x.onload=()=>{let j=null;try{j=JSON.parse(x.responseText)}catch(e){}x.status<300?res(j):rej(new Error((j&&(j.errore||j.error))||"Caricamento non riuscito."))};x.onerror=()=>rej(new Error("Connessione con Gaster Bless interrotta durante il caricamento"+(blob.size>50e6?" ("+Math.round(blob.size/1e6)+" MB)":"")+". Il motivo è scritto nella finestra nera di Gaster Bless."));x.send(blob)})}
async function uploadFiles(fs){if(!SH||!fs.length)return;const l=SH.l;let n=0,i=0;
  for(const f of fs){i++;if(!SH)break;SH.busy=`Caricamento ${i}/${fs.length}: ${f.name}`;drawSheet();
    try{const isV=f.type.startsWith("video/");const blob=isV?f:await prepImg(f);
      await xhrUpload(`/api/upload?k=${encodeURIComponent(l.k)}&cap=${encodeURIComponent(f.name.replace(/\.[^.]+$/,""))}`,blob,p=>{const b=$("#pb");if(b)b.style.width=Math.round(p*100)+"%"});n++}
    catch(e){toast(f.name+": "+e.message,4500)}}
  if(SH)SH.busy="";await refresh();if(SH&&SH.l===l){SH.sel=itemsOf(l).length-1;drawSheet()}
  if(n)toast(n+" elemento/i aggiunto/i. Per questo gruppo sono visibili solo a te finché non spunti «Visibile ai giocatori» (o li proietti).",5000)}

/* ---------- Proiezione su secondo schermo (Master) ---------- */
const isProj=(l,it)=>PROJ&&(it.k==="builtin"?PROJ.kind==="builtin"&&PROJ.k===l.k:PROJ.kind==="item"&&PROJ.id===it.id);
/* Dove proiettare (richiesta dell'utente, 2026-11-06), scelta del Master ricordata su questo dispositivo:
   "giocatori" = sui dispositivi dei giocatori (audio a zero) e su quello del Master (audio attivo);
   "schermo"   = finestra di proiezione su un secondo schermo collegato al PC;
   "wowlink"   = finestra di proiezione trasmessa senza fili al proiettore Wowlink (Win+K, Miracast). */
const DOVE={giocatori:"📱 Dispositivi dei giocatori",schermo:"🖥 Secondo schermo o proiettore via cavo (HDMI)",wowlink:"📽 Proiettore Wowlink"};
let PROJD=(()=>{try{return localStorage.getItem("gasterbless-proj-dove")||"schermo"}catch(e){return"schermo"}})();if(!DOVE[PROJD])PROJD="schermo";
async function project(p){try{PROJ=await api("/api/proj",Object.assign({dove:PROJD},p));await refresh();projStatus();projOverlay();
  if(PROJD!=="giocatori"&&!projStatus.hint){projStatus.hint=1;toast("Se la finestra di proiezione non è aperta, aprila da «📺 Proiezione».",4000)}}catch(e){toast(e.message)}}
/* Proiezione sui dispositivi dei giocatori: riquadro a tutto schermo nella mappa (giocatori e Master) */
let pjovK="",pjCaricata=false; // pjCaricata: dopo il primo stato, un video proiettato riparte solo per una nuova proiezione
function projOverlay(){const on=!PJ&&PROJ&&PROJ.dove==="giocatori"&&PROJ.kind&&PROJ.kind!=="none";const k=on?JSON.stringify([PROJ.kind,PROJ.id,PROJ.k,PROJ.file]):"";
  let o=$("#pjov");if(!on){if(o){const v=o.querySelector("video");if(v){v.pause();v.removeAttribute("src");v.load()}o.remove()}pjovK="";return}
  if(o&&pjovK===k)return;if(o)o.remove();pjovK=k;
  o=document.createElement("div");o.id="pjov";const ma=role==="master";let it=null,h="";
  if(PROJ.kind==="item")for(const c in CONTENT)for(const x of CONTENT[c].items||[])if(x.id===PROJ.id)it=x;
  if(PROJ.kind==="map")h=`<img src="${esc(compat("/api/immagine")+"&vista=giocatori&v="+Date.now())}" alt="Mappa">`;
  else if(PROJ.kind==="builtin")h=`<img src="img/lm/${esc(PROJ.k)}.webp" alt="">`;
  else if(PROJ.kind==="beast")h=`<img src="${esc(mURL(PROJ.file))}" alt="">`+(PROJ.cap?`<div class="pjcap">${esc(PROJ.cap)}</div>`:"");
  else if(PROJ.kind==="item"&&ma&&it)h=(it.k==="vid"?`<video src="${esc(mURL(it.file))}" playsinline${pjCaricata?" autoplay":""} preload="metadata" controls></video>`:`<img src="${esc(mURL(it.file))}" alt="">`)+(it.cap?`<div class="pjcap">${esc(it.cap)}</div>`:"");
  else if(PROJ.kind==="item")h=`<video data-pj-id="${esc(PROJ.id)}" playsinline muted preload="auto"></video><img data-pj-id="${esc(PROJ.id)}" alt="" hidden>`;
  o.innerHTML=`<div class="pjovb"><b>📺 ${esc(projLabel())}</b>${ma?'<button data-pjx>Chiudi per tutti</button>':'<button data-pjh>Nascondi</button>'}</div><div class="pjovc">${h}</div>`;
  document.body.appendChild(o);
  if(PROJ.kind==="item"&&!ma)projItemPlayer(o);
  const v=o.querySelector("video");if(v&&ma&&!pjCaricata){toast("📺 Questa sessione ha un video in proiezione: avvialo dai comandi o «Chiudi per tutti».",5000)}
  else if(v&&ma){v.muted=false;v.play().catch(()=>{v.muted=true;v.play().catch(()=>{});toast("Clicca sul video per attivare l'audio.",4000)})}
  o.querySelector("[data-pjx]")?.addEventListener("click",()=>project({kind:"none",dove:"giocatori"}));
  o.querySelector("[data-pjh]")?.addEventListener("click",()=>{o.remove()});}
/* Giocatore a distanza: al tavolo l'audio esce dal PC del Master, chi gioca da casa sua invece lo sente solo dal suo
   dispositivo, quindi il video parte con l'audio al massimo. Se il browser blocca l'audio automatico, un tasto lo attiva. */
function audioPieno(v,o){v.muted=false;v.volume=1;v.play().catch(()=>{v.muted=true;v.play().catch(()=>{});
  if(!o||o.querySelector("[data-pja]"))return;const b=document.createElement("button");b.dataset.pja="";b.className="pjaudio";b.textContent="🔊 Attiva l'audio";
  b.onclick=()=>{v.muted=false;v.volume=1;v.play().catch(()=>{});b.remove()};o.appendChild(b)})}
/* Se il telefono blocca l'avvio automatico (risparmio energetico, Samsung Internet, iPhone in modalità basso consumo)
   il video resterebbe fermo senza dire nulla: compare un tasto per avviarlo. */
function tastoAvvio(v,o){if(!o||o.querySelector("[data-pjp]"))return;const b=document.createElement("button");b.dataset.pjp="";b.className="pjaudio";b.textContent="▶ Tocca per avviare il video";
  b.onclick=()=>{v.play().then(()=>b.remove()).catch(()=>{})};o.appendChild(b)}
/* Precaricamento (2026-11-11): al tavolo 5-6 telefoni scaricano lo stesso video insieme e, partendo subito con un attimo
   di video in memoria, si bloccavano e ripartivano. Ora il telefono aspetta di avere circa 6 secondi di video (tutto, se è
   più corto), al massimo 12 secondi, mostrando l'avanzamento; poi parte. iPhone e iPad non scaricano senza play: lì si
   parte subito come prima (Safari continua a scaricare mentre riproduce). */
const IOS=/iP(hone|ad|od)/.test(navigator.userAgent)||(navigator.platform==="MacIntel"&&navigator.maxTouchPoints>1);
function precarica(v,o,poi){if(IOS){poi();return}let fatto=false;const bar=document.createElement("div");bar.className="pjcarica";bar.textContent="⏳ Caricamento del video…";if(o)o.appendChild(bar);
  const avanti=()=>{try{const t=v.currentTime||0;for(let i=0;i<v.buffered.length;i++)if(v.buffered.start(i)<=t+.1&&v.buffered.end(i)>t)return v.buffered.end(i)-t}catch(e){}return 0};
  const serve=()=>{const d=isFinite(v.duration)&&v.duration>0?v.duration:0;return d?Math.min(6,Math.max(.5,d-.3)):6};
  const ev=["progress","canplaythrough","loadeddata","durationchange","suspend"];
  const fine=()=>{if(fatto)return;fatto=true;clearInterval(ti);clearTimeout(to);ev.forEach(e=>v.removeEventListener(e,ctl));bar.remove();poi()};
  const ctl=()=>{if(fatto)return;if(!v.isConnected)return fine();const a=avanti(),q=serve();/* non readyState: Chrome dice «abbastanza» con mezzo secondo in memoria. networkState 1 (fermo) con qualcosa in memoria:
     il browser ha smesso di scaricare perché ha tutto il file o abbastanza scorta */
  if(a>=q||(v.networkState===1&&a>0.3))return fine();bar.textContent="⏳ Caricamento del video… "+Math.min(99,Math.round(a/q*100))+"%"};
  ev.forEach(e=>v.addEventListener(e,ctl));const ti=setInterval(ctl,400),to=setTimeout(fine,12000);ctl()}
/* il giocatore non conosce l'elenco dei contenuti: chiede al server lo stato (che include ciò che è proiettato) */
async function projItemPlayer(o){try{const s=await api("/api/state");const loc=Object.values(s.locs||{});let it=s.projItem&&s.projItem.id===PROJ.id?s.projItem:null;
  if(!it)for(const l of loc)for(const x of l.items||[])if(x.id===PROJ.id)it=x;
  if(!it||!o.isConnected)return;const v=o.querySelector("video"),im=o.querySelector("img[data-pj-id]");
  if(it.k==="vid"){const c=o.querySelector(".pjovc");v.preload="auto";v.src=mURL(it.file);
    precarica(v,c,()=>{if(LONTANO)audioPieno(v,c);else v.play().catch(()=>tastoAvvio(v,c))})}else{v.remove();im.src=mURL(it.file);im.hidden=false}
  if(it.cap){const c=document.createElement("div");c.className="pjcap";c.textContent=it.cap;o.querySelector(".pjovc").appendChild(c)}}catch(e){}}
function projLabel(){if(!PROJ||PROJ.kind==="none")return"schermo nero";if(PROJ.kind==="map")return"mappa dei giocatori";
  if(PROJ.kind==="beast")return"immagine di "+(PROJ.cap||"PNG/mostro");
  if(PROJ.kind==="builtin")return"mappa locale di "+((LOCS.find(l=>l.k===PROJ.k)||{}).n||"");
  for(const k in CONTENT)for(const it of CONTENT[k].items||[])if(it.id===PROJ.id)return(it.k==="vid"?"video «":"immagine «")+(it.cap||"")+"»";return"contenuto"}
function projStatus(){$("#pjs").textContent="📺 In proiezione: "+projLabel()}
function projVid(){if(!PROJ||PROJ.kind!=="item")return false;for(const k in CONTENT)for(const it of CONTENT[k].items||[])if(it.id===PROJ.id)return it.k==="vid";return false}
$("#prj").onclick=()=>{const o=dlgX(`<h2>📺 Proiezione</h2><p class="mut">In proiezione ora: <b>${esc(projLabel())}</b>${PROJ&&PROJ.kind!=="none"&&PROJ.dove?" · "+esc(DOVE[PROJ.dove]||""):""}.</p>
  <fieldset class="pjdove"><legend>Dove proiettare</legend>${Object.entries(DOVE).map(([k,t])=>`<label><input type="radio" name="pjd" value="${k}"${PROJD===k?" checked":""}> ${t}</label>`).join("")}</fieldset>
  <div class="mut pjinfo"></div>
  <p class="pjapri"><button data-a="open">Apri la finestra di proiezione</button></p>
  <p class="pjschermi mut" aria-live="polite"></p>
  <details class="pjdiag"><summary>🔍 Diagnosi del proiettore (da copiare se non si apre sul proiettore)</summary><pre class="pjdiagt"></pre><button type="button" data-s="copia">📋 Copia</button></details>
  ${pannelloAuto()}
  <p><button data-a="map">🗺 Proietta la mappa (vista dei giocatori)</button> <button data-a="none">⬛ Ferma la proiezione</button></p>
  ${projVid()?`<p>Video: <button data-c="play">▶ Play</button> <button data-c="pause">⏸ Pausa</button> <button data-c="restart">⟲ Dall’inizio</button> <button data-c="mute">🔇/🔊</button></p>`:""}
  ${pannelloRegolazioni()}
  <p class="mut">Per proiettare un’immagine o un video: apri la scheda di una località (tasto destro sul segnalino) e premi «📺 Proietta»; per un PNG o un mostro, «Proietta» nel bestiario. Ciò che proietti diventa visibile anche ai giocatori di questo gruppo.</p><p style="text-align:right"><button data-a="x">Chiudi</button></p>`);
  const INFO={giocatori:"Il contenuto compare a tutto schermo sui telefoni e tablet dei giocatori che hanno aperto la Mappa: <b>senza audio</b> per chi è al tavolo, <b>audio al massimo</b> per chi gioca a distanza; su questo dispositivo con l’audio. Si chiude con «Chiudi per tutti».",
    schermo:"1. Collega il proiettore o la TV con il cavo (HDMI) e premi <b>Win + P → Estendi</b> (con «Duplica» il browser vede un solo schermo). 2. «Apri la finestra di proiezione»: con Chrome o Edge, dato una volta il permesso degli altri schermi, si apre da sola sul proiettore. 3. Premi Invio: va a tutto schermo. L’audio esce dal secondo schermo.",
    wowlink:"1. Accendi il Wowlink e scegli <b>Screen Mirroring</b> (o Miracast). 2. Sul PC premi <b>Win + K</b>, scegli il proiettore e poi <b>Cambia modalità di proiezione → Estendi</b>. 3. «Apri la finestra di proiezione»: con il permesso degli altri schermi si apre da sola sul proiettore; premi Invio. Se il proiettore non compare: PC e proiettore vicini, Wi-Fi del PC acceso."};
  const agg=()=>{o.querySelector(".pjinfo").innerHTML=INFO[PROJD];o.querySelector(".pjapri").hidden=PROJD==="giocatori";o.querySelector(".pjreg").hidden=PROJD==="giocatori"};agg();
  collegaRegolazioni(o);statoSchermi(o);
  o.addEventListener("change",e=>{if(e.target.name==="pjd"){PROJD=e.target.value;try{localStorage.setItem("gasterbless-proj-dove",PROJD)}catch(_){}agg();
    if(PROJ&&PROJ.kind&&PROJ.kind!=="none"&&PROJ.dove!==PROJD)project({kind:PROJ.kind,k:PROJ.k,id:PROJ.id,file:PROJ.file,cap:PROJ.cap})}});
  o.onclick=async e=>{if(e.target===o){o.remove();return}if(e.target.closest("[data-r]"))return;
    const cc=e.target.closest("[data-cmd]");if(cc){const t=cc.previousElementSibling.textContent;try{await navigator.clipboard.writeText(t);toast("Comando copiato: incollalo in PowerShell (amministratore).",3500)}catch(_){const r=document.createRange();r.selectNodeContents(cc.previousElementSibling);const se=getSelection();se.removeAllRanges();se.addRange(r);toast("Testo selezionato: Ctrl+C per copiarlo.",4000)}return}
    const sb=e.target.closest("[data-s]");if(sb){if(sb.dataset.s==="copia"){const t=o.querySelector(".pjdiagt").textContent;try{await navigator.clipboard.writeText(t);toast("Diagnosi copiata: incollala nella chat.",3000)}catch(_){const r=document.createRange();r.selectNodeContents(o.querySelector(".pjdiagt"));const se=getSelection();se.removeAllRanges();se.addRange(r);toast("Testo selezionato: Ctrl+C per copiarlo.",4000)}return}
      if(sb.dataset.s==="permesso"){try{SCHERMI=await getScreenDetails()}catch(_){}}else if(PJW&&!PJW.closed){const os=schermoProiezione();if(os)portaSu(PJW,os)}statoSchermi(o);return}const b=e.target.closest("[data-a],[data-c]");if(!b)return;
    if(b.dataset.c){api("/api/projcmd",{c:b.dataset.c}).catch(er=>toast(er.message));return}
    const a=b.dataset.a;if(a==="x")o.remove();else if(a==="open")apriFinestraProiezione();
    else{await project({kind:a});o.remove()}}};
/* Stato degli schermi nella finestra «📺 Proiezione» (2026-11-12, portatile con il proiettore in HDMI): dice se il browser
   vede il proiettore come secondo schermo e, se manca, il permesso, con un tasto per chiederlo (un clic tutto suo, così la
   finestra di proiezione si apre poi subito sul proiettore). */
/* Tutto schermo senza Invio (richiesta dell'utente, 2026-11-13): i browser lo permettono solo ai siti indicati da un
   criterio di Windows (AutomaticFullscreenAllowedForUrls; AutoplayAllowlist per l'audio dei video). Qui i comandi
   PowerShell già scritti con l'indirizzo di questo Gaster Bless e per il browser in uso; li esegue il Master, come
   amministratore. La finestra di proiezione va poi a tutto schermo da sola (apriFinestraProiezione). */
function pannelloAuto(){const edge=/Edg\//.test(navigator.userAgent),nome=edge?"Edge":"Chrome",base=edge?"HKLM:\\SOFTWARE\\Policies\\Microsoft\\Edge":"HKLM:\\SOFTWARE\\Policies\\Google\\Chrome",o=location.origin;
  const cmd=c=>`<div class="pjcmd"><code>${esc(c)}</code><button type="button" data-cmd="1">📋 Copia</button></div>`;
  const per=chiave=>[`New-Item -Path "${base}\\${chiave}" -Force | Out-Null`,`New-ItemProperty -Path "${base}\\${chiave}" -Name "1" -Value "${o}" -PropertyType String -Force | Out-Null`];
  return `<details class="pjauto"><summary>⚙ Tutto schermo senza premere Invio (impostazione di ${nome})</summary>
  <p class="mut">${nome} manda a tutto schermo da solo solo i siti che Windows gli indica. Una volta sola, su questo PC: apri <b>PowerShell come amministratore</b> (tasto destro su Start → «Terminale (amministratore)»), copia e incolla un comando alla volta. Indirizzo usato: <b>${esc(o)}</b> (se apri Gaster Bless da un altro indirizzo, ripeti da lì).</p>
  <p><b>1. Tutto schermo automatico</b></p>${per("AutomaticFullscreenAllowedForUrls").map(cmd).join("")}
  <p><b>2. Audio dei video senza clic</b> (facoltativo)</p>${per("AutoplayAllowlist").map(cmd).join("")}
  <p class="mut">3. Chiudi e riapri ${nome}; controlla in <b>${edge?"edge":"chrome"}://policy</b> che le due voci ci siano con stato «OK». ${nome} scriverà «gestito dalla tua organizzazione»: è normale con queste impostazioni.</p>
  <p class="mut">Per annullare (sempre come amministratore):</p>${cmd(`Remove-Item -Path "${base}\\AutomaticFullscreenAllowedForUrls" -Recurse -Force`)}${cmd(`Remove-Item -Path "${base}\\AutoplayAllowlist" -Recurse -Force`)}</details>`}
/* Diagnosi (2026-11-12): ciò che il browser dice degli schermi, da copiare nella chat se la finestra non va sul proiettore */
async function diagnosiSchermi(){const r=[],ua=navigator.userAgent,b=/Edg\/(\d+)/.exec(ua)?"Edge "+RegExp.$1:/Chrome\/(\d+)/.exec(ua)?"Chrome "+RegExp.$1:/Firefox\/(\d+)/.exec(ua)?"Firefox "+RegExp.$1:ua.slice(0,60);
  r.push("Browser: "+b,"Pagina sicura: "+(window.isSecureContext?"sì":"NO")+" · dentro l'app: "+(window!==window.top?"sì":"no"));
  try{const fp=document.featurePolicy;if(fp)r.push("Regola del riquadro: window-management "+fp.allowsFeature("window-management")+", fullscreen "+fp.allowsFeature("fullscreen"))}catch(e){}
  r.push("Più schermi (screen.isExtended): "+(window.screen.isExtended===undefined?"non disponibile":window.screen.isExtended?"sì":"NO"),"Questo schermo: "+screen.width+"×"+screen.height+" · finestra in "+window.screenX+","+window.screenY);
  r.push("getScreenDetails: "+(window.getScreenDetails?"sì":"NO"),"Permesso altri schermi: "+((await permessoSchermi())||"sconosciuto"));
  if(SCHERMI)(SCHERMI.screens||[]).forEach((s,i)=>r.push("  schermo "+(i+1)+": "+(s.label||"")+" "+(s.width||s.availWidth)+"×"+(s.height||s.availHeight)+" da "+s.availLeft+","+s.availTop+(s.isPrimary?" · principale":"")+(s===SCHERMI.currentScreen?" · quello della mappa":"")));
  const pr=schermoProiezione();r.push("Schermo scelto per la proiezione: "+(pr?(pr.label||"")+" da "+pr.availLeft+","+pr.availTop:"nessuno"));
  try{r.push("Finestra di proiezione: "+(PJW&&!PJW.closed?"aperta in "+PJW.screenX+","+PJW.screenY+" ("+PJW.outerWidth+"×"+PJW.outerHeight+")":"non aperta da questa pagina"))}catch(e){r.push("Finestra di proiezione: non leggibile")}
  r.push("Versione: "+(document.querySelector('script[src*="carta.js"]')?"carta.js":"")+" · "+new Date().toLocaleString("it-IT"));
  return r.join("\n")}
async function statoSchermi(o){const p=o.querySelector(".pjschermi");if(!p)return;
  const dg=o.querySelector(".pjdiagt");if(dg)diagnosiSchermi().then(t=>{dg.textContent=t}).catch(()=>{});
  if(!window.getScreenDetails){p.textContent=window.screen.isExtended===false?"🖥 "+AIUTO_ESTENDI:"🖥 Con questo browser la finestra non si apre da sola sull’altro schermo: usa Chrome o Edge, oppure trascinala.";return}
  const perm=await permessoSchermi();
  if(perm==="granted"&&!SCHERMI)try{SCHERMI=await getScreenDetails()}catch(e){}
  const pr=schermoProiezione(),n=SCHERMI?(SCHERMI.screens||[]).length:0;
  if(perm!=="granted"){p.innerHTML=(window.screen.isExtended===false?"🖥 "+esc(AIUTO_ESTENDI)+"<br>":"")+"🖥 Per aprire la finestra direttamente sul proiettore il browser deve poter usare gli altri schermi. "+
      (perm==="denied"?"Il permesso è stato negato: icona a sinistra dell’indirizzo → «Gestione delle finestre» → Consenti.":"<button type=\"button\" data-s=\"permesso\">🖥 Consenti l’uso degli altri schermi</button>");return}
  if(!pr){p.textContent="🖥 "+AIUTO_ESTENDI;return}
  p.innerHTML="📽 Proiettore pronto: secondo schermo "+(pr.width||pr.availWidth)+"×"+(pr.height||pr.availHeight)+" ("+n+" schermi in tutto). La finestra si apre lì."+(PJW&&!PJW.closed?" <button type=\"button\" data-s=\"sposta\">📽 Porta la finestra sul proiettore</button>":"")}
/* Regolazioni del proiettore (proiezione pulita, 2026-11-12): salvate in questo browser (proiettore.js), la finestra di
   proiezione le applica subito. Valgono per il secondo schermo e il Wowlink, non per i dispositivi dei giocatori. */
const CURSORI=[["luce","Luminosità"],["ombre","Ombre (schiarisce le zone scure)"],["contrasto","Contrasto"],["colori","Colori (intensità)"],["calore","Calore (freddo ↔ caldo)"],["nitidezza","Nitidezza"],["margine","Margine ai bordi"]];
const valoreCursore=(k,v)=>(v>0&&["luce","contrasto","colori","calore"].includes(k)?"+":"")+v+(k==="margine"?"%":"");
function pannelloRegolazioni(){const r=BP.leggi();
  return `<details class="pjreg"><summary>🎛 Regolazioni del proiettore: nitidezza e colori</summary>
  <label class="pjr"><span>Profilo</span><select name="pjprof">${Object.entries(BP.PROFILI).map(([k,p])=>`<option value="${k}"${r.profilo===k?" selected":""}>${esc(p.nome)}</option>`).join("")}<option value="misura"${r.profilo==="misura"?" selected":""}>Su misura</option></select></label>
  ${CURSORI.map(([k,t])=>`<label class="pjr"><span>${t}</span><input type="range" name="pjr-${k}" min="${BP.LIMITI[k][0]}" max="${BP.LIMITI[k][1]}" step="1" value="${r[k]}"><output>${valoreCursore(k,r[k])}</output></label>`).join("")}
  <label class="pjr"><span>Adatta l’immagine</span><select name="pjadatta"><option value="intera"${r.adatta==="intera"?" selected":""}>Intera (bande nere se serve)</option><option value="riempi"${r.adatta==="riempi"?" selected":""}>Riempi lo schermo (taglia i bordi)</option></select></label>
  <label class="pjr"><span>Didascalie</span><span><input type="checkbox" name="pjdid"${r.didascalie?" checked":""}> scrivi il titolo sotto l’immagine</span></label>
  <p><button type="button" data-r="prova">${BP.prova()?"🧪 Togli la schermata di prova":"🧪 Schermata di prova"}</button> <button type="button" data-r="base">↺ Valori del profilo</button></p>
  <p class="mut">Si vedono subito sul proiettore. «Ombre» schiarisce le zone scure, che il proiettore impasta (le immagini di Gaster Bless sono cupe); «Nitidezza» rinforza i bordi, utile soprattutto quando un’immagine piccola viene ingrandita; «Margine ai bordi» se il proiettore taglia i lati. Valgono per mappa, immagini, video e video dei canti nella finestra di proiezione. La schermata di prova si toglie da sola chiudendo questa finestra.</p></details>`}
function collegaRegolazioni(o){const reg=o.querySelector(".pjreg");if(!reg)return;
  const mostra=r=>{reg.querySelector("[name=pjprof]").value=r.profilo;CURSORI.forEach(([k])=>{const i=reg.querySelector(`[name="pjr-${k}"]`);i.value=r[k];i.nextElementSibling.textContent=valoreCursore(k,r[k])});
    reg.querySelector("[name=pjadatta]").value=r.adatta;reg.querySelector("[name=pjdid]").checked=r.didascalie;
    reg.querySelector("[data-r=prova]").textContent=BP.prova()?"🧪 Togli la schermata di prova":"🧪 Schermata di prova"};
  reg.addEventListener("input",e=>{const n=e.target.name||"";let r=BP.leggi();
    if(n.startsWith("pjr-")){const k=n.slice(4);r[k]=+e.target.value;if(k!=="margine")r.profilo="misura"}
    else if(n==="pjprof")r=e.target.value==="misura"?Object.assign(r,{profilo:"misura"}):BP.conProfilo(r,e.target.value);
    else if(n==="pjadatta")r.adatta=e.target.value;else if(n==="pjdid")r.didascalie=e.target.checked;else return;
    mostra(BP.salva(r))});
  reg.addEventListener("click",e=>{const b=e.target.closest("[data-r]");if(!b)return;e.preventDefault();
    if(b.dataset.r==="prova")BP.impostaProva(!BP.prova());
    else{const r=BP.leggi();BP.salva(BP.conProfilo(r,BP.PROFILI[r.profilo]?r.profilo:"buio"))}
    mostra(BP.leggi())});
  // chiudendo la finestra, la schermata di prova sparisce dal proiettore
  const mo=new MutationObserver(()=>{if(!o.isConnected){mo.disconnect();if(BP.prova())BP.impostaProva(false)}});mo.observe(document.body,{childList:true})}
/* «📺 Proietta» su un’immagine o un video (scheda della località, bestiario): si sceglie ogni volta dove proiettare
   (richiesta dell’utente, 2026-11-06). Dispositivi dei giocatori: il video parte da solo sui loro telefoni (e chi non
   è sulla Mappa ci viene portato); secondo schermo o Wowlink: si apre la finestra di proiezione. */
/* Finestra di proiezione direttamente sul proiettore (richiesta dell'utente, 2026-11-12): la prima volta il browser chiede
   QUI, sul PC del Master, il permesso di usare gli altri schermi; da allora la finestra si apre sul proiettore (o sul
   secondo schermo), grande quanto lui. Se è già aperta non si riapre: non si ricarica e resta a tutto schermo. */
let SCHERMI=null,PJW=null;
async function permessoSchermi(){if(!navigator.permissions)return"";for(const name of["window-management","window-placement"]){try{return(await navigator.permissions.query({name})).state}catch(e){}}return""}
(async()=>{try{if(PJ||!window.getScreenDetails)return;if(await permessoSchermi()==="granted")SCHERMI=await getScreenDetails()}catch(e){}})();
function schermoProiezione(){if(!SCHERMI)return null;const l=SCHERMI.screens||[];return l.find(s=>s!==SCHERMI.currentScreen&&!s.isPrimary)||l.find(s=>s!==SCHERMI.currentScreen)||null}
const urlProiezione=()=>"/carta/?sessione="+encodeURIComponent(SID)+"#proj";
// apre la finestra (o ritrova quella già aperta, anche dopo aver ricaricato la mappa) sullo schermo «o»
function apriSu(o){let w=null;
  try{w=window.open("","gasterbless-proj",o?`popup,left=${o.availLeft},top=${o.availTop},width=${o.availWidth},height=${o.availHeight},fullscreen`:"popup,width=1280,height=720")}catch(e){}
  if(!w)return null;
  let gia=false;try{gia=w.location.pathname==="/carta/"&&w.location.hash==="#proj"&&new URLSearchParams(w.location.search).get("sessione")===SID}catch(e){}
  if(!gia)w.location.href=urlProiezione();
  if(o)portaSu(w,o); // anche una finestra ritrovata (magari aperta prima sullo schermo del PC) va sul proiettore
  return w}
// sposta la finestra sullo schermo «o» se non c'è già (serve il permesso degli altri schermi; a tutto schermo non si sposta)
function portaSu(w,o){try{const cx=w.screenX+(w.outerWidth||0)/2,cy=w.screenY+(w.outerHeight||0)/2;
  if(cx>=o.availLeft&&cx<o.availLeft+o.availWidth&&cy>=o.availTop&&cy<o.availTop+o.availHeight)return true;
  w.moveTo(o.availLeft,o.availTop);w.resizeTo(o.availWidth,o.availHeight);return true}catch(e){return false}}
// un solo schermo per il browser: il proiettore non è collegato o Windows lo «duplica» (portatile + HDMI: Win + P → Estendi)
const unoSchermo=()=>SCHERMI?(SCHERMI.screens||[]).length<2:window.screen.isExtended===false;
const AIUTO_ESTENDI="Il browser vede un solo schermo: il proiettore non è collegato oppure Windows lo «duplica». Premi Win + P e scegli «Estendi» (con il Wowlink: Win + K → proiettore → «Cambia modalità di proiezione» → «Estendi»), poi riapri la finestra.";
function dopoApertura(w,o,negato){PJW=w;
  if(!w){toast("Il browser ha bloccato la finestra: consenti i popup per questo sito (icona nella barra degli indirizzi).",6000);return}
  if(o)toast("📽 Finestra di proiezione aperta sul proiettore: premi Invio (o cliccaci dentro) per il tutto schermo.",6000);
  else if(unoSchermo())toast(AIUTO_ESTENDI,12000);
  else if(negato)toast("Il browser non ha il permesso di usare gli altri schermi: icona a sinistra dell’indirizzo → «Gestione delle finestre» → Consenti. Intanto: Win + Shift + → sposta la finestra sul proiettore, poi Invio.",11000);
  else if(!window.getScreenDetails)toast("Questo browser non sa aprire la finestra sull’altro schermo (serve Chrome o Edge): premi Win + Shift + → per spostarla sul proiettore, poi Invio.",10000);
  else toast("La finestra è su questo schermo: premi Win + Shift + → (freccia destra) per spostarla sul proiettore, poi Invio.",9000)}
function apriFinestraProiezione(){
  if(PJW&&!PJW.closed){try{if(!PJW.location.hash.includes("proj")||new URLSearchParams(PJW.location.search).get("sessione")!==SID)PJW.location.href=urlProiezione()}catch(e){}
    const o=schermoProiezione();if(o)portaSu(PJW,o);return PJW} // già aperta: al massimo si sposta sul proiettore
  if(SCHERMI||!window.getScreenDetails){const o=schermoProiezione(),w=apriSu(o);dopoApertura(w,o,false);return w} // subito, nel clic
  // primo uso: il permesso si chiede qui; se la risposta arriva tardi il browser blocca la finestra e basta un altro clic
  (async()=>{let negato=false;try{SCHERMI=await getScreenDetails()}catch(e){negato=true}
    const o=schermoProiezione(),w=apriSu(o);
    if(!w&&!negato){toast("Permesso registrato: premi di nuovo «Apri la finestra di proiezione» e si aprirà sul proiettore.",8000);return}
    dopoApertura(w,o,negato)})();
  return null}
function sceltaProiezione(p,cosa){const ora=PROJ&&PROJ.kind&&PROJ.kind!=="none";
  const SPIEGA={giocatori:"A tutto schermo sui telefoni e tablet dei giocatori, il video parte da solo (senza audio al tavolo, audio al massimo per chi gioca a distanza). Qui con l’audio.",
    schermo:"Nella finestra di proiezione, sul proiettore o sul monitor collegati con il cavo (Win + P → Estendi).",
    wowlink:"Nella finestra di proiezione, trasmessa al proiettore con Win + K (Screen Mirroring)."};
  const o=dlgX(`<h2>📺 Dove proiettare ${esc(cosa)}?</h2><div class="pjscelte">${Object.entries(DOVE).map(([k,t])=>`<button data-d="${k}" class="${k===PROJD?"on":""}"><b>${t}</b><small>${SPIEGA[k]}</small></button>`).join("")}</div>
    <p style="text-align:right">${ora?'<button data-d="none">⬛ Ferma la proiezione</button> ':''}<button data-d="x">Annulla</button></p>`);
  o.onclick=async e=>{if(e.target===o){o.remove();return}const b=e.target.closest("[data-d]");if(!b)return;const d=b.dataset.d;o.remove();if(d==="x")return;
    if(d==="none"){project({kind:"none"});return}
    PROJD=d;try{localStorage.setItem("gasterbless-proj-dove",d)}catch(_){}
    const aperta=PJW&&!PJW.closed;if(d!=="giocatori")apriFinestraProiezione(); // subito, nel clic: altrimenti il browser blocca la finestra
    await project(p);
    if(d==="giocatori")toast("📱 In proiezione sui dispositivi dei giocatori.",3500);
    else if(aperta)toast("📽 Sul proiettore.",2500)}}
function dlgX(h){const o=document.createElement("div");o.className="dlg x";o.innerHTML='<div class="bx sm">'+h+'</div>';document.body.appendChild(o);return o}

/* ---------- Invita giocatori ---------- */
$("#inv").onclick=()=>invite();
function invite(){const u=(ME.lan||[])[0],il=u&&GROUP?u+"/?invito="+GROUP.invite:"";
  const o=dlgX(`<h2>👥 Invita i giocatori</h2>${u?`<p>I giocatori, collegati al <b>Wi-Fi di casa</b>, inquadrano il QR o aprono il <b>link d’invito</b>: entrano subito, senza password.</p><img class="qr" alt="Codice QR del link d’invito" src="/api/qr.png?u=${encodeURIComponent(il)}"><div class="url" id="ilink">${esc(il)}</div><p style="text-align:center"><button data-a="cp">📋 Copia link</button> <button data-a="rg" title="Il vecchio link smette di funzionare">↻ Nuovo link</button></p>`:`<p class="err">Questo PC non sembra collegato a una rete locale: collegalo al Wi-Fi o alla rete di casa.</p>`}
  <p style="text-align:center;margin:4px 0">In alternativa: apri <b>${esc(u||"l’indirizzo del PC")}</b> e usa la password di <b>${esc(GROUP?GROUP.name:"")}</b>:</p><div class="pw">${esc(GROUP?GROUP.pw:"")}</div>
  ${(ME.lan||[]).length>1?`<p class="mut">Altri indirizzi di questo PC: ${ME.lan.slice(1).map(esc).join(" · ")}</p>`:""}
  <p class="mut"><b>Installare l’app sul telefono:</b> Android (Chrome) ⋮ → «Aggiungi a schermata Home»; iPhone/iPad (Safari) Condividi → «Aggiungi alla schermata Home». La prima volta Windows può chiedere di consentire l’accesso alla rete a Gaster Bless: scegli <b>Reti private</b>.</p><p style="text-align:right"><button data-a="x">Chiudi</button></p>`);
  o.onclick=async e=>{const b=e.target.closest("[data-a]");if(e.target===o||(b&&b.dataset.a==="x")){o.remove();return}if(!b)return;
    if(b.dataset.a==="cp"){try{await navigator.clipboard.writeText(il);toast("Link copiato: invialo ai giocatori (WhatsApp, Telegram…).")}catch(er){toast("Copia non riuscita: seleziona il link a mano.")}}
    if(b.dataset.a==="rg"){if(!b.dataset.c){b.dataset.c=1;b.textContent="Conferma: il vecchio link smetterà di funzionare";return}try{await api("/api/groups",{id:GROUP.id,regen:true});await refresh();o.remove();invite();toast("Nuovo link creato.")}catch(er){toast(er.message)}}}}

/* ---------- Nebbia di guerra ---------- */
const FT=(()=>{const c=document.createElement("canvas");c.width=809;c.height=524;const x=c.getContext("2d");x.fillStyle="#9ea8b0";x.fillRect(0,0,809,524);let s=7;const r=()=>(s=(s*16807)%2147483647)/2147483647;
  for(let i=0;i<170;i++){const px=r()*809,py=r()*524,rad=40+r()*110,l=r()<.55,g=x.createRadialGradient(px,py,0,px,py,rad);g.addColorStop(0,l?"rgba(235,240,244,.55)":"rgba(90,100,110,.45)");g.addColorStop(1,l?"rgba(235,240,244,0)":"rgba(90,100,110,0)");x.fillStyle=g;x.fillRect(px-rad,py-rad,rad*2,rad*2)}return c})();
// Prestazioni (P2): "timbro" con la sfumatura di una cella, creato una volta sola; stesso aspetto di prima.
const FR=CW*.5*1.9,FST=(()=>{const d=Math.ceil(FR*2),c=document.createElement("canvas");c.width=c.height=d;const x=c.getContext("2d"),g=x.createRadialGradient(FR,FR,0,FR,FR,FR);g.addColorStop(0,"rgba(0,0,0,1)");g.addColorStop(.55,"rgba(0,0,0,1)");g.addColorStop(1,"rgba(0,0,0,0)");x.fillStyle=g;x.fillRect(0,0,d,d);return c})();
function fogOn(c,op){const x=c.getContext("2d");x.globalCompositeOperation="source-over";x.clearRect(0,0,809,524);x.drawImage(FT,0,0);
  x.globalCompositeOperation="destination-out";
  for(let j=0;j<CR;j++)for(let i=0;i<CC;i++)if(fog[j*CC+i])x.drawImage(FST,(i+.5)*CW*.5-FR,(j+.5)*CW*.5-FR)
  c.style.opacity=op}
function drawFog(){fogOn($("#fog"),role==="master"?".5":".97");updMk()}
function paint(e){const r=map.getBoundingClientRect(),ci=(e.clientX-r.left)/r.width*W/CW,cj=(e.clientY-r.top)/r.height*H/CW,n=+$("#br").value,v=tool==="rev"?1:0;let ch=false;
  for(let j=Math.max(0,Math.floor(cj-n));j<=Math.min(CR-1,Math.ceil(cj+n));j++)for(let i=Math.max(0,Math.floor(ci-n));i<=Math.min(CC-1,Math.ceil(ci+n));i++)
    if(Math.hypot(i+.5-ci,j+.5-cj)<=n+.3&&fog[j*CC+i]!==v){fog[j*CC+i]=v;ch=true}
  if(ch&&!raf)raf=requestAnimationFrame(()=>{raf=0;drawFog()})}
async function saveFog(){const sv=$("#sv");if(saving){pend=true;return}saving=true;sv.textContent="Salvataggio…";
  // B7: si manda anche la base (la nebbia del server da cui si è partiti): il server applica solo le celle
  // cambiate su questo dispositivo, così due dispositivi del Master non si cancellano a vicenda.
  try{const inv=pack(),r=await api("/api/group",FOGB?{fog:inv,base:FOGB}:{fog:inv});
    if(r&&r.fog){FOGB=r.fog;const u=unpack(r.fog),pr=unpack(inv);let ch=false;for(let i=0;i<fog.length;i++)if(fog[i]===pr[i]&&fog[i]!==u[i]){fog[i]=u[i];ch=true}if(ch)drawFog()}
    sv.textContent="Visibile ai giocatori ✓"}catch(e){sv.textContent="Errore di salvataggio";toast(e.message)}
  saving=false;if(pend){pend=false;saveFog()}}
map.addEventListener("pointerdown",e=>{if(role!=="master"||tool==="route"||e.button)return;e.preventDefault();painting=true;try{map.setPointerCapture(e.pointerId)}catch(_){}paint(e)});
map.addEventListener("pointermove",e=>{if(painting)paint(e)});
const stopP=()=>{if(painting){painting=false;saveFog()}};map.addEventListener("pointerup",stopP);map.addEventListener("pointercancel",stopP);
function setTool(){const r=tool==="route";map.style.touchAction=r?"":"none";vp.style.touchAction=r?"":"none";map.style.cursor=r?"crosshair":"none";brushPrev()}
// Modalità nebbia: mentre è attiva si può solo svelare o coprire (più lo zoom); disattivandola tornano le altre funzioni
function setFog(on,mode){if(on&&role!=="master")return;tool=on?(mode||"rev"):"route";const b=document.body;b.classList.toggle("fogmode",on);$("#fogt").hidden=!on;
  $("#fogm").classList.toggle("on",on);$("#fogm").textContent=on?"🌫 Nebbia: attiva":"🌫 Rimuovi nebbia";$("#fogm").setAttribute("aria-pressed",on);
  $("#fgr").classList.toggle("on",tool==="rev");$("#fgc").classList.toggle("on",tool==="cov");
  if(on){closeSheet();document.querySelectorAll(".dlg.x,#lbx").forEach(d=>d.remove());drag.on=false}
  setTool()}
$("#fogm").onclick=()=>setFog(tool==="route");$("#fgr").onclick=()=>setFog(true,"rev");$("#fgc").onclick=()=>setFog(true,"cov");$("#fgx").onclick=()=>setFog(false);
document.addEventListener("keydown",e=>{if(e.key==="Escape"&&tool!=="route")setFog(false)});
// anteprima del pennello sulla mappa
let bpXY=null;
function brushPrev(e){const g=$("#ov");let c=$("#bpc");if(tool==="route"||!e&&!bpXY){if(c)c.remove();if(tool==="route")bpXY=null;return}
  if(e){const r=map.getBoundingClientRect();bpXY=[(e.clientX-r.left)/r.width*W,(e.clientY-r.top)/r.height*H]}
  if(!c)c=el("circle",{id:"bpc",fill:"rgba(159,208,255,.18)","stroke-width":2,"pointer-events":"none"},g);
  c.setAttribute("cx",bpXY[0]);c.setAttribute("cy",bpXY[1]);c.setAttribute("r",(+$("#br").value+.3)*CW);c.setAttribute("stroke",tool==="rev"?"#9fd0ff":"#d9a441")}
map.addEventListener("pointermove",e=>{if(tool!=="route")brushPrev(e)});map.addEventListener("pointerleave",()=>{const c=$("#bpc");if(c)c.remove();bpXY=null});
$("#br").addEventListener("input",()=>brushPrev());
$("#ra").onclick=()=>{fog.fill(1);drawFog();saveFog()};$("#ca").onclick=()=>{fog.fill(0);drawFog();saveFog()};

/* ---------- Scala (per gruppo) ---------- */
let mt=0;
function setScale(v,save){v=Math.min(10,Math.max(.05,+v||.25));miHex=v;if(document.activeElement!==$("#mh"))$("#mh").value=v;panel();
  if(save){clearTimeout(mt);mt=setTimeout(()=>api("/api/group",{miHex:v}).catch(e=>toast(e.message)),600)}}
$("#mh").oninput=()=>setScale($("#mh").value,true);$("#mh0").onclick=()=>{$("#mh").value=.25;setScale(.25,true)};

/* ---------- Inquadratura iniziale: riempie lo schermo (telefoni) e centra la zona esplorata ---------- */
let needFit=false;
function fitView(){needFit=false;const vw=vp.clientWidth,vh=vp.clientHeight;if(!vw||!vh)return;
  const z=Math.min(6,Math.max(1,vh/(vw*H/W)));zoom=z;map.style.width=z*100+"%";labels();
  let sx=0,sy=0,n=0;if(role==="player")for(let j=0;j<CR;j++)for(let i=0;i<CC;i++)if(fog[j*CC+i]){sx+=i;sy+=j;n++}
  const cx=n?(sx/n+.5)*CW/W:.5,cy=n?(sy/n+.5)*CW/H:.5,r=map.getBoundingClientRect();
  vp.scrollLeft=cx*r.width-vw/2;vp.scrollTop=cy*r.height-vh/2}
/* ---------- Stato dal server ---------- */
// aggiornamento dello stato: se ne arriva un altro mentre uno è in corso, lo ripete alla fine (nessun dato perso)
let rf=null,rfAgain=false,RSIG="",FOGB="";
function refresh(){if(rf){rfAgain=true;return rf}rf=(async()=>{do{rfAgain=false;await refreshOnce()}while(rfAgain)})().finally(()=>{rf=null});return rf}
async function refreshOnce(){{try{const s=await api("/api/state");
    // Prestazioni (P3): se lo stato è identico all'ultimo non si ridisegna nulla; la nebbia solo se è cambiata.
    const sig=JSON.stringify(s);if(sig===RSIG&&!needFit)return;RSIG=sig;const fog0=pack();
    if(role==="master"){CONTENT=s.content||{};GROUP=s.group;G=s.g||null;if(!GROUP){showSessions();return}
      $("#rb").textContent="· Master · "+GROUP.name;if(!painting&&!saving&&!pend){fog=unpack(G.fog);FOGB=G.fog}setScale(G.miHex,false);PROJ=G.proj||{kind:"none"};PARTY=G.party||null;projStatus();projOverlay()}
    else{PST=s;PARTY=s.party||null;fog=unpack(s.fog);setScale(s.miHex,false);bgImg(s.fog);PROJ=s.proj||{kind:"none"};projOverlay()}
    geloAgg();if(!DIARIO)diarioAgg();setTimeout(()=>{pjCaricata=true},0);
    if(pack()!==fog0||!drawFog.ok){drawFog.ok=1;drawFog()}markers();fillSel();preview();if(needFit)requestAnimationFrame(fitView);if(SH&&!(SH.o.contains(document.activeElement)&&/INPUT|TEXTAREA/.test(document.activeElement.tagName)))drawSheet()}
  catch(e){if(e.status!==401)toast("Server non raggiungibile: controlla che Gaster Bless sia avviato sul PC del Master.",4000)}}}
function listen(){if(es)es.close();es=new EventSource(compat("/api/events"));
  es.onmessage=e=>{let m;try{m=JSON.parse(e.data)}catch(_){return}
    if(m.t==="music"){MUS.state=m.s;MUS.cfg=m.c;musApply();return}
    if(m.t==="song"){songStart(m);return}if(m.t==="songend"){songEnd();return}if(m.t==="songs"){SONG.lock=m.lock||{};songUI();return}
    if(PJ){onProj(m);return}
    if(m.t==="viaggi"){diarioAgg();return}
    if(m.t==="state")refresh();else if(m.t==="session")boot();else if(m.t==="proj"){PROJ=m.p;if(role==="master")projStatus();projOverlay()}
    else if(m.t==="cmd"){const v=$("#pjov video");if(v){if(m.c==="play")v.play().catch(()=>{});else if(m.c==="pause")v.pause();else if(m.c==="restart"){v.currentTime=0;v.play().catch(()=>{})}else if(m.c==="mute"&&role==="master")v.muted=!v.muted}}
    else if(m.t==="show"&&role==="player")showToPlayer(m.items||[]);else if(m.t==="hide"&&role==="player"){const d=$("#shw");if(d)d.remove()}};
  es.onerror=()=>{if(es.readyState===2)setTimeout(boot,3000)}}
function lost(){if(!lost.on){lost.on=1;closeSheet();toast("La sessione è terminata o il Master l’ha cambiata.",4000);setTimeout(()=>{lost.on=0;boot()},800)}}

/* ---------- Ingresso: Master sceglie la sessione, giocatori con password ---------- */
function show(id){["gLoad","gPl","gMa"].forEach(x=>$("#"+x).hidden=x!==id);document.body.classList.remove("in","ma","pl")}
async function boot(){
  if(!SID){show("gLoad");$("#gLoad p").textContent="Apri la mappa dalla sezione Mappa di Gaster Bless.";return}
  try{const io=await(await fetch("/api/v1/io",{credentials:"same-origin"})).json();CSRF=io.csrf||"";LONTANO=!!io.daFuori;if(!io.collegato)throw new Error("Accedi a Gaster Bless.")}catch(e){show("gLoad");$("#gLoad p").textContent=e.message||"Server non raggiungibile.";return}
  let me;try{me=await api("/api/me")}catch(e){show("gLoad");$("#gLoad p").textContent=e.message;return}
  ME=me;
  if(PJ){if(me.role!=="master"){document.body.innerHTML="<p style='color:#fff;padding:20px'>La proiezione si apre solo dal PC del Master.</p>";return}document.body.className="pj";listen();startProj();return}
  enter(me.role==="master"?"master":"player")}
$("#fLogin").onsubmit=async e=>{e.preventDefault();$("#lerr").textContent="";try{await api("/api/login",{pw:$("#lpw").value});$("#lpw").value="";boot()}catch(er){$("#lerr").textContent=er.message}};
/* Immagine della mappa (dal ramo main-ke8sy4): il Master la vede completa; il giocatore riceve dal server una
   copia con le zone sotto la nebbia del suo gruppo coperte (nessuno spoiler aprendo l'indirizzo dell'immagine).
   L'indirizzo cambia con la nebbia, così il browser chiede la versione nuova. */
let BGV="";
function bgImg(fogStr){if(role==="master"){if(BGV!=="m"){BGV="m";img.src="img/gasterbless.webp"}return}
  let h=5381;const t=fogStr||"";for(let i=0;i<t.length;i++)h=((h<<5)+h+t.charCodeAt(i))|0;const v=(h>>>0).toString(36);
  if(BGV!==v){BGV=v;img.src=compat("/api/immagine")+"&v="+v}}
function enter(r){role=r;CONSULTA=r==="player"&&SID.startsWith("g:");
  if(CONSULTA&&!$("#consulta")){const d=document.createElement("div");d.id="consulta";d.className="gkb";d.setAttribute("role","status");
    d.textContent="👁 Consultazione: fuori dalle sessioni la mappa si può solo guardare (luoghi svelati, percorsi «Da … a …»). Il segnalino del gruppo si sposta durante la partita.";
    $("#mt").after(d)}document.body.classList.remove("ma","pl");document.body.classList.add("in",r==="master"?"ma":"pl"); /* senza cancellare le scelte (pannello nascosto, comandi aperti) */BGV="";if(r==="master")bgImg();if(r==="player")$("#rb").textContent="· Giocatore"+(ME.hosting?" · "+ME.hosting.name:"");
  setFog(false);songLoad();wps=[];legs=[];PVL=null;needFit=true;RSIG="";FOGB="";drawFog.ok=0;if(r==="master")musLoad();else{AU.forEach(a=>a.pause())}$("#pv").hidden=true;closeSheet();render();listen();refresh()}
// Gruppi e sessioni si gestiscono nella sezione Sessioni della nuova app.
async function showSessions(){show("gLoad");$("#gLoad p").textContent="Scegli la sessione dalla sezione Mappa di Gaster Bless.";}
$("#fGroup").onsubmit=async e=>{e.preventDefault();$("#gerr").textContent="";try{await api("/api/groups",{id:$("#gid").value,name:$("#gname").value,day:$("#gday").value,pw:$("#gpw").value});$("#fGroup").reset();$("#gid").value="";$("#gnew").open=false;showSessions()}catch(er){$("#gerr").textContent=er.message}};
$("#gcancel").onclick=()=>{$("#fGroup").reset();$("#gid").value="";$("#gnew").open=false};
$("#fMpw").onsubmit=async e=>{e.preventDefault();try{await api("/api/masterpw",{pw:$("#mpw").value});ME.hasMasterPw=!!$("#mpw").value.trim();$("#mpwok").textContent="Salvata ✓";$("#mpw").value=""}catch(er){$("#mpwok").textContent=er.message}};
$("#ses").onclick=()=>showSessions();
$("#ex").onclick=async()=>{try{await api("/api/logout",{})}catch(e){}if(es)es.close();role=null;boot()};

/* ---------- Finestra di proiezione ----------
   Proiezione pulita (richiesta dell'utente, 2026-11-12): a tutto schermo, senza menu, avvisi né puntatore; tutto ciò che
   si vede passa dalle regolazioni del proiettore (web/carta/proiettore.js), che il Master cambia in «📺 Proiezione» e che
   qui si applicano subito. */
let pjAudio=false,pjAuto=false,pjVista=null,pjProva=null,pjQuieto=0,pjSveglia=null,pjAvvio=0;
async function schermoIntero(){try{
    // se la finestra è già sul proiettore (aperta lì dalla mappa) basta il tutto schermo sul suo schermo: il permesso degli
    // altri schermi si chiede solo se la finestra è finita sullo schermo del PC e il permesso non c'è ancora
    const perm="getScreenDetails" in window?await permessoSchermi():"denied";
    if(perm==="granted"||perm!=="denied"&&window.screen.isExtended){const sd=await getScreenDetails();const o=sd.screens.find(s=>s!==sd.currentScreen&&!s.isPrimary)||sd.screens.find(s=>s!==sd.currentScreen);
      if(o&&sd.currentScreen&&sd.currentScreen.isPrimary){const f=$("#pjfs");if(f)f.textContent="⛶ Tutto schermo sul proiettore"}
      // già sul secondo schermo (aperta lì dalla mappa): resta su questo
      await document.documentElement.requestFullscreen(o&&sd.currentScreen&&sd.currentScreen.isPrimary?{screen:o}:{})}else await document.documentElement.requestFullscreen()}
  catch(e){try{await document.documentElement.requestFullscreen()}catch(_){}}}
async function sveglia(){try{if(navigator.wakeLock&&document.visibilityState==="visible"&&!pjSveglia){pjSveglia=await navigator.wakeLock.request("screen");pjSveglia.addEventListener("release",()=>{pjSveglia=null})}}catch(e){}}
function pjRegola(){const r=BP.leggi(),pj=$("#pj");pj.style.setProperty("--pjmar",r.margine+"%");pj.classList.toggle("pj-senzadid",!r.didascalie);
  document.querySelectorAll("#pjscena img").forEach(si=>{si.style.filter=BP.filtroCSS(r)+(si.classList.contains("sfuma")?"":" drop-shadow(0 12px 28px rgba(0,0,0,.65))")});pjAdattaScena();
  if(pjVista)pjVista.aggiorna();pjMostraProva()}
function pjMostraProva(){if(pjProva){pjProva.vista.ferma();pjProva.el.remove();pjProva=null}if(!BP.prova())return;
  const d=document.createElement("div");d.id="pjprova";$("#pj").insertBefore(d,$("#pjgo"));
  const dpr=Math.min(window.devicePixelRatio||1,3),c=BP.schermataProva(Math.max(640,Math.round(d.clientWidth*dpr)),Math.max(360,Math.round(d.clientHeight*dpr)));
  d.appendChild(c);pjProva={el:d,vista:BP.mostra(d,c,{modo:"esatta"})}}
function pjSorgente(el,opz){if(pjVista)pjVista.ferma();pjVista=BP.mostra(el.parentNode,el,opz)}
async function startProj(){document.title="Gaster Bless · proiezione";
  $("#pjgo").onclick=async()=>{$("#pjgo").remove();pjAudio=true;pjAvvio=Date.now();await schermoIntero();sveglia();
    if(!document.fullscreenElement){const f=$("#pjfs");if(f)f.hidden=false} // non riuscito (per esempio dopo il permesso): basta un altro clic
    const v=$("#pj video");if(v){v.muted=false;v.play().catch(()=>{})}};
  // la finestra appena aperta dalla mappa ha la tastiera: Invio (o un altro tasto) vale come il clic, senza spostare il mouse
  document.addEventListener("keydown",e=>{if(e.key==="Escape"||!$("#pjgo"))return;e.preventDefault();$("#pjgo").click()});
  // con il «tutto schermo automatico» consentito dal browser per Gaster Bless (criterio di Chrome o Edge) parte da sola, senza
  // il riquadro d'avvio: l'audio prova a partire (serve anche l'autoplay consentito), altrimenti al primo tasto o clic
  try{const st=await navigator.permissions.query({name:"fullscreen",allowWithoutGesture:true});
    if(st.state==="granted"){await document.documentElement.requestFullscreen();pjAuto=true;const g=$("#pjgo");if(g)g.remove();sveglia();
      const sblocca=()=>{if(pjAudio)return;pjAudio=true;pjAvvio=Date.now();document.querySelectorAll("#pj video").forEach(v=>{v.muted=false;v.play().catch(()=>{})})};
      document.addEventListener("keydown",sblocca,{once:true});document.addEventListener("pointerdown",sblocca,{once:true})}}catch(e){}
  // doppio clic: entra o esce dal tutto schermo; tasto discreto quando non è a tutto schermo (solo muovendo il mouse)
  $("#pj").addEventListener("dblclick",()=>{if(!pjAudio&&!pjAuto||Date.now()-pjAvvio<1500)return;/* il doppio clic d'avvio non esce subito */if(document.fullscreenElement)document.exitFullscreen().catch(()=>{});else schermoIntero()});
  const fs=document.createElement("button");fs.id="pjfs";fs.type="button";fs.textContent="⛶ Tutto schermo";fs.hidden=true;fs.onclick=e=>{e.stopPropagation();schermoIntero()};$("#pj").appendChild(fs);
  document.addEventListener("fullscreenchange",()=>{fs.hidden=!pjAudio&&!pjAuto||!!document.fullscreenElement});
  // puntatore nascosto dopo 2 secondi fermo
  const muovi=()=>{document.body.classList.remove("pj-quieto");clearTimeout(pjQuieto);pjQuieto=setTimeout(()=>document.body.classList.add("pj-quieto"),2000)};
  document.addEventListener("mousemove",muovi);muovi();
  document.addEventListener("visibilitychange",()=>{if(pjAudio)sveglia()});
  window.addEventListener("storage",e=>{if(e.key===BP.CHIAVE||e.key===BP.CHIAVE_PROVA||e.key===null)pjRegola()});
  window.addEventListener("resize",()=>{if(pjProva)pjMostraProva();pjAdattaScena()});
  pjRegola();
  try{PROJ=await api("/api/proj")}catch(e){}drawProj();musLoad();songLoad();pjScenaIniziale()}
/* «🎭 PNG in scena» anche sul proiettore (richiesta dell'utente, 2026-11-12): grande, su un lato, senza sfondo, sopra ciò
   che si proietta; ingrandito anche se il file è piccolo, con i colori delle regolazioni del proiettore. */
let pjScenaK="";
/* più PNG in scena insieme (2026-11-13): stessa disposizione di web/ui/scena.js (lati sinistra, centro, destra; in fila senza coprirsi) */
function pjDisposizione(lati,W,H){const n=lati.length,w=Math.min(W*.58,W*.98/n),h=H*.94*(n<=2?1:Math.max(.74,1-.07*(n-2))),g={sinistra:[],centro:[],destra:[]},o=[];
  lati.forEach((l,i)=>g[l].push(i));g.sinistra.forEach((i,k)=>{o[i]=k*w});g.destra.forEach((i,k)=>{o[i]=W-(g.destra.length-k)*w});
  const fs=g.sinistra.length*w,id=W-g.destra.length*w,cw=g.centro.length*w,c0=Math.min(Math.max((W-cw)/2,fs),id-cw);g.centro.forEach((i,k)=>{o[i]=c0+k*w});return{o,w,h}}
function pjAdattaScena(){const box=$("#pjscena");if(!box)return;const it=[...box.querySelectorAll(".pjsc")];if(!it.length)return;
  const d=pjDisposizione(it.map(x=>x.dataset.lato),box.clientWidth,box.clientHeight);
  it.forEach((x,i)=>{x.style.left=Math.round(d.o[i])+"px";x.style.width=Math.round(d.w)+"px";x.style.height=Math.round(d.h)+"px";const im=x.querySelector("img");if(!im||!im.naturalWidth)return;
    const k=Math.min(d.w/im.naturalWidth,d.h/im.naturalHeight);im.style.width=Math.round(im.naturalWidth*k)+"px";im.style.height=Math.round(im.naturalHeight*k)+"px";im.classList.add("pronta")})}
function pjScena(l){l=Array.isArray(l)?l:(l?[l]:[]);const ks=l.map(sc=>sc.voce+":"+sc.quando+":"+sc.lato+":"+sc.nome),k=ks.join("|");if(k===pjScenaK&&(!l.length||$("#pjscena")))return;pjScenaK=k;
  let box=$("#pjscena");if(!l.length){if(box)box.remove();return}
  if(!box){box=document.createElement("div");box.id="pjscena";$("#pj").insertBefore(box,$("#pjprova")||$("#pjgo"))}
  box.querySelectorAll(".pjsc").forEach(x=>{if(!ks.includes(x.dataset.k))x.remove()});const pres=new Map([...box.querySelectorAll(".pjsc")].map(x=>[x.dataset.k,x]));let dopo=null;
  l.forEach((sc,i)=>{let x=pres.get(ks[i]);if(!x){const lato=sc.lato==="sinistra"||sc.lato==="centro"?sc.lato:"destra",jpg=/\.jpe?g$/i.test(sc.file||"");x=document.createElement("div");x.className="pjsc "+lato;x.dataset.k=ks[i];x.dataset.lato=lato;
    const im=document.createElement("img");im.alt=sc.nome||"";im.src=sc.url;if(jpg)im.className="sfuma";im.style.filter=BP.filtroCSS(BP.leggi())+(jpg?"":" drop-shadow(0 12px 28px rgba(0,0,0,.65))");
    im.addEventListener("load",pjAdattaScena);x.appendChild(im);if(sc.nome){const n=document.createElement("div");n.className="pjscnome";n.textContent=sc.nome;x.appendChild(n)}
    if(dopo)dopo.after(x);else box.prepend(x)}dopo=x});
  pjAdattaScena()}
async function pjScenaIniziale(){try{const s=await api("/api/state"),g=(s.group||{}).id;
  window.__pjGruppo=g;window.dispatchEvent(new CustomEvent("gasterbless:pj-gruppo",{detail:g}));/* lettura dei tarocchi in corso */const r=await fetch("/api/v1/scena",{credentials:"same-origin"});const l=r.ok?await r.json():[];
  pjScena(Array.isArray(l)?l.filter(x=>x.gruppo===g):[])}catch(e){}}
async function onProj(m){if(m.t==="scena"){pjScena(Array.isArray(m.scene)?m.scene:(m.attiva?m.scena:null));return}
  if(m.t==="tarocchi"){window.dispatchEvent(new CustomEvent("gasterbless:tarocchi",{detail:m.lettura}));return}/* tarocchi di la veggente (tarocchi_proiezione.js) */if(m.t==="proj"){PROJ=m.p;drawProj()}else if(m.t==="state"&&PROJ.kind==="map")drawProj();
  else if(m.t==="cmd"){const v=$("#pj video");if(!v)return;if(m.c==="play")v.play().catch(()=>{});else if(m.c==="pause")v.pause();else if(m.c==="restart"){v.currentTime=0;v.play().catch(()=>{})}else if(m.c==="mute")v.muted=!v.muted}}
let pjSeq=0;
async function drawProj(){const my=++pjSeq;let s;try{s=await api("/api/state")}catch(e){return}if(my!==pjSeq)return;
  const pj=$("#pj"),gobtn=$("#pjgo");if(pjVista){pjVista.ferma();pjVista=null}
  pj.querySelectorAll(":scope>:not(#pjgo):not(#pjfs):not(#pjprova):not(#songv)").forEach(n=>n.remove());CONTENT=s.content||{};const g=s.g||{};
  // palco: tutto lo schermo meno il «Margine ai bordi» delle regolazioni
  const pv=document.createElement("div");pv.id="pjv";pj.insertBefore(pv,pj.firstChild);
  const add=h=>{const d=document.createElement("div");d.innerHTML=h;while(d.firstChild)pv.appendChild(d.firstChild)};
  const cap=t=>{if(!t)return;add(`<div id="pjcap"></div>`);$("#pjcap").textContent=t};
  if(PROJ.kind==="map"){fog=unpack(g.fog);add(`<div id="pjm"><img src="img/gasterbless.webp" alt=""><canvas id="pjf" width="809" height="524"></canvas><svg id="pjsv" viewBox="0 0 1618 1048"></svg></div>`);
    // «pjsv»: prima era «pjs», lo stesso id della scritta «In proiezione» della barra, e i segni finivano lì (corretto 2026-11-12)
    fogOn($("#pjf"),"1");const sv=$("#pjsv");LOCS.forEach(l=>{if(!cell(l.x,l.y))return;el("circle",{cx:l.x,cy:l.y,r:l.mj?13:11,fill:l.mj?"rgba(217,164,65,.35)":"rgba(95,211,200,.25)",stroke:l.mj?"#d9a441":"#5fd3c8","stroke-width":l.mj?3:2},sv);
      if((g.pubNames||{})[l.k]){const t=el("text",{x:l.x,y:l.y-18,"text-anchor":"middle",class:"nm"},sv);t.textContent=l.n}});
    // mappa e nebbia in un'unica immagine, regolata per il proiettore; nomi e segni restano sopra, vettoriali e nitidi
    const bi=$("#pjm img");try{await bi.decode()}catch(e){}if(my!==pjSeq)return;
    const cmp=document.createElement("canvas");cmp.width=W;cmp.height=H;const cx=cmp.getContext("2d");
    if(bi.naturalWidth)cx.drawImage(bi,0,0,W,H);cx.drawImage($("#pjf"),0,0,W,H);
    bi.classList.add("pjsorgente");$("#pjf").classList.add("pjsorgente");$("#pjm").appendChild(cmp);cmp.className="pjcomp";
    pjSorgente(cmp,{modo:"esatta"})}
  else if(PROJ.kind==="builtin"){add(`<img class="it" src="img/lm/${esc(PROJ.k)}.webp" alt="">`);pjSorgente(pv.querySelector("img"))}
  else if(PROJ.kind==="beast"){add(`<img class="it" src="${esc(mURL(PROJ.file))}" alt="">`);pjSorgente(pv.querySelector("img"));cap(PROJ.cap)}
  else if(PROJ.kind==="item"){let it=null;for(const k in CONTENT)for(const x of CONTENT[k].items||[])if(x.id===PROJ.id)it=x;
    if(it&&it.k==="vid"){add(`<video src="${esc(mURL(it.file))}" playsinline autoplay preload="auto" ${pjAudio||pjAuto?"":"muted"}></video>`);const v=pv.querySelector("video");pjSorgente(v);
      if(pjAuto&&!pjAudio)v.play().catch(()=>{v.muted=true;v.play().catch(()=>{})})} // autoplay con l'audio non consentito: parte muto
    else if(it){add(`<img class="it" src="${esc(mURL(it.file))}" alt="">`);pjSorgente(pv.querySelector("img"))}
    if(it)cap(it.cap)}
  else add(`<div id="pjt">GASTER BLESS</div>`)}


/* ---------- Bestiario: PNG e mostri (Master), immagini mostrabili ai giocatori ---------- */
const MAXB=300;let BEST=null,BOV={entries:{},extras:[]},BSEL=new Set(),BF="tutti",BQ="";
async function loadBest(){if(!BEST){const [j,c]=await Promise.all([fetch("bestiario.json").then(x=>x.json()),fetch("mostri.json").then(x=>x.ok?x.json():{voci:[]}).catch(()=>({voci:[]}))]);j.voci=j.voci.concat(c.voci||[]);BEST=j}try{BOV=await api("/api/best")}catch(e){}if(!BOV.entries)BOV.entries={};if(!BOV.extras)BOV.extras=[]}
/* le schede compilate nella sezione «PNG e mostri» dell'app ("sheet") prendono il posto di quelle di base */
function bAll(){const sh=id=>(BOV.entries[id]||{}).sheet;return BEST.voci.map(e=>sh(e.id)?Object.assign({},e,{scheda:sh(e.id)}):e).concat((BOV.extras||[]).map(x=>({id:x.id,nome:x.nome,cat:x.cat,ruolo:x.ruolo,fonte:"Personale",pag:[],scheda:sh(x.id),base:sh(x.id)?undefined:(x.base||undefined)})))}
const bById=id=>bAll().find(e=>e.id===id);
const bImg=id=>(BOV.entries[id]||{}).img;
const norm=t=>t.toLowerCase().normalize("NFKD").replace(/[̀-ͯ]/g,"");
function bSub(e){if(e.cat==="png")return(e.profilo||e.ruolo||"PNG")+(e.ruolo&&e.profilo?" · "+e.ruolo:"");const s=e.scheda;return s?`GS ${s.gs.split(" ")[0]} · ${s.tipo}`:(e.fonte==="App. D"?"Scheda nell'appendice D":e.fonte==="Personale"?(e.ruolo||"Voce personale"):"Scheda da compilare")}
$("#bst").onclick=async()=>{try{await loadBest()}catch(e){toast("Impossibile caricare il bestiario.");return}bestDlg()};
function bestDlg(){const o=dlgX(`<div class="sh"><h2>📖 PNG e mostri</h2><span class="mo-only">Solo Master</span></div>
  <div class="bar" style="margin:0 0 8px"><input id="bq" type="search" placeholder="Cerca per nome, ruolo o tipo…" style="flex:1;min-width:200px;font-size:15px" value="${esc(BQ)}">
  <div class="chips" id="bfc">${[["tutti","Tutti"],["png","PNG"],["mostro","Mostri"],["cat","Catalogo"],["img","Con immagine"],["sel","Selezionati"]].map(([k,t])=>`<button data-f="${k}" class="${BF===k?"on":""}">${t}</button>`).join("")}</div></div>
  <ul class="bl" id="blist" role="listbox" aria-multiselectable="true"></ul>
  <div class="bar"><b id="bn"></b><button data-a="clr">Deseleziona</button><span class="sp"></span><button data-a="man">📕 Manuali</button><button data-a="new">＋ Nuova voce</button><button data-a="go" id="bgo" style="background:var(--ac);color:var(--bg);font-size:15px;padding:6px 16px">Apri…</button><button data-a="x">Chiudi</button></div>
  <div id="bnew" class="ed" hidden><input id="nn" type="text" placeholder="Nome" maxlength="80"><select id="nc"><option value="png">PNG</option><option value="mostro">Mostro</option></select><input id="nr" type="text" placeholder="Ruolo o descrizione breve" maxlength="120"><button data-a="save">Aggiungi</button></div>
  <p class="mut" style="margin:8px 0 0">Clic per selezionare uno o più elementi, doppio clic per aprirne subito uno. ${esc(BEST.attribuzione.split(" Testi")[0])}</p>`);
  o.querySelector(".bx").classList.remove("sm");
  const list=()=>{const q=norm(BQ.trim());let v=bAll().filter(e=>(BF==="tutti"||(BF==="png"&&e.cat==="png")||(BF==="mostro"&&e.cat!=="png"&&e.fonte!=="Codex")||(BF==="cat"&&e.fonte==="Codex")||(BF==="img"&&bImg(e.id))||(BF==="sel"&&BSEL.has(e.id)))&&(!q||norm(e.nome+" "+(e.en||"")+" "+(e.ruolo||"")+" "+(e.profilo||"")+" "+(e.note||"")).includes(q)));
    v.sort((a,b)=>{const qa=q&&norm(a.nome).startsWith(q),qb=q&&norm(b.nome).startsWith(q);return(qb-qa)||a.nome.localeCompare(b.nome,"it")});
    const tot=v.length;v=v.slice(0,MAXB);
    $("#blist").innerHTML=v.length?v.map(e=>`<li data-id="${e.id}" class="${BSEL.has(e.id)?"on":""}" role="option" aria-selected="${BSEL.has(e.id)}"><input type="checkbox" tabindex="-1" ${BSEL.has(e.id)?"checked":""}>${bImg(e.id)?`<img class="tb" src="/media/${esc(bImg(e.id))}" alt="" loading="lazy">`:`<span class="tb">${e.cat==="png"?"👤":"🐺"}</span>`}<div style="flex:1;min-width:0"><b>${esc(e.nome)}</b><small class="mut">${esc(bSub(e))}</small></div><span class="tag ${e.fonte==="App. D"?"d":""}">${e.cat==="png"?"PNG":"Mostro"}${e.fonte==="App. D"?" · App. D":e.fonte==="Personale"?" · personale":e.fonte==="Codex"?" · catalogo":""}</span></li>`).join("")+(tot>MAXB?`<li class="mut">Altre ${tot-MAXB} voci: scrivi nella ricerca per trovarle.</li>`:""):`<li class="mut">Nessun risultato.</li>`;
    $("#bn").textContent=BSEL.size?BSEL.size+" selezionat"+(BSEL.size>1?"i":"o"):"Nessuna selezione";$("#bgo").disabled=!BSEL.size};
  list();setTimeout(()=>$("#bq").focus(),30);
  $("#bq").oninput=e=>{BQ=e.target.value;list()};
  $("#bq").onkeydown=e=>{if(e.key==="Enter"){const f=$("#blist li[data-id]");if(f){BSEL.add(f.dataset.id);list();choose(o)}}};
  $("#bfc").onclick=e=>{const b=e.target.closest("[data-f]");if(!b)return;BF=b.dataset.f;o.querySelectorAll("#bfc button").forEach(x=>x.classList.toggle("on",x===b));list()};
  const cnt=()=>{$("#bn").textContent=BSEL.size?BSEL.size+" selezionat"+(BSEL.size>1?"i":"o"):"Nessuna selezione";$("#bgo").disabled=!BSEL.size};
  $("#blist").onclick=e=>{const li=e.target.closest("li[data-id]");if(!li)return;const id=li.dataset.id,on=!BSEL.has(id);on?BSEL.add(id):BSEL.delete(id);
    li.classList.toggle("on",on);li.setAttribute("aria-selected",on);li.querySelector("input").checked=on;cnt()};
  $("#blist").ondblclick=e=>{const li=e.target.closest("li[data-id]");if(!li)return;BSEL.clear();BSEL.add(li.dataset.id);list();choose(o)};
  o.onclick=async e=>{if(e.target===o){o.remove();return}const b=e.target.closest("[data-a]");if(!b)return;const a=b.dataset.a;
    if(a==="x")o.remove();else if(a==="clr"){BSEL.clear();list()}else if(a==="go")choose(o);else if(a==="man"){o.remove();manualsDlg()}else if(a==="new"){$("#bnew").hidden=!$("#bnew").hidden;$("#nn").focus()}
    else if(a==="save"){try{const x=await api("/api/best",{new:{nome:$("#nn").value,cat:$("#nc").value,ruolo:$("#nr").value}});await loadBest();BSEL.add(x.id);$("#bnew").hidden=true;$("#nn").value=$("#nr").value="";list();toast("Voce aggiunta e selezionata.")}catch(er){toast(er.message)}}}}

/* ---------- Scheda ufficiale: i manuali PDF del Master, aperti alla pagina giusta ---------- */
const MANS=()=>BOV.manuals||[];
function pdfPage(m,book){return m.spread?m.refPdf+Math.floor(book/2)-Math.floor(m.refBook/2):m.refPdf+(book-m.refBook)}
// pagina del libro proposta: appendice D per le schede dell'avventura, prima citazione per gli altri PNG
function defPage(e){const cx=e.fonte==="Codex"&&(e.origine||"").match(/^bestiario, p\. (\d+)/);if(cx)return{tipo:"mm",page:+cx[1]};const r=(e.rif||"").match(/pag\. (\d+)/);if(r)return{tipo:"avventura",page:+r[1]};if(e.fonte==="App. D"&&e.pag&&e.pag.length)return{tipo:"avventura",page:e.pag[0]};if(e.cat==="png"&&e.pag&&e.pag.length)return{tipo:"avventura",page:e.pag[0]};return{tipo:"mm",page:0}}
// voce di cui aprire la scheda ufficiale: per un PNG con profilo del Manuale dei Mostri, la creatura base
function offTarget(e){if(e.cat==="png"&&e.base&&e.fonte==="MM")return bById(e.base);return e}
// dove aprire la scheda: pagina indicata dal Master > pagina nota dell'avventura > pagina trovata dall'app nel manuale
function offRef(e){const t=offTarget(e),ov=BOV.entries[t.id]||{},d=defPage(t),ms=MANS(),ix=x=>(x.index||{})[t.id];
  const m=ms.find(x=>x.id===ov.man)||ms.find(x=>x.tipo===d.tipo&&(ov.page||d.page||ix(x)))||ms.find(x=>ix(x))||ms.find(x=>x.tipo===d.tipo);
  if(!m)return null;if(ov.page)return{m,page:ov.page,t};if(d.page&&m.tipo===d.tipo)return{m,page:d.page,t};if(ix(m))return{m,pdf:ix(m),t};return null}
function offHTML(e){const t=offTarget(e),ov=BOV.entries[t.id]||{},d=defPage(t),r=offRef(e),ms=MANS();
  if(!ms.length)return`<div class="ed"><b>📕 Scheda ufficiale</b><span class="mut">Carica i tuoi manuali in PDF (Manuale dei Mostri, l'avventura…) per aprirli qui alla pagina giusta.</span><button data-a="mans">📕 Manuali</button></div>`;
  const sel=ov.man||(r&&r.m.id)||"";
  return`<div class="ed"><b>📕 Scheda ufficiale${t!==e?` <small class="mut">(${esc(t.nome)})</small>`:""}</b>${r?`<button data-a="off" style="background:var(--ac);color:var(--bg)">${r.page?`Apri pag. ${r.page}`:`Apri la scheda (pag. ${r.pdf} del PDF)`}</button>`:""}
    <label>Manuale <select id="offm" data-for="${t.id}">${ms.map(m=>`<option value="${m.id}"${m.id===sel?" selected":""}>${esc(m.nome)}</option>`).join("")}</select></label>
    <label>Pagina del libro <input id="offp" type="number" min="1" max="2000" value="${ov.page||d.page||""}" placeholder="?"></label><button data-a="offsave">Salva</button>
    ${!ov.page&&d.page?`<span class="mut">Pagina proposta dall'elenco dell'avventura.</span>`:""}${r&&r.pdf?`<span class="mut">Pagina trovata dall'app nel manuale.</span>`:""}${!r?`<span class="mut">${ms.some(m=>m.tipo===d.tipo&&!m.indexed)?"Premi «🔎 Cerca le schede» in «📕 Manuali», oppure indica la pagina.":"Scheda non trovata nel manuale: indica la pagina."}</span>`:""}</div>`}
// Visualizzatore PDF integrato (PDF.js): non dipende dalle impostazioni del browser sui PDF
let pdfLib=null;const pdfDocs={};
function loadPdfLib(){if(pdfLib)return Promise.resolve(pdfLib);return new Promise((res,rej)=>{const sc=document.createElement("script");sc.src="/vendor/pdfjs/pdf.min.js";
  sc.onload=()=>{pdfLib=window.pdfjsLib;pdfLib.GlobalWorkerOptions.workerSrc="/vendor/pdfjs/pdf.worker.min.js";res(pdfLib)};sc.onerror=()=>rej(new Error("visualizzatore PDF non disponibile"));document.head.appendChild(sc)})}
async function pdfDoc(m){if(!pdfDocs[m.file]){const L=await loadPdfLib();
    pdfDocs[m.file]=L.getDocument({url:"/manuali/"+m.file,isEvalSupported:false,disableAutoFetch:true,rangeChunkSize:262144,standardFontDataUrl:"/vendor/pdfjs/standard_fonts/",withCredentials:true}).promise.catch(e=>{delete pdfDocs[m.file];throw e})}
  return pdfDocs[m.file]}
function openManual(m,book,title,pdfp,eid){let pg=Math.max(1,pdfp||pdfPage(m,book)),scale=0,doc=null,busy=0;const url=`/manuali/${m.file}#page=${pg}`;
  const o=dlgX(`<div class="sh"><h2>📕 ${esc(title)}</h2><span class="mo-only">Solo Master</span><div><a href="${url}" target="_blank" rel="noopener">Apri nel browser ↗</a> <button data-a="x">Chiudi</button></div></div>
    <div class="bar" style="margin:0 0 6px"><button data-a="pp" title="Pagina precedente">‹</button><label>Pagina PDF <input id="pvn" type="number" min="1" value="${pg}" style="width:70px"></label><span class="mut" id="pvt"></span><button data-a="nx" title="Pagina successiva">›</button>
      <button data-a="zo">−</button><button data-a="fit">Adatta</button><button data-a="zi">+</button>${eid?`<button data-a="crop" id="pvcrop" title="Trascina un rettangolo sull'illustrazione: diventa l'immagine di questa voce">✂ Ritaglia immagine</button>`:""}<span class="mut">${esc(m.nome)}${pdfp?" · pagina trovata dall'app":" · pagina del libro "+book}</span></div>
    <div id="pvw" style="height:72vh;overflow:auto;background:#525659;border-radius:6px;text-align:center"><p id="pvs" style="color:#eee;padding:30px">Apertura del manuale…</p><div id="pvbox" style="position:relative;display:inline-block"><canvas id="pvc" style="display:none;margin:8px auto;box-shadow:0 2px 10px #0008"></canvas><div id="pvsel" hidden style="position:absolute;border:2px dashed #ffd24a;background:#ffd24a22;pointer-events:none"></div></div></div>
    <div class="bar" id="pvcb" hidden style="margin:6px 0 0"><span class="mut">Trascina sulla pagina per scegliere l'illustrazione.</span><span class="sp"></span><button data-a="cropok" id="pvok" disabled style="background:var(--ac);color:var(--bg)">Usa come immagine</button><button data-a="cropno">Annulla</button></div>
    <p class="mut" style="margin:6px 0 0">Se la pagina non è quella giusta, correggi la calibrazione in «📕 Manuali».</p>`);
  o.querySelector(".bx").classList.remove("sm");
  const draw=async()=>{if(!doc||busy)return;busy=1;try{pg=Math.min(Math.max(1,pg),doc.numPages);$("#pvn").value=pg;$("#pvt").textContent="di "+doc.numPages;
      const page=await doc.getPage(pg),w=$("#pvw").clientWidth-20,v0=page.getViewport({scale:1}),sc=scale||Math.max(.2,w/v0.width),dpr=window.devicePixelRatio||1,v=page.getViewport({scale:sc*dpr}),c=$("#pvc");
      c.width=v.width;c.height=v.height;c.style.width=(v.width/dpr)+"px";c.style.height=(v.height/dpr)+"px";await page.render({canvasContext:c.getContext("2d"),viewport:v}).promise;
      c.style.display="block";$("#pvs").style.display="none";scale=sc}catch(e){$("#pvs").style.display="block";$("#pvs").textContent="Impossibile mostrare la pagina: "+e.message}finally{busy=0}};
  pdfDoc(m).then(d=>{doc=d;scale=0;draw()}).catch(e=>{$("#pvs").textContent="Impossibile aprire il PDF ("+(e.message||e)+"). Prova «Apri nel browser» o ricarica il manuale in «📕 Manuali»."});
  o.onclick=e=>{if(e.target===o){o.remove();return}const b=e.target.closest("[data-a]");if(!b)return;const a=b.dataset.a;
    if(a==="crop"){cropOn=!cropOn;$("#pvcb").hidden=!cropOn;$("#pvc").style.cursor=cropOn?"crosshair":"";if(!cropOn){selR=null;$("#pvsel").hidden=true}return}
    if(a==="cropno"){cropOn=false;selR=null;$("#pvcb").hidden=true;$("#pvsel").hidden=true;$("#pvc").style.cursor="";return}
    if(a==="cropok"){if(!selR)return;const c=$("#pvc"),k=c.width/c.clientWidth,w=Math.round(selR.w*k),h=Math.round(selR.h*k),t=document.createElement("canvas");t.width=w;t.height=h;
      t.getContext("2d").drawImage(c,Math.round(selR.x*k),Math.round(selR.y*k),w,h,0,0,w,h);b.disabled=true;b.textContent="Salvataggio…";
      t.toBlob(async bl=>{try{await xhrUpload("/api/best/img?id="+encodeURIComponent(eid),await prepImg(bl),()=>{});await loadBest();toast("Immagine ritagliata dal manuale e salvata.");$("#pvcb").hidden=true;$("#pvsel").hidden=true;cropOn=false;selR=null}catch(er){toast(er.message)}finally{b.disabled=false;b.textContent="Usa come immagine"}},"image/png");return}
    if(a==="x")o.remove();else if(a==="pp"){pg--;draw()}else if(a==="nx"){pg++;draw()}else if(a==="zi"){scale*=1.25;draw()}else if(a==="zo"){scale/=1.25;draw()}else if(a==="fit"){scale=0;draw()}};
  $("#pvn").onchange=e=>{pg=+e.target.value||1;draw()};
  // Ritaglio dell'illustrazione dal manuale del Master (resta sul PC: si salva come le immagini caricate a mano)
  let cropOn=false,selR=null,p0=null;const cv=$("#pvc"),box=$("#pvsel"),pos=e=>{const r=cv.getBoundingClientRect();return{x:Math.min(Math.max(0,e.clientX-r.left),r.width),y:Math.min(Math.max(0,e.clientY-r.top),r.height)}};
  cv.addEventListener("pointerdown",e=>{if(!cropOn)return;e.preventDefault();cv.setPointerCapture(e.pointerId);p0=pos(e)});
  cv.addEventListener("pointermove",e=>{if(!cropOn||!p0)return;const p=pos(e);selR={x:Math.min(p0.x,p.x),y:Math.min(p0.y,p.y),w:Math.abs(p.x-p0.x),h:Math.abs(p.y-p0.y)};
    Object.assign(box.style,{left:(cv.offsetLeft+selR.x)+"px",top:(cv.offsetTop+selR.y)+"px",width:selR.w+"px",height:selR.h+"px"});box.hidden=false});
  cv.addEventListener("pointerup",()=>{p0=null;$("#pvok").disabled=!(selR&&selR.w>20&&selR.h>20)})}

// Ricerca automatica delle schede: l'app legge il testo del manuale (sul PC del Master) e trova la pagina di ogni voce
const ALIAS={"mm-swarm-of-rats":["Sciame di topi"],"mm-flameskull":["Teschio fiammeggiante"],"mm-dire-wolf":["Lupo crudele"],"mm-wraith":["Spirito dannato","Wraith"],"mm-specter":["Spettro"],
  "mm-poltergeist":["Poltergeist"],"mm-werewolf":["Licantropo","Lupo mannaro"],"mm-cult-fanatic":["Fanatico del culto"],"mm-shadow-demon":["Demone ombra","Demone delle ombre"],"mm-will-o-wisp":["Fuoco fatuo"],
  "mm-giant-poisonous-snake":["Serpente velenoso gigante"],"mm-swarm-of-poisonous-snakes":["Sciame di serpenti velenosi"],"mm-saber-toothed-tiger":["Tigre dai denti a sciabola"],"mm-warhorse-skeleton":["Scheletro di cavallo da guerra"],
  "mm-giant-wolf-spider":["Ragno lupo gigante"],"mm-rug-of-smothering":["Tappeto soffocante"],"mm-gray-ooze":["Melma grigia"],"mm-vampire-spawn":["Progenie vampirica"],"mm-night-hag":["Megera notturna"],"mm-bandit-captain":["Capo dei banditi"]};
const nk=t=>norm(t).replace(/[^a-z0-9]+/g," ").trim();
function bNames(e){const a=[e.nome.replace(/\s*\(.*\)\s*$/,"")];if(e.en)a.push(e.en);(ALIAS[e.id]||[]).forEach(x=>a.push(x));if(e.cat==="png")a.push(e.nome.split(/,| e /)[0]);return[...new Set(a.map(nk).filter(x=>x.length>2))]}
async function indexManual(m,onp){const doc=await pdfDoc(m),N=doc.numPages,want=new Map(),best={};let chars=0;
  const ents=bAll().filter(e=>m.tipo==="avventura"?(e.fonte==="App. D"||e.cat==="png"):m.tipo==="mm"?(e.cat!=="png"&&e.fonte==="MM"):true);
  ents.forEach(e=>bNames(e).forEach(n=>want.set(n,(want.get(n)||[]).concat(e.id))));
  for(let p=1;p<=N;p++){const tc=await (await doc.getPage(p)).getTextContent(),rows=new Map(),hs=[];
    for(const it of tc.items){if(!it.str)continue;const t=it.transform,h=Math.hypot(t[2],t[3])||Math.abs(t[3])||10,y=Math.round(t[5]/Math.max(2,h*.5));chars+=it.str.length;hs.push(h);
      const r=rows.get(y)||{parts:[],h:0};r.parts.push({x:t[4],w:it.width||0,s:it.str,h});r.h=Math.max(r.h,h);rows.set(y,r)}
    const med=hs.sort((a,b)=>a-b)[hs.length>>1]||10;
    for(const r of rows.values()){r.parts.sort((a,b)=>a.x-b.x);let txt="",end=null;for(const q of r.parts){if(end!==null&&q.x-end>q.h*.25)txt+=" ";txt+=q.s;end=q.x+q.w}
      const k=nk(txt);const ids=want.get(k);if(!ids)continue;const sc=r.h/med;
      for(const id of ids)if(!best[id]||sc>best[id].s+.05)best[id]={p,s:sc}}
    if(onp&&(p%5===0||p===N))onp(p,N)}
  const index={};for(const id in best)index[id]=best[id].p;return{index,noText:chars<N*15,found:Object.keys(index).length,total:ents.length}}
async function runIndex(m,status){try{status("Lettura del manuale…");const r=await indexManual(m,(p,N)=>status(`Ricerca delle schede: pagina ${p} di ${N}…`));
    await api("/api/manual",{id:m.id,index:r.index,noText:r.noText});await loadBest();
    status(r.noText?"⚠ Questo PDF è una scansione senza testo: non posso cercare le schede, indica le pagine a mano.":`Schede trovate: ${r.found} su ${r.total}.`);return r}
  catch(e){status("Ricerca non riuscita: "+(e.message||e))}}
function manualsDlg(){const o=dlgX("");o.querySelector(".bx").classList.remove("sm");
  const draw=()=>{const ms=MANS();o.querySelector(".bx").innerHTML=`<div class="sh"><h2>📕 I tuoi manuali</h2><span class="mo-only">Solo Master</span><div><button data-a="back">← PNG e mostri</button> <button data-a="x">Chiudi</button></div></div>
    <p class="mut">Carica le tue copie in PDF dei manuali: restano nella cartella GasterBlessDati\\Manuali di questo PC e non vengono mai inviate ai giocatori. Per ogni manuale indica una pagina di riferimento, così l'app trova le altre: apri il PDF, cerca una pagina con il numero stampato e scrivi a quale pagina del PDF corrisponde.</p>
    ${ms.map(m=>`<div class="ed" data-man="${m.id}"><input type="text" data-k="nome" value="${esc(m.nome)}" style="flex:1;min-width:160px">
      <select data-k="tipo"><option value="mm"${m.tipo==="mm"?" selected":""}>Manuale dei Mostri</option><option value="avventura"${m.tipo==="avventura"?" selected":""}>Avventura (l'antagonista)</option><option value="altro"${m.tipo==="altro"?" selected":""}>Altro</option></select>
      <label>pag. libro <input type="number" data-k="refBook" min="1" value="${m.refBook}"></label><label>= pag. PDF <input type="number" data-k="refPdf" min="1" value="${m.refPdf}"></label>
      <label title="Scansione con due pagine del libro per ogni pagina del PDF"><input type="checkbox" data-k="spread"${m.spread?" checked":""}> doppia pagina</label>
      <button data-a="msave">Salva</button><button data-a="mtest">Prova</button><button data-a="midx" title="L'app legge il manuale e trova la pagina della scheda di ogni voce">🔎 Cerca le schede</button><button data-a="mdel">🗑</button>
      <span class="mut" data-st style="flex-basis:100%">${m.indexed?(m.noText?"⚠ PDF scansionato senza testo: indica le pagine a mano.":`Schede trovate: ${Object.keys(m.index||{}).length}.`):"Schede non ancora cercate."}</span></div>`).join("")||`<p class="mut">Nessun manuale caricato.</p>`}
    <div class="ed"><b>Aggiungi</b><input type="text" id="mnn" placeholder="Nome (es. Manuale dei Mostri)" style="flex:1;min-width:160px"><select id="mnt"><option value="mm">Manuale dei Mostri</option><option value="avventura">Avventura (l'antagonista)</option><option value="altro">Altro</option></select>
      <input type="file" id="mnf" accept="application/pdf,.pdf" hidden><button data-a="mup">＋ Scegli PDF…</button><span class="mut" id="mnb"></span></div>`;
    $("#mnf").onchange=async ev=>{const f=ev.target.files[0];if(!f)return;const nome=$("#mnn").value.trim()||f.name.replace(/\.pdf$/i,"");$("#mnb").textContent="Caricamento…";
      try{await xhrUpload(`/api/manual?nome=${encodeURIComponent(nome)}&tipo=${$("#mnt").value}`,new Blob([f],{type:"application/pdf"}),p=>{const b=$("#mnb");if(b)b.textContent="Caricamento "+Math.round(p*100)+"%"},"PUT");const nm=await loadBest().then(()=>MANS()[MANS().length-1]);draw();toast("Manuale caricato: cerco le schede…");
        if(nm&&nm.tipo!=="altro"){const row=o.querySelector(`[data-man="${nm.id}"] [data-st]`);await runIndex(nm,t=>{const r2=o.querySelector(`[data-man="${nm.id}"] [data-st]`);if(r2)r2.textContent=t});}}catch(er){$("#mnb").textContent="";toast(er.message)}}};
  draw();
  o.onclick=async e=>{if(e.target===o){o.remove();return}const b=e.target.closest("[data-a]");if(!b)return;const a=b.dataset.a,row=b.closest("[data-man]"),id=row&&row.dataset.man;
    const val=k=>{const x=row.querySelector(`[data-k=${k}]`);return x.type==="checkbox"?x.checked:x.type==="number"?+x.value:x.value};
    if(a==="x")o.remove();else if(a==="back"){o.remove();bestDlg()}else if(a==="mup")$("#mnf").click();
    else if(a==="msave"||a==="mtest"){try{await api("/api/manual",{id,nome:val("nome"),tipo:val("tipo"),refBook:val("refBook"),refPdf:val("refPdf"),spread:val("spread")});await loadBest();if(a==="mtest"){const m=MANS().find(x=>x.id===id);openManual(m,m.refBook,"Prova: pagina "+m.refBook)}else{draw();toast("Salvato.")}}catch(er){toast(er.message)}}
    else if(a==="mdel"){if(!b.dataset.c){b.dataset.c=1;b.textContent="Conferma";return}try{await api("/api/manual?id="+id,undefined,"DELETE");await loadBest();draw()}catch(er){toast(er.message)}}
    else if(a==="midx"){const m=MANS().find(x=>x.id===id);b.disabled=true;await runIndex(m,t=>{const st=row.querySelector("[data-st]");if(st)st.textContent=t});b.disabled=false}}}
// Dopo la selezione: scheda (solo Master) o immagine (mostrabile ai giocatori)
function choose(prev){const ids=[...BSEL].filter(id=>bById(id));/* B9: voci eliminate nel frattempo */if(!ids.length)return;
  const c=dlgX(`<h2>Cosa vuoi aprire?</h2><p>${ids.length===1?`<b>${esc(bById(ids[0]).nome)}</b>`:`${ids.length} voci selezionate`}</p>
  <p><button data-a="s" style="font-size:16px;padding:10px 16px;width:100%;text-align:left">📜 Scheda <small class="mut">— visibile solo a te</small></button></p>
  <p><button data-a="i" style="font-size:16px;padding:10px 16px;width:100%;text-align:left">🖼 Immagine <small class="mut">— puoi mostrarla ai giocatori o proiettarla</small></button></p><p style="text-align:right"><button data-a="x">Annulla</button></p>`);
  c.onclick=e=>{if(e.target===c){c.remove();return}const b=e.target.closest("[data-a]");if(!b)return;c.remove();if(b.dataset.a==="x")return;if(prev)prev.remove();b.dataset.a==="s"?sheetsDlg(ids,0):imagesDlg(ids)}}
function statHTML(s){const mod=v=>{const m=Math.floor((v-10)/2);return v+" ("+(m>=0?"+":"")+m+")"};const row=(k,v)=>v?`<p><b>${k}</b> ${esc(v)}</p>`:"";
  const blk=(t,a)=>a&&a.length?`<h4>${t}</h4>`+a.map(x=>`<p><b><i>${esc(x.n)}.</i></b> ${esc(x.d)}</p>`).join(""):"";
  return`<div class="it">${esc(s.taglia)} ${esc(s.tipo)}, ${esc(s.all)}</div><hr>${row("Classe Armatura",s.ca)}${row("Punti ferita",s.pf)}${row("Velocità",s.vel)}<hr>
  <div class="car">${["FOR","DES","COS","INT","SAG","CAR"].map((k,i)=>`<div><b>${k}</b>${mod(s.car[i])}</div>`).join("")}</div><hr>
  ${row("Tiri salvezza",s.ts)}${row("Abilità",s.abil)}${row("Vulnerabilità ai danni",s.vuln)}${row("Resistenze ai danni",s.res)}${row("Immunità ai danni",s.imm)}${row("Immunità alle condizioni",s.immc)}${row("Sensi",s.sensi)}${row("Linguaggi",s.ling)}${row("Sfida",s.gs)}<hr>
  ${(s.tratti||[]).map(x=>`<p><b><i>${esc(x.n)}.</i></b> ${esc(x.d)}</p>`).join("")}${blk("Azioni",s.azioni)}${blk("Reazioni",s.reazioni)}${blk("Azioni leggendarie",s.legg)}${s.nota?`<p class="mut">${/^Importata da/.test(s.nota)?"📥":"📕"} ${esc(s.nota)}</p>`:""}`}
function sheetsDlg(ids,cur){const o=dlgX("");o.querySelector(".bx").classList.remove("sm");
  const draw=()=>{const e=bById(ids[cur]),ov=BOV.entries[e.id]||{},base=e.base?bById(e.base):null,sh=e.scheda||(base&&base.scheda),bov=base?BOV.entries[base.id]||{}:{};
    o.querySelector(".bx").innerHTML=`${ids.length>1?`<div class="tabs">${ids.map((id,i)=>`<button data-t="${i}" class="${i===cur?"on":""}">${esc(bById(id).nome)}</button>`).join("")}</div>`:""}
    <div class="sh"><h2>📜 ${esc(e.nome)}</h2><span class="mo-only">Scheda visibile solo al Master</span><div><button data-a="img">🖼 Immagine</button> <button data-a="back">← Elenco</button> <button data-a="x">Chiudi</button></div></div>
    ${e.cat==="png"?`<p><b>Ruolo:</b> ${esc(e.ruolo||"")}${e.profilo?`<br><b>Profilo:</b> ${esc(e.profilo)}`:""}${e.pag&&e.pag.length?`<br><b>Pagine dell'avventura:</b> ${e.pag.join(", ")}`:""}</p>`:`<p>${e.en?`<i>${esc(e.en)}</i> · `:""}${esc(e.note||"")}${e.origine?esc(e.origine):""}${e.ambiente?` · <b>Ambiente:</b> ${esc(e.ambiente)}`:""}${e.pag&&e.pag.length?` · <b>Pagine:</b> ${e.pag.join(", ")}`:""}</p>`}
    ${offHTML(e)}
    ${ov.custom?`<div class="stb"><h3>${esc(e.nome)}</h3><div class="txt" id="cst"></div></div>`:""}
    ${sh?`<div class="stb"><h3>${esc(base?base.nome:e.nome)}${base?` <small class="mut">(scheda usata da ${esc(e.nome)})</small>`:""}</h3>${statHTML(sh)}</div>`:(!ov.custom?`<p class="mut">${esc(e.rif||(base&&base.rif)||"Nessuna scheda: personaggio non combattente o senza statistiche.")}</p>`:"")}
    ${base&&!sh&&bov.custom?`<div class="stb"><h3>${esc(base.nome)}</h3><div class="txt" id="bcst"></div></div>`:""}
    <div class="cols"><div><h3>Scheda personalizzata / trascrizione</h3><textarea id="tcu" placeholder="Scrivi o incolla qui la scheda (es. dall'appendice D) o le modifiche: apparirà in cima.">${esc(ov.custom||"")}</textarea></div><div><h3>Note del Master</h3><textarea id="tno" placeholder="Tattiche, segreti, frasi tipiche…">${esc(ov.notes||"")}</textarea></div></div>
    <p><button data-a="save">Salva</button> <span class="mut" id="bst2"></span></p>${e.fonte==="Personale"?`<p><button data-a="del">🗑 Elimina questa voce personale</button></p>`:""}<p class="mut">${sh?esc(BEST.attribuzione):""}</p>`;
    const c1=$("#cst");if(c1)c1.textContent=ov.custom;const c2=$("#bcst");if(c2)c2.textContent=bov.custom};
  draw();
  o.onclick=async e=>{if(e.target===o){o.remove();return}const t=e.target.closest("[data-t]");if(t){cur=+t.dataset.t;draw();return}const b=e.target.closest("[data-a]");if(!b)return;const a=b.dataset.a,id=ids[cur];
    if(a==="off"){const r=offRef(bById(id));if(r)openManual(r.m,r.page,bById(id).nome,r.pdf,id);return}
    if(a==="offsave"){const mid=$("#offm").value,pg=+$("#offp").value||0,tid=$("#offm").dataset.for;try{await api("/api/best",{id:tid,man:mid,page:pg});await loadBest();draw();toast("Pagina della scheda ufficiale salvata.")}catch(er){toast(er.message)}return}
    if(a==="mans"){o.remove();manualsDlg();return}
    if(a==="x")o.remove();else if(a==="back"){o.remove();bestDlg()}else if(a==="img"){o.remove();imagesDlg(ids)}
    else if(a==="save"){try{await api("/api/best",{id,custom:$("#tcu").value,notes:$("#tno").value});await loadBest();draw();$("#bst2").textContent="Salvato ✓"}catch(er){toast(er.message)}}
    else if(a==="del"){if(!b.dataset.c){b.dataset.c=1;b.textContent="Conferma eliminazione";return}try{await api("/api/best?id="+encodeURIComponent(id),undefined,"DELETE");BSEL.delete(id);await loadBest();o.remove();bestDlg()}catch(er){toast(er.message)}}}}
function imagesDlg(ids){const o=dlgX("");o.querySelector(".bx").classList.remove("sm");let names=true;
  const draw=()=>{o.querySelector(".bx").innerHTML=`<div class="sh"><h2>🖼 Immagini</h2><div><button data-a="sh">📜 Schede</button> <button data-a="back">← Elenco</button> <button data-a="x">Chiudi</button></div></div>
    <div class="bar" style="margin:0 0 10px"><button data-a="show" style="background:var(--ac);color:var(--bg)">👥 Mostra ai giocatori</button><label><input type="checkbox" id="snm"${names?" checked":""}> mostra anche i nomi</label><button data-a="hide">Nascondi ai giocatori</button><span class="mut">Le schede non vengono mai inviate ai giocatori.</span></div>
    <div class="ig">${ids.map(id=>{const e=bById(id),im=bImg(id);return`<div class="card" data-id="${id}"><div class="ph">${im?`<img src="${esc(mURL(im))}" alt="${esc(e.nome)}" data-z="1">`:`<span>Nessuna immagine</span>`}</div><b>${esc(e.nome)}</b>
      <div class="bar" style="margin:0"><label><input type="file" accept="image/*" hidden data-up="${id}"><span class="lnk" style="color:var(--ac);cursor:pointer;text-decoration:underline">${im?"Sostituisci":"＋ Carica immagine"}</span></label>${im?`<button data-a="pj" data-id="${id}">📺 Proietta</button><button data-a="one" data-id="${id}">👥 Solo questa</button><button data-a="rm" data-id="${id}" title="Rimuovi immagine">🗑</button>`:""}</div></div>`}).join("")}</div>
    <p class="mut" id="ubz"></p>`;
    o.querySelectorAll("[data-up]").forEach(inp=>inp.onchange=async ev=>{const f=ev.target.files[0];if(!f)return;$("#ubz").textContent="Caricamento di "+f.name+"…";
      try{const blob=await prepImg(f);await xhrUpload("/api/best/img?id="+encodeURIComponent(inp.dataset.up),blob,p=>{$("#ubz").textContent="Caricamento "+Math.round(p*100)+"%"});await loadBest();draw()}catch(er){$("#ubz").textContent="";toast(er.message)}});
    const sn=$("#snm");if(sn)sn.onchange=()=>names=sn.checked};
  draw();
  const show=async list=>{const it=list.filter(id=>bImg(id)).map(id=>({id,name:names?bById(id).nome:""}));if(!it.length){toast("Carica prima un'immagine.");return}
    try{const r=await api("/api/best/show",{items:it});toast(r.shown===1?"Immagine mostrata ai giocatori.":r.shown+" immagini mostrate ai giocatori.")}catch(er){toast(er.message)}};
  o.onclick=async e=>{if(e.target===o){o.remove();return}const z=e.target.closest("[data-z]");if(z){const lb=document.createElement("div");lb.id="lbx";lb.innerHTML=`<img src="${esc(z.src)}" alt="">`;lb.onclick=()=>lb.remove();document.body.appendChild(lb);return}
    const b=e.target.closest("[data-a]");if(!b)return;const a=b.dataset.a,id=b.dataset.id;
    if(a==="x")o.remove();else if(a==="back"){o.remove();bestDlg()}else if(a==="sh"){o.remove();sheetsDlg(ids,0)}
    else if(a==="show")show(ids);else if(a==="one")show([id]);
    else if(a==="hide"){try{await api("/api/best/show",{hide:true});toast("Immagini chiuse sui dispositivi dei giocatori.")}catch(er){toast(er.message)}}
    else if(a==="pj")sceltaProiezione({kind:"beast",file:bImg(id),cap:names?bById(id).nome:""},"l’immagine di "+(bById(id).nome||"questo PNG o mostro"));
    else if(a==="rm"){if(!b.dataset.c){b.dataset.c=1;b.textContent="Conferma";return}try{await api("/api/best/img?id="+encodeURIComponent(id),undefined,"DELETE");await loadBest();draw()}catch(er){toast(er.message)}}}}
// Lato giocatore: immagini mostrate dal Master
function showToPlayer(items){const old=$("#shw");if(old)old.remove();let i=0;const d=document.createElement("div");d.id="shw";d.setAttribute("role","dialog");document.body.appendChild(d);
  const draw=()=>{const it=items[i];d.innerHTML=`<img src="${esc(mURL(it.file))}" alt="${esc(it.name||"Immagine")}">${it.name?`<h2></h2>`:""}<div class="nv">${items.length>1?`<button data-a="p">‹</button><span style="align-self:center">${i+1}/${items.length}</span><button data-a="n">›</button>`:""}<button data-a="x">Chiudi</button></div>`;const h=d.querySelector("h2");if(h)h.textContent=it.name};
  draw();d.onclick=e=>{const b=e.target.closest("[data-a]");if(!b)return;if(b.dataset.a==="x")d.remove();else{i=(i+(b.dataset.a==="n"?1:-1)+items.length)%items.length;draw()}}}

/* ---------- Musica di sottofondo (Master): cartella GasterBlessDati\Musica, suona sul PC del Master o sulla proiezione ---------- */
let MUS={files:[],fight:[],cfg:{volume:.6,mode:"playlist",out:"master",fight:[]},state:{},locs:{},dir:""},MP=null;
const AU=[new Audio(),new Audio()];let auI=0,auCur="",auPos={},auBlocked=false,mvT=0;
const isEngine=()=>PJ?MUS.cfg.out==="proj":(role==="master"&&MUS.cfg.out==="master");
const musURL=f=>"/musica/"+f.split("/").map(encodeURIComponent).join("/");
const musName=f=>f?f.split("/").pop().replace(/\.[^.]+$/,"").replace(/[_]+/g," "):"";
function fade(a,to,ms,done){clearInterval(a._ft);const from=a.volume,t0=performance.now();a._ft=setInterval(()=>{const k=Math.min(1,(performance.now()-t0)/ms);a.volume=Math.max(0,Math.min(1,from+(to-from)*k));if(k>=1){clearInterval(a._ft);if(done)done()}},40)}
/* canale del browser «gasterbless-musica»: il tracker del Master chiede se una finestra della mappa suona la musica dello scontro;
   se nessuna risponde la suona lui (web/sezioni/iniziativa.js). Quando la mappa riprende a suonare lo avvisa e lui smette. */
const BCM=(()=>{try{const c=new BroadcastChannel("gasterbless-musica");c.onmessage=e=>{if(e.data==="chi-suona"&&isEngine()&&!auBlocked)c.postMessage("suono")};return c}catch(e){return null}})();
function auPlay(n){n.play().then(()=>{auBlocked=false;fade(n,bgVol(),1200);musUI();if(BCM)BCM.postMessage("suono")}).catch(()=>{auBlocked=true;musUI()})}
AU.forEach(a=>{a.preload="auto";a.addEventListener("ended",()=>{if(a===AU[auI]&&isEngine()&&!a.loop)api("/api/music/cmd",{op:"ended",file:auCur}).catch(()=>{})})});
document.addEventListener("pointerdown",()=>{if(auBlocked&&isEngine()&&MUS.state.playing)auPlay(AU[auI])},true);
function musApply(){const s=MUS.state||{};
  if(!isEngine()){AU.forEach(a=>{clearInterval(a._ft);a.pause()});if(auCur)auPos[auCur]=AU[auI].currentTime;auCur="";musUI();return}
  const cur=AU[auI];cur.loop=MUS.cfg.mode==="repeat";
  if((s.file||"")!==auCur){ // cambio di brano: dissolvenza incrociata, ripresa dal punto in cui era rimasto
    if(auCur){auPos[auCur]=cur.currentTime;fade(cur,0,1200,()=>cur.pause())}
    auCur=s.file||"";if(!auCur){musUI();return}
    auI^=1;const n=AU[auI];n.src=musURL(auCur);n.loop=MUS.cfg.mode==="repeat";n.volume=0;
    const pos=auPos[auCur];if(pos)n.addEventListener("loadedmetadata",()=>{if(pos<n.duration-2)n.currentTime=pos},{once:true});
    if(s.playing)auPlay(n)}
  else if(auCur){if(s.playing){cur.paused?auPlay(cur):fade(cur,bgVol(),300)}else fade(cur,0,500,()=>cur.pause())}
  musUI()}
function musUI(){if(PJ)return;const s=MUS.state||{};$("#mpp").textContent=s.playing?"⏸":"▶";
  if(document.activeElement!==$("#mvo"))$("#mvo").value=Math.round(MUS.cfg.volume*100);
  $("#mnm").textContent=auBlocked&&isEngine()&&s.playing?"🔇 clicca per attivare l'audio":s.file?(s.fight?"⚔ ":"")+musName(s.file)+(MUS.cfg.out==="proj"?" (proiezione)":""):"Musica";
  if(MP)MP()}
async function musLoad(){try{const r=await api("/api/music");r.files=r.files||[];r.fight=r.fight||[];r.locs=r.locs||{};r.cfg.fight=r.cfg.fight||[];MUS=r;musApply()}catch(e){}}
const mcmd=b=>api("/api/music/cmd",b).catch(e=>toast(e.message));
$("#mpp").onclick=()=>mcmd(MUS.state.playing?{op:"pause"}:{op:"resume"});
$("#mnx").onclick=()=>mcmd({op:"next"});
function openTracker(app){/* nuova app: il tracker è la sezione Iniziativa */try{window.top.location.href="/iniziativa?s="+encodeURIComponent(SID)}catch(e){location.href="/iniziativa?s="+encodeURIComponent(SID)}}
$("#mfi").onclick=()=>openTracker(true);
$("#trk").onclick=()=>openTracker(false);
$("#mvo").oninput=e=>{MUS.cfg.volume=e.target.value/100;if(isEngine()&&MUS.state.playing)fade(AU[auI],bgVol(),150);clearTimeout(mvT);mvT=setTimeout(()=>mcmd({op:"volume",value:MUS.cfg.volume}),300)};
$("#mpn").onclick=()=>musicPanel();
function musicPanel(){const o=dlgX("");o.querySelector(".bx").classList.remove("sm");
  const draw=()=>{if(!o.isConnected){MP=null;return}const s=MUS.state||{},fset=new Set(MUS.fight||[]),fe=document.activeElement;if(fe&&o.contains(fe)&&fe.tagName==="SELECT")return;
    o.querySelector(".bx").innerHTML=`<div class="sh"><h2>🎵 Musica di sottofondo</h2><span class="mo-only">Solo Master</span><div><button data-a="x">Chiudi</button></div></div>
    <p class="mut" style="margin-top:0">Metti i brani (MP3, OGG, M4A, WAV, FLAC) nella cartella <b>${esc(MUS.dir)}</b>; quelli nella sottocartella <b>Combattimento</b> sono usati negli scontri. I giocatori non sentono nulla sui loro telefoni: la musica esce dal PC del Master.</p>
    <div class="bar" style="margin:0 0 8px"><button data-a="rf">⟳ Aggiorna elenco</button>${ME.local?`<button data-a="fold">📂 Apri cartella</button>`:""}<input type="file" id="mup" accept="audio/*,.mp3,.ogg,.m4a,.wav,.flac,.opus" multiple hidden><button data-a="up">＋ Aggiungi brani</button><label><input type="checkbox" id="mupf"> nella cartella Combattimento</label><span class="mut" id="mub"></span></div>
    <div class="bar" style="margin:0 0 8px"><label>Ordine <select id="mmo"><option value="playlist">In ordine</option><option value="shuffle">Casuale</option><option value="repeat">Ripeti il brano</option></select></label>
      <label>Riproduci su <select id="mou"><option value="master">questo PC (finestra del Master)</option><option value="proj">finestra di proiezione (TV)</option></select></label>
      <button data-a="pp">${s.playing?"⏸ Pausa":"▶ Riproduci"}</button><button data-a="pv">⏮</button><button data-a="nx">⏭</button><button data-a="stop">⏹</button><button data-a="fi" class="${s.fight?"on":""}" id="mfi2">⚔ Combattimento ${s.fight?"attivo":""}</button></div>
    ${MUS.files.length?`<ul class="ml">${MUS.files.map(f=>`<li class="${f===s.file?"on":""}"><button data-play="${esc(f)}" title="Riproduci">▶</button><span class="nm2">${esc(musName(f))}${f.includes("/")?` <small class="mut">${esc(f.split("/").slice(0,-1).join("/"))}</small>`:""}</span><label title="Usa negli scontri"><input type="checkbox" data-fi="${esc(f)}"${fset.has(f)?" checked":""}> ⚔</label></li>`).join("")}</ul>`:`<p class="err" style="color:var(--ac)">La cartella è vuota: aggiungi dei brani e premi «Aggiorna elenco».</p>`}
    <p class="mut">Musica delle località: si sceglie nella scheda di ogni località e parte da sola quando ne apri o proietti un video. ${Object.keys(MUS.locs||{}).length?"Impostate: "+Object.entries(MUS.locs).map(([k,f])=>esc(((LOCS.find(l=>l.k===k)||{}).n||k)+" → "+musName(f))).join(" · "):"Nessuna impostata."}</p>`;
    $("#mmo").value=MUS.cfg.mode;$("#mou").value=MUS.cfg.out;
    $("#mmo").onchange=e=>mcmd({op:"mode",text:e.target.value});$("#mou").onchange=e=>mcmd({op:"out",text:e.target.value});
    $("#mup").onchange=async ev=>{const fs=[...ev.target.files];let n=0;for(const f of fs){$("#mub").textContent=`Caricamento ${++n}/${fs.length}…`;try{await xhrUpload(`/api/music/upload?name=${encodeURIComponent(f.name)}${$("#mupf").checked?"&fight=1":""}`,new Blob([f],{type:"application/octet-stream"}),()=>{})}catch(er){toast(f.name+": "+er.message)}}await musLoad();draw()};
    o.querySelectorAll("[data-fi]").forEach(c=>c.onchange=()=>mcmd({op:"setfight",files:[...o.querySelectorAll("[data-fi]:checked")].map(x=>x.dataset.fi)}).then(musLoad))};
  MP=draw;draw();
  o.onclick=async e=>{if(e.target===o){o.remove();MP=null;return}const pl=e.target.closest("[data-play]");if(pl){mcmd({op:"play",file:pl.dataset.play});return}const b=e.target.closest("[data-a]");if(!b)return;const a=b.dataset.a;
    if(a==="x"){o.remove();MP=null}else if(a==="rf"){await musLoad();draw()}else if(a==="fold")api("/api/music/folder",{}).catch(er=>toast(er.message));else if(a==="up")$("#mup").click();
    else if(a==="pp")mcmd(MUS.state.playing?{op:"pause"}:{op:"resume"});else if(a==="nx")mcmd({op:"next"});else if(a==="pv")mcmd({op:"prev"});else if(a==="stop")mcmd({op:"stop"});else if(a==="fi")mcmd({op:"fight",on:!MUS.state.fight})}}

/* ---------- Canti per tutto il tavolo (Viandante, bardico): giocatori e Master; il Master li blocca ---------- */
let SONG={lock:{},kind:"",videos:{}};const AS=new Audio();let songOn=false;
function songUI(){document.querySelectorAll(".songs .sg").forEach(b=>{const k=b.dataset.song,lk=!!SONG.lock[k];b.classList.toggle("on",SONG.kind===k);
    b.classList.toggle("locked",lk&&role!=="master");b.disabled=lk&&role==="player";b.title=lk&&role==="player"?"Bloccato dal Master":(SONG.kind===k&&role==="master"?"Clic per interrompere il canto":b.dataset.t||b.title);
    if(!b.dataset.t)b.dataset.t=b.title;const base=k==="viandante"?"🎻 Canto Viandante":"🪕 Canto bardico";b.textContent=(lk&&role==="player"?"🔒 ":"")+base+(SONG.kind===k?" ♪":"")});
  document.querySelectorAll(".songs .lk").forEach(b=>{const lk=!!SONG.lock[b.dataset.lock];b.textContent=lk?"🔒":"🔓";b.classList.toggle("on",lk);b.setAttribute("aria-pressed",lk)})}
document.querySelector(".songs").onclick=async e=>{const b=e.target.closest("button");if(!b)return;
  if(b.dataset.lock){try{await api("/api/song",{op:"lock",kind:b.dataset.lock,on:!SONG.lock[b.dataset.lock]})}catch(er){toast(er.message)}return}
  const k=b.dataset.song;if(role==="master"&&SONG.kind===k){api("/api/song",{op:"stop"}).catch(er=>toast(er.message));return}
  if(role==="master"&&(SONG.videos[k]||[]).length){songChoose(k);return} // video nella cartella del canto: il Master sceglie
  try{await api("/api/song",{kind:k})}catch(er){toast(er.message,4500)}};
/* Video nei canti (dal ramo main-ke8sy4): brano a caso oppure un video della cartella, per tutto il tavolo */
function songChoose(k){const o=document.createElement("div");o.className="dlg songch";const nm=f=>f.split("/").pop().replace(/\.[^.]+$/,"");
  o.innerHTML=`<div class="bx sm"><h2>${k==="viandante"?"🎻 Canto Viandante":"🪕 Canto bardico"}</h2><p class="mut">Un brano audio a caso, oppure un video da mostrare a tutto il tavolo (sugli schermi dei giocatori senza audio; chi gioca a distanza lo sente al massimo).</p>
  <div class="songl"><button data-sv="">🎵 Brano audio a caso</button>${SONG.videos[k].map(f=>`<button data-sv="${esc(f)}">🎬 ${esc(nm(f))}</button>`).join("")}</div><button data-sx>Annulla</button></div>`;
  document.body.appendChild(o);
  o.onclick=async e=>{if(e.target===o||e.target.closest("[data-sx]")){o.remove();return}const b=e.target.closest("[data-sv]");if(!b)return;o.remove();
    try{await api("/api/song",b.dataset.sv?{kind:k,video:b.dataset.sv}:{kind:k})}catch(er){toast(er.message,4500)}}}
let songVista=null;
function songVideo(m){songVideoOff();const tavolo=PJ?MUS.cfg.out==="proj":(role==="master"&&MUS.cfg.out==="master");
  const o=document.createElement("div");o.id="songv";o.innerHTML=PJ?"":`<div class="songvb"><b>${esc(m.name)}</b>${role==="master"?'<button data-svx>Chiudi per tutti</button>':''}</div>`;
  const gioc=role!=="master"&&!PJ; // telefono di un giocatore: parte dopo il precaricamento (vedi precarica)
  const v=document.createElement("video");v.preload="auto";v.src=compat("/api/songvideo");v.autoplay=!gioc;v.playsInline=true;v.muted=!tavolo;v.controls=role==="master"&&!PJ;o.appendChild(v);
  // sul proiettore: senza titolo né comandi, a tutto schermo e regolato (proiettore.js), sotto il tasto d'avvio
  if(PJ){o.className="pjpulito";$("#pj").insertBefore(o,$("#pjgo"));songVista=BP.mostra(o,v)}else document.body.appendChild(o);const via=()=>{if(!tavolo&&LONTANO&&role!=="master")audioPieno(v,o);else v.play().catch(()=>{v.muted=true;v.play().catch(()=>{})})};
  if(gioc)precarica(v,o,via);else via();
  if(tavolo){songOn=true;if(MUS.state.playing&&auCur)fade(AU[auI],bgVol(),800)}
  o.querySelector("[data-svx]")?.addEventListener("click",()=>api("/api/song",{op:"stop"}).catch(er=>toast(er.message)));
  v.addEventListener("ended",()=>{if(role==="master")api("/api/song",{op:"end"}).catch(()=>{})});}
function songVideoOff(){if(songVista){songVista.ferma();songVista=null}const o=document.getElementById("songv");if(o){const v=o.querySelector("video");if(v){v.pause();v.removeAttribute("src");v.load()}o.remove()}}
const bgVol=()=>MUS.cfg.volume*(songOn?.2:1);
function songStart(m){SONG.kind=m.kind;songUI();toast(`${m.kind==="viandante"?"🎻":"🪕"} ${m.name}: avviato da ${m.by}.`,3500);
  if(m.video){songVideo(m);return}
  if(PJ?MUS.cfg.out!=="proj":!(role==="master"&&MUS.cfg.out==="master"))return; // suona solo sull'audio del tavolo
  songOn=true;if(MUS.state.playing&&auCur)fade(AU[auI],bgVol(),800);
  AS.src=musURL(m.file);AS.volume=Math.min(1,Math.max(.5,MUS.cfg.volume*1.3));AS.play().catch(()=>{auBlocked=true;musUI();toast("Clicca per attivare l'audio del canto.")})}
function songEnd(){SONG.kind="";songUI();songVideoOff();if(songOn){songOn=false;AS.pause();if(MUS.state.playing&&auCur)fade(AU[auI],bgVol(),1200)}}
AS.addEventListener("ended",()=>{if(songOn)api("/api/song",{op:"end"}).catch(()=>{})});AS.addEventListener("error",()=>{if(songOn)api("/api/song",{op:"end"}).catch(()=>{})});
document.addEventListener("pointerdown",()=>{if(songOn&&AS.paused&&AS.src&&!AS.ended)AS.play().catch(()=>{})},true);
async function songLoad(){try{const r=await api("/api/song");SONG.lock=r.songLock||{};SONG.kind=r.song||"";SONG.videos=r.videos||{}}catch(e){}songUI()}
/* ---------- Pacchetto ---------- */
let pkT=0;
$("#pkgo").onclick=async()=>{const st=$("#pkst");try{const r=await api("/api/package",{manuali:$("#pkm").checked,gruppi:$("#pkg").checked});st.textContent="Preparazione…"+(r.warn?" ⚠ "+r.warn:"");
    clearInterval(pkT);pkT=setInterval(async()=>{try{const j=await api("/api/package");const pc=j.total?Math.round(j.done*100/j.total):100;
      if(j.running)st.textContent=`Copia dei contenuti: ${pc}% (${j.files} file)`;else{clearInterval(pkT);st.textContent=j.err?"Errore: "+j.err:`✔ Pacchetto pronto (${j.files} file, ${Math.round(j.done/1048576)} MB) in ${j.dir}`+(j.warn?" — ⚠ "+j.warn:"")}}catch(e){clearInterval(pkT)}},700)}
  catch(er){st.textContent=er.message}};
/* ---------- Monte Gelo: stato del gruppo, fascia e tiri dell'ora (Master) ---------- */
let gkSig="";
async function geloAgg(){try{const g=await api("/api/gelo");const sig=JSON.stringify(g);if(sig===gkSig)return;gkSig=sig;
  const nuovo=!GK||JSON.stringify(GK.pts)!==JSON.stringify(g.pts);GK=g;if(nuovo){gkPrepara();legs.forEach(l=>l.segs.forEach(s=>s.pz=null));render()}
  const b=$("#gkb"),seg=(g.segnalini||[]);if(!b)return;
  if(!seg.length){b.hidden=true;return}
  const s=seg[0],R=g.regole;b.hidden=false;
  b.innerHTML=`❄ <b>Monte Gelo</b>: il gruppo è a <b>${m0(s.quota)} m</b>${s.alta?" (alta quota)":""} · ${R.temperatura} °C · raffiche di vento gelato · ghiaccio e neve · TS ogni ora (Costituzione CD ${R.cdFreddo}, Forza o Destrezza CD ${R.cdVento}${s.alta?" con svantaggio":""})`+
    (role==="master"?` <button id="gkt">❄ Tiri di quest'ora</button>`:"");
  const bt=$("#gkt");if(bt)bt.onclick=()=>gkTiri()}catch(e){}}
function gkTiri(){const seg=(GK.segnalini||[]).filter(s=>s.pg&&s.pg.length);if(!seg.length){toast("Nessun personaggio del gruppo sul tratto del Monte Gelo.");return}
  const s=seg[0],o=document.createElement("div");o.className="dlg";o.innerHTML='<div class="bx gk" role="dialog" aria-modal="true"></div>';document.body.appendChild(o);
  const bx=o.querySelector(".bx"),sg=n=>(n>=0?"+":"")+n;
  bx.innerHTML=`<h2>❄ Tiri di quest'ora · Monte Gelo</h2><p class="mut">Quota ${m0(s.quota)} m${s.alta?" · <b>alta quota</b>: TS contro il vento con svantaggio, un 1 naturale fa precipitare":""}. Controlla chi indossa davvero gli abiti contro il freddo (letti dall'equipaggiamento della scheda).</p>
    <table class="gkt"><tr><th>Personaggio</th><th>Pelliccia</th><th>Mantello pesante</th><th>Immune al freddo</th><th title="Armatura pesante, scarponi o stivali da scalatore: vantaggio al TS contro il vento (quello che fa precipitare)">Presa salda</th><th>TS contro il vento</th></tr>${s.pg.map(p=>`<tr data-pg="${esc(p.id)}"><td>${esc(p.nome)}</td>
      <td><input type="checkbox" data-k="pelliccia"${p.pelliccia?" checked":""} aria-label="Abiti in pelliccia"></td><td><input type="checkbox" data-k="mantello"${p.mantello?" checked":""} aria-label="Mantello pesante"></td>
      <td><input type="checkbox" data-k="immune" aria-label="Immune al freddo"></td>
      <td><label><input type="checkbox" data-k="appiglio"${p.appiglio?" checked":""} aria-label="Presa salda (armatura pesante o scarponi da scalatore)"> <small class="mut">${esc(p.appiglioDa||"")}</small></label></td>
      <td><select data-k="vento" aria-label="Caratteristica per il vento"><option value="FOR"${p.vento==="FOR"?" selected":""}>Forza ${sg(p.bonusFOR)}</option><option value="DES"${p.vento==="DES"?" selected":""}>Destrezza ${sg(p.bonusDES)}</option></select></td></tr>`).join("")}</table>
    <div id="gkr"></div><p style="text-align:right"><button data-a="x">Chiudi</button> <button data-a="tira">🎲 Tira</button></p>`;
  let id=null;
  bx.onclick=async e=>{const b=e.target.closest("[data-a]");if(!b)return;
    if(b.dataset.a==="x"){o.remove();return}
    if(b.dataset.a==="tira"){const pg=[...bx.querySelectorAll("tr[data-pg]")].map(tr=>({id:tr.dataset.pg,pelliccia:tr.querySelector('[data-k=pelliccia]').checked,mantello:tr.querySelector('[data-k=mantello]').checked,immune:tr.querySelector('[data-k=immune]').checked,appiglio:tr.querySelector('[data-k=appiglio]').checked,vento:tr.querySelector('[data-k=vento]').value}));
      try{const r=await api("/api/gelo/tiri",{segnalino:s.id,pg});id=r.id;const d=x=>x?`[${x.d20.join(", ")}] ${x.totale}`:"";
        $("#gkr").innerHTML=`<h3>Esiti</h3><ul>${r.risultati.map(e=>`<li class="${e.precipita?"gk-cade":""}"><b>${esc(e.nome)}</b>: ${esc(e.testo)} <small class="mut">${e.freddo?"Cos "+d(e.freddo)+" · ":""}vento ${d(e.vento)}</small></li>`).join("")}</ul>
          <p class="mut">Indebolimento e danni non sono ancora applicati: premi «Applica» per confermarli (o chiudi per scartare i tiri).</p><p style="text-align:right"><button data-a="applica">✔ Applica</button></p>`;
        b.textContent="🎲 Tira di nuovo"}catch(er){toast(er.message,5000)}}
    if(b.dataset.a==="applica"&&id){try{await api("/api/gelo/applica",{id});toast("Esiti applicati alle schede.");o.remove()}catch(er){toast(er.message,5000)}}};
  o.onclick=e=>{if(e.target===o)o.remove()}}
/* ---------- Viaggi del segnalino del gruppo (2026-11-08) ----------
   Spostando il segnalino di più di 1/4 di miglio si calcola il percorso più rapido (strade e sentieri, o fuori strada
   come «Da … a …») con l'andatura scelta, il tempo e i tratti rallentati dal terreno e dal Monte Gelo. Il server lo
   registra nel diario di viaggio, fa passare il tempo di gioco (24 ore = un giorno) e manda l'avviso al gruppo e al Master. */
async function calcolaViaggio(da,a){if(!da||pxMi(Math.hypot(a[0]-da[0],a[1]-da[1]))<.25)return null;
  const punto=async p=>{let i=-1;try{i=(await api("/api/strada",{op:"aggancia",x:Math.round(p[0]),y:Math.round(p[1]),r:15})).i}catch(e){}
    if(i>=0){const[x,y]=pt(i);return{x,y,i}}return{x:Math.round(p[0]),y:Math.round(p[1]),i:-1}};
  const A=await punto(da),B=await punto(a),mph=+$("#pace").value||3;
  let l=null;try{l=await legBetween(A,B)}catch(e){}
  if(!l)l={segs:[straight([A.x,A.y],[B.x,B.y])]};
  if(A.x!==Math.round(da[0])||A.y!==Math.round(da[1]))l.segs.unshift(straight([Math.round(da[0]),Math.round(da[1])],[A.x,A.y]));
  if(B.x!==Math.round(a[0])||B.y!==Math.round(a[1]))l.segs.push(straight([B.x,B.y],[Math.round(a[0]),Math.round(a[1])]));
  const m=new Map();let miglia=0;l.segs.forEach(s=>pezzi(s).forEach(q=>{const mi=pxMi(q.px);miglia+=mi;const k=q.tipo+"|"+q.nome;m.set(k,(m.get(k)||0)+mi)}));
  if(miglia<.25)return null;
  const pz=[...m.entries()].map(([k,mi])=>{const[tipo,nome]=k.split("|");return{tipo,nome,mi:Math.round(mi*100)/100,f:fatt(tipo)}});
  const ore=legH(l,mph),gk=legGk(l),fatica=ore+(gk.alta?gk.alta/(mph*.5):0); // alta quota: ogni ora conta doppia per la fatica
  let arr=null,bd=20;for(const x of LOCS){const d=Math.hypot(x.x-a[0],x.y-a[1]);if(d<bd){bd=d;arr=x}}
  return{miglia:Math.round(miglia*100)/100,ore,fatica,mph,pezzi:pz,arrivo:arr?arr.k:"",gelo:gk.mi>=.05?{mi:gk.mi,alta:gk.alta,quota:gkQuota(gk.f1)}:null}}
async function inviaMossa(corpo,da){let v=null;try{v=await calcolaViaggio(da,[corpo.x,corpo.y])}catch(e){}
  if(v)corpo.viaggio=v;try{await api("/api/party",corpo)}catch(er){toast(er.message)}refresh();diarioAgg()}
let DIARIO=null;
async function diarioAgg(){try{DIARIO=await api("/api/viaggi")}catch(e){return}const b=$("#dvg");if(b)b.textContent="🧭 Giorno "+DIARIO.giorno;
  const o=$("#dvgd");if(o)diarioDisegna(o)}
function diarioDisegna(o){const d=DIARIO||{lista:[]},ma=role==="master",oggi=d.oreGioco%24,T={terreno:"⛰️",gelo:"❄️",marcia:"🥵",giorno:"🌅"};
  o.querySelector(".bx").innerHTML=`<h2>🧭 Diario di viaggio</h2>
  <p><b>Giorno ${d.giorno}</b> · ${fmt(oggi)} trascorse oggi in viaggio · ${fmt(d.oreMarcia)} di marcia (oltre 8 h è marcia forzata) · in tutto ${mi1(d.migliaTotali)} miglia</p>
  ${d.lista.length?`<table class="gkt"><tr><th>Giorno</th><th>Arrivo</th><th>Miglia</th><th>Tempo</th><th>Eventi</th><th>Chi</th></tr>${d.lista.map(v=>`<tr><td>${v.giorno}</td><td>${esc(v.arrivo||"—")}</td><td>${mi1(v.miglia)}</td><td>${fmt(v.ore)}</td><td>${v.eventi.map(t=>T[t]||"").join(" ")}</td><td>${esc(v.chi)}</td></tr>`).join("")}</table>`:`<p class="mut">Nessun viaggio: si registrano gli spostamenti del segnalino del gruppo oltre 1/4 di miglio.</p>`}
  <p class="mut">Ogni 24 ore di viaggio passa un giorno di gioco (anche per le Professioni).${ma?" «Nuovo giorno» quando il gruppo si accampa: la marcia ripartirà da zero.":""}</p>
  <p style="text-align:right">${ma?`<button data-a="ann">↶ Annulla l'ultimo viaggio</button> <button data-a="gio">🌙 Nuovo giorno</button> `:""}<button data-a="x">Chiudi</button></p>`}
function diarioApri(){const o=document.createElement("div");o.className="dlg";o.id="dvgd";o.innerHTML='<div class="bx" role="dialog" aria-modal="true"></div>';document.body.appendChild(o);diarioDisegna(o);diarioAgg();
  o.onclick=async e=>{if(e.target===o){o.remove();return}const b=e.target.closest("[data-a]");if(!b)return;const a=b.dataset.a;
    if(a==="x"){o.remove();return}
    try{if(a==="gio"){const r=await api("/api/viaggi/giorno",{});toast("🌙 Comincia il giorno "+r.giorno+".")}
      if(a==="ann"){await api("/api/viaggi/ultimo",undefined,"DELETE");toast("Ultimo viaggio annullato: riporta il segnalino dove serve.")}}catch(er){toast(er.message)}
    diarioAgg()}}
/* Pannello laterale della Mappa (Da … a …, percorso) a scomparsa (2026-11-08): la scelta resta salvata sul dispositivo. */
(()=>{const b=document.createElement("button");b.id="lato";const K="gasterbless.carta.lato";
  // Sul telefono il pannello parte nascosto (la mappa ha bisogno di tutto lo schermo) finché non lo si mostra.
  const nascosto=()=>{try{const v=localStorage.getItem(K);return v==="no"||(v===null&&innerWidth<=800)}catch(e){return innerWidth<=800}};
  const applica=()=>{const n=nascosto();document.body.classList.toggle("senza-lato",n);b.textContent=n?"◧ Mostra pannello":"◨ Nascondi pannello";b.setAttribute("aria-pressed",String(n))};
  b.title="Nasconde o mostra il pannello a destra (percorso, Da … a …): la mappa si allarga";
  b.onclick=()=>{try{localStorage.setItem(K,nascosto()?"si":"no")}catch(e){}applica()};
  $("header .sp").before(b);applica()})(); // nella testata: lo vedono anche i giocatori (la barra #mt è solo del Master)
(()=>{const b=document.createElement("button");b.id="dvg";b.title="Diario di viaggio: strada fatta, tempo trascorso e giorno di gioco";b.textContent="🧭 Diario";b.onclick=diarioApri;
  $("header .sp").before(b)})();
/* Telefono (2026-11-09): i comandi della testata e della barra del Master stanno dietro «⚙ Comandi»; restano in vista
   solo titolo, zoom e il tasto stesso, così la mappa ha quasi tutto lo schermo. */
(()=>{const b=document.createElement("button");b.id="cmdt";b.setAttribute("aria-expanded","false");
  const agg=()=>{const on=document.body.classList.contains("cmd");b.textContent=on?"✕ Chiudi comandi":"⚙ Comandi";b.setAttribute("aria-expanded",String(on))};
  b.onclick=()=>{document.body.classList.toggle("cmd");agg()};$("header h1").after(b);agg()})();
/* ---------- Avvio ---------- */
/* il service worker è quello dell'app (/sw.js) */
(async()=>{try{LOCS=await (await fetch("locs.json")).json()}catch(e){}
  try{const t=await (await fetch("terreno.json")).json();if(t&&t.zone)TERR=t}catch(e){} // zone di terreno per i tempi fuori strada
  $("#ld").hidden=true;boot()})();

/* Gaster Bless (nuova app): pulsante del Master per posizionare il gruppo sulla mappa */
(()=>{const b=document.createElement("button");b.id="party";b.className="mo";b.title="Clicca poi sulla mappa: il segnalino del gruppo (con i ritratti dei PG) va lì. Poi si sposta anche trascinandolo; tasto destro per dividerlo o personalizzarlo.";b.textContent="📍 Posiziona il gruppo";
  $("#mt").insertBefore(b,$("#mt").children[1]||null);
  b.onclick=()=>{if(tool!=="route")setFog(false);partyMode=!partyMode;b.classList.toggle("on",partyMode);map.style.cursor=partyMode?"copy":"crosshair";if(partyMode)toast("Clicca sulla mappa dove si trova il gruppo.")};
  document.addEventListener("keydown",e=>{if(e.key==="Escape"&&partyMode){partyMode=false;b.classList.remove("on");map.style.cursor="crosshair"}})})();
/* Testata ordinata (controllo del 2026-11-09): i comandi in gruppi con il loro nome, sempre a un clic.
   Percorso (andatura, fuori strada, annulla, azzera, scala), Vista (nomi, pannello, diario), Strumenti (PNG e mostri,
   inviti, proiezione, sessioni, iniziativa, esci). Zoom e canti restano liberi: si usano anche con la nebbia. */
(()=>{const H=$("header");if(!H)return;
  const gruppo=(nome,cls,els)=>{const v=els.filter(Boolean);if(!v.length)return null;const g=document.createElement("div");g.className="grp "+cls;g.setAttribute("role","group");g.setAttribute("aria-label",nome);
    const l=document.createElement("span");l.className="grp-t";l.textContent=nome;g.append(l,...v);return g};
  const lab=id=>{const x=$("#"+id);return x?x.closest("label")||x:null};
  const perc=gruppo("Percorso","grp-perc",[lab("pace"),lab("off"),$("#undo"),$("#clr"),lab("mh"),$("#mhp")]);
  const vista=gruppo("Vista","grp-vista",[lab("lb"),$("#lato"),$("#dvg")]);
  const strum=gruppo("Strumenti","grp-strum",[$("#bst"),$("#inv"),$("#prj"),$("#ses"),$("#trk"),$("#ex")]);
  const sp=H.querySelector(".sp"),zo=$("#zo"),zi=$("#zi"),songs=H.querySelector(".songs");
  [perc,vista].forEach(g=>{if(g)sp.before(g)});
  if(zo&&zi){sp.before(zo,zi)}
  if(strum)H.append(strum);
  if(songs)strum?strum.before(songs):H.append(songs);
  // Su PC le righe vuote non servono: i gruppi si dispongono da soli.
})();
