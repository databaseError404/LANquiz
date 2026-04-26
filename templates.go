package main

const playerHTML = `<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<link rel="manifest" href="/manifest.webmanifest">
<title>LAN Quiz — Игрок</title>
<style>
body{
  margin:0;
  font-family:Arial,Helvetica,sans-serif;
  background:#0b1020;
  color:#fff;
  padding:18px;
}
.wrap{
  max-width:560px;
  margin:0 auto;
}
.card{
  background:#141b34;
  border-radius:20px;
  padding:18px;
  box-shadow:0 10px 30px rgba(0,0,0,.25);
}
input,button{
  font-size:18px;
  border-radius:12px;
  padding:12px;
}
input{
  width:100%;
  box-sizing:border-box;
  border:1px solid #2b3c6a;
  background:#0c1430;
  color:#fff;
}
input[type="checkbox"]{
  width:auto;
  padding:0;
  margin:0;
  accent-color:#4da3ff;
}
button{
  border:none;
  cursor:pointer;
}
.row{
  display:flex;
  gap:12px;
  flex-wrap:wrap;
}
.answers{
  display:grid;
  grid-template-columns:1fr 1fr;
  gap:12px;
  margin-top:16px;
}
.answer{
  min-height:86px;
  font-size:32px;
  font-weight:800;
  background:#1a2446;
  color:#fff;
}
.answer.active{
  outline:3px solid #4f8dff;
  background:#1a2f5f;
}
.answer.correct{
  outline:3px solid #4fd18b;
  background:#16322b;
}
.answer.revealed{
  outline:3px solid #9ca3af;
  background:#374151;
}
.answer.wrong{
  outline:3px solid #ff6b6b;
  background:#4a1f2b;
}
.answer:disabled{
  opacity:.55;
  cursor:not-allowed;
}
.muted{
  color:#aab7dd;
}
.hidden{
  display:none;
}
.box{
  background:#0c1430;
  border-radius:12px;
  padding:12px;
  margin-top:12px;
}
.status-ok{
  background:#123322;
}
.status-warn{
  background:#3a2e11;
}
.status-bad{
  background:#441c24;
}
.small{
  font-size:12px;
}
.hintText{
  display:block;
  margin-top:3px;
  font-size:11px;
  color:#97a0b8;
}
.myAnswerRight{
  background:#123322;
}
.myAnswerWrong{
  background:#441c24;
}
.playerStatsTitle{
  margin-top:14px;
  color:#d6e3ff;
  font-weight:700;
}
.playerStatsTable{
  width:100%;
  border-collapse:collapse;
  margin-top:8px;
}
.playerStatsTable th,
.playerStatsTable td{
  border-bottom:1px solid #2b3c6a;
  padding:8px;
  text-align:center;
}
</style>
</head>
<body>
<div class="wrap">
  <div class="card">
    <div id="joinBox">
      <input id="teamName" placeholder="Название команды">
      <div class="row" style="margin-top:12px">
        <button onclick="join()">Подключиться</button>
      </div>
    </div>

    <div id="gameBox" class="hidden">
      <div class="box" id="status">Подключение...</div>

      <div class="row">
        <div class="box" style="flex:1">Команда: <b id="teamLabel">—</b></div>
        <div class="box" style="flex:1">Вопрос: <b id="roundLabel">—</b></div>
      </div>

      <div id="playerNonBurnRow" class="row hidden">
        <div class="box" style="flex:1">Несгораемые суммы: <b id="playerSafeSums">—</b></div>
      </div>

      <div class="row">
        <div class="box" style="flex:1">Таймер: <b id="timerLabel">—</b></div>
        <div id="myAnswerBox" class="box" style="flex:1">Ваш ответ: <b id="myAnswerLabel">—</b></div>
      </div>

      <div class="answers">
        <button class="answer" data-choice="А">А</button>
        <button class="answer" data-choice="Б">Б</button>
        <button class="answer" data-choice="В">В</button>
        <button class="answer" data-choice="Г">Г</button>
      </div>

      <div class="playerStatsTitle">Ваша статистика по вопросам</div>
      <table class="playerStatsTable">
        <thead id="playerStatsHead"></thead>
        <tbody id="playerStatsBody"></tbody>
      </table>

      <details class="small muted" style="margin-top:14px">
        <summary>Служебное</summary>
        <div style="margin-top:8px">
          <button onclick="changeTeam()" style="background:#3a2e11">Сменить команду</button>
        </div>
      </details>
    </div>
  </div>
</div>

<script>
if('serviceWorker' in navigator){
  navigator.serviceWorker.register('/sw.js').catch(()=>{});
}

const teamIdKey='quiz_team_id';
const teamNameKey='quiz_team_name';

let teamId=localStorage.getItem(teamIdKey)||'';
let teamName=localStorage.getItem(teamNameKey)||'';
let state=null;
let es=null;
let connected=false;
let serverTimeOffsetMs=0;

const $=id=>document.getElementById(id);

if(teamName){
  $('teamName').value=teamName;
}

document.querySelectorAll('.answer').forEach(btn=>{
  btn.onclick=()=>sendAnswer(btn.dataset.choice);
});

function setStatus(text, kind=''){
  const el=$('status');
  el.textContent=text;
  el.className='box';
  if(kind==='ok') el.classList.add('status-ok');
  if(kind==='warn') el.classList.add('status-warn');
  if(kind==='bad') el.classList.add('status-bad');
}

function syncServerClockOffset(data){
  if(!data || typeof data.serverTimeUnix!=='number') return;
  serverTimeOffsetMs = data.serverTimeUnix*1000 - Date.now();
}

function nowByServerClockMs(){
  return Date.now() + serverTimeOffsetMs;
}

async function api(url, method='GET', body=null){
  const res=await fetch(url,{
    method,
    headers:{'Content-Type':'application/json'},
    body:body?JSON.stringify(body):null
  });

  if(!res.ok){
    const txt = await res.text();
    throw new Error(txt || ('HTTP '+res.status));
  }

  const ct=res.headers.get('content-type')||'';
  if(ct.includes('application/json')){
    return res.json();
  }
  return null;
}

async function join(){
  try{
    const name=$('teamName').value.trim();
    if(!name){
      alert('Введите название команды');
      return;
    }

    setStatus('Подключаемся к серверу...', 'warn');

    const data=await api('/api/join','POST',{teamName:name,teamId});

    teamId=data.teamId;
    teamName=data.name;

    localStorage.setItem(teamIdKey,teamId);
    localStorage.setItem(teamNameKey,teamName);

    $('joinBox').classList.add('hidden');
    $('gameBox').classList.remove('hidden');
    $('teamLabel').textContent=teamName;

    connect();
    await refresh();

    setStatus('Подключено', 'ok');
  }catch(e){
    console.error(e);
    setStatus('Ошибка подключения', 'bad');
    alert('Ошибка подключения: ' + (e.message || e));
  }
}

async function refresh(){
  try{
    state=await api('/api/state');
    syncServerClockOffset(state);
    render();
  }catch(e){
    console.error(e);
    setStatus('Сервер недоступен', 'bad');
  }
}

function connect(){
  if(es) es.close();

  es=new EventSource('/events');

  es.onopen=()=>{
    connected=true;
  };

  es.onmessage=e=>{
    try{
      const data=JSON.parse(e.data);
      if(data.type==='keepalive') return;
      state=data;
      syncServerClockOffset(state);
      connected=true;
      render();
    }catch(err){
      console.error(err);
    }
  };

  es.onerror=()=>{
    connected=false;
    setStatus('Соединение потеряно. Пытаемся переподключиться...', 'warn');
  };
}

function changeTeam(){
  if(!confirm('Сменить команду? Текущее подключение будет сброшено.')) return;

  if(es){
    es.close();
    es=null;
  }

  teamId='';
  teamName='';
  localStorage.removeItem(teamIdKey);
  localStorage.removeItem(teamNameKey);

  $('teamName').value='';
  $('joinBox').classList.remove('hidden');
  $('gameBox').classList.add('hidden');
  setStatus('Введите новое название команды', 'warn');
}

function myTeam(){
  if(!state || !state.teams) return null;
  return state.teams.find(t=>t.id===teamId) || null;
}

function resetToJoinBecauseRemoved(){
  if(es){
    es.close();
    es=null;
  }
  teamId='';
  localStorage.removeItem(teamIdKey);
  $('joinBox').classList.remove('hidden');
  $('gameBox').classList.add('hidden');
  setStatus('Команда удалена ведущим. Введите название и подключитесь снова.', 'warn');
}

function render(){
  if(!state) return;

  $('roundLabel').textContent=state.round ? state.round.number : '—';

  const me=myTeam();
  if(teamId && !me){
    resetToJoinBecauseRemoved();
    return;
  }

  if(me && me.name && String(me.name)!==String(teamName||'')){
    teamName=String(me.name);
    localStorage.setItem(teamNameKey, teamName);
    $('teamLabel').textContent=teamName;
    $('teamName').value=teamName;
  }

  const myAns=(me && me.choice) || '—';
  $('myAnswerLabel').textContent=myAns;
  const myAnswerBox=$('myAnswerBox');
  myAnswerBox.classList.remove('myAnswerRight','myAnswerWrong');

  const roundNo=state.round ? Number(state.round.number||0) : 0;
  const rounds=Array.isArray(state.statsRounds) ? state.statsRounds : [];
  const teamStats=Array.isArray(state.teamStats) ? state.teamStats : [];
  const myStats=teamStats.find(ts=>String(ts.teamId||'')===String(teamId||'')) || null;

  const nonBurnEnabled=!!(state.round && state.round.nonBurnMode);
  $('playerNonBurnRow').classList.toggle('hidden', !nonBurnEnabled);

  if(nonBurnEnabled){
    const sums=(myStats && Array.isArray(myStats.safeSums) && myStats.safeSums.length)
      ? myStats.safeSums.join(', ')
      : '—';
    $('playerSafeSums').textContent=sums;
  }else{
    $('playerSafeSums').textContent='—';
  }
  const idx=rounds.indexOf(roundNo);
  const currentResult=(myStats && idx>=0 && Array.isArray(myStats.roundResults)) ? (myStats.roundResults[idx]||'') : '';

  if(state.round.acceptLate){
    if(me && me.answered){
      setStatus('Идёт приём оставшихся ответов. Ваш ответ уже сохранён.', 'ok');
    }else{
      setStatus('Идёт приём оставшихся ответов. Можно ответить без таймера.', 'warn');
    }
  }else if(!state.round.open){
    if(state.round.revealed && state.round.correct){
      if(currentResult==='right'){
        myAnswerBox.classList.add('myAnswerRight');
        setStatus('Вопрос закрыт. Правильный ответ: ' + state.round.correct + '. Ваш ответ: верно.', 'ok');
      }else if(currentResult==='wrong'){
        myAnswerBox.classList.add('myAnswerWrong');
        setStatus('Вопрос закрыт. Правильный ответ: ' + state.round.correct + '. Ваш ответ: неверно.', 'bad');
      }else{
        setStatus('Вопрос закрыт. Правильный ответ: ' + state.round.correct, 'ok');
      }
    }else{
      setStatus('Вопрос закрыт. Ждите следующий вопрос.', 'warn');
    }
  }else{
    if(me && me.answered){
      if(state.round.allowChange){
        setStatus('Ответ принят. Можно изменить до конца вопроса.', 'ok');
      }else{
        setStatus('Ответ принят.', 'ok');
      }
    }else{
      setStatus('Вопрос открыт. Выберите ответ.', 'ok');
    }
  }

  document.querySelectorAll('.answer').forEach(btn=>{
    const canRoundOpen = state.round.open && (!me || !me.answered || state.round.allowChange);
    const canLate = state.round.acceptLate && (!me || !me.answered);
    const can = canRoundOpen || canLate;
    btn.disabled=!can;

    const isMyChoice = myAns===btn.dataset.choice;
    const revealReady = !state.round.open && !!state.round.revealed && !!state.round.correct;
    const isCorrectChoice = state.round.correct===btn.dataset.choice;
    const hasMyAnswer = !!myAns && myAns!=='—';

    btn.classList.toggle('active', isMyChoice && !revealReady);
    btn.classList.toggle('wrong', revealReady && isMyChoice && !isCorrectChoice);
    btn.classList.toggle('correct', revealReady && isCorrectChoice && hasMyAnswer);
    btn.classList.toggle('revealed', revealReady && isCorrectChoice && !hasMyAnswer);
  });

  renderPlayerStats();
}

function renderPlayerStats(){
  const head=$('playerStatsHead');
  const body=$('playerStatsBody');
  head.innerHTML='';
  body.innerHTML='';

  const rounds=Array.isArray(state.statsRounds) ? state.statsRounds : [];
  const teamStats=Array.isArray(state.teamStats) ? state.teamStats : [];
  const myStats=teamStats.find(ts=>String(ts.teamId||'')===String(teamId||'')) || null;

  const trHead=document.createElement('tr');
  trHead.innerHTML='<th>Вопросы</th>' + rounds.map(r=>'<th>'+r+'</th>').join('') + '<th>Счёт</th>';
  head.appendChild(trHead);

  if(!myStats){
    const tr=document.createElement('tr');
    tr.innerHTML='<td colspan="'+(rounds.length+2)+'">Нет данных</td>';
    body.appendChild(tr);
    return;
  }

  const tr=document.createElement('tr');
  const results=Array.isArray(myStats.roundResults) ? myStats.roundResults : [];
  const scoreCell=(state && state.round && state.round.nonBurnMode)
    ? ('<b>'+Number(myStats.totalScore||0)+'</b><div class="small">след.: '+Number(myStats.nextScore||0)+'</div>')
    : ('<b>'+Number(myStats.totalScore||0)+'</b>');
  tr.innerHTML='<td>Результат</td>' +
    rounds.map((_,i)=>'<td>'+playerStatusMark(results[i]||'noanswer')+'</td>').join('') +
    '<td>'+scoreCell+'</td>';
  body.appendChild(tr);
}

function playerStatusMark(status){
  if(status==='right') return '✅';
  if(status==='wrong') return '❌';
  if(status==='pending') return '⏳';
  return '—';
}

async function sendAnswer(choice){
  try{
    if(!teamId){
      alert('Сначала подключитесь как команда');
      return;
    }
    await api('/api/answer','POST',{teamId,choice});
    await refresh();
  }catch(e){
    console.error(e);
    alert('Ошибка отправки ответа: ' + (e.message || e));
  }
}

setInterval(async()=>{
  if(teamId){
    try{
      await api('/api/ping','POST',{teamId});
    }catch(e){
      const msg=String((e && e.message) || e || '');
      if(msg.includes('unknown team')){
        resetToJoinBecauseRemoved();
        return;
      }
      console.error('Ping error', e);
    }
  }
},5000);

setInterval(()=>{
  if(!state || !state.round || !state.round.open || !state.round.closesAt){
    $('timerLabel').textContent='—';
    return;
  }

  const end=new Date(state.round.closesAt).getTime();
  let left=Math.max(0,Math.floor((end-nowByServerClockMs())/1000));
  $('timerLabel').textContent=
    String(Math.floor(left/60)).padStart(2,'0')+':' +
    String(left%60).padStart(2,'0');
},250);

if(teamId && teamName){
  $('joinBox').classList.add('hidden');
  $('gameBox').classList.remove('hidden');
  $('teamLabel').textContent=teamName;
  setStatus('Восстанавливаем подключение...', 'warn');
  connect();
  refresh();
}else{
  refresh();
}
</script>
</body>
</html>`

const hostHTML = `<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>LAN Quiz — Ведущий</title>
<style>
body{
  margin:0;
  font-family:Arial,Helvetica,sans-serif;
  background:#09111f;
  color:#fff;
  padding:18px;
}
.wrap{
  width:100%;
  max-width:none;
  margin:0;
}
.grid{
  display:grid;
  grid-template-columns:max-content minmax(0, 1fr);
  gap:16px;
}
@media (max-width: 980px){
  .grid{grid-template-columns:1fr}
}
.card{
  background:#121b31;
  border-radius:20px;
  padding:18px;
  box-shadow:0 10px 30px rgba(0,0,0,.25);
}
.hostControlsCard{
  width:max-content;
  max-width:100%;
}
input,button{
  font-size:16px;
  border-radius:12px;
  padding:12px;
}
input{
  width:100%;
  box-sizing:border-box;
  border:1px solid #2b3c6a;
  background:#0c1430;
  color:#fff;
}
#duration{
  width:220px;
}
button{
  border:none;
  cursor:pointer;
}
.row{
  display:flex;
  gap:10px;
  flex-wrap:wrap;
}
.checkRow label{
  display:flex;
  align-items:center;
  gap:8px;
  line-height:1.2;
  white-space:nowrap;
}
.kpis{
  display:grid;
  grid-template-columns:repeat(4,1fr);
  gap:12px;
}
.kpi{
  background:#0c1530;
  border-radius:14px;
  padding:12px;
}
table{
  width:100%;
  border-collapse:collapse;
}
th,td{
  padding:10px;
  border-bottom:1px solid #28385f;
  text-align:left;
}
.hidden{
  display:none;
}
.status{
  margin-top:12px;
  padding:10px 12px;
  border-radius:12px;
  background:#1c2542;
}
#hostStatus{
  display:inline-block;
}
.qrBox{
  margin-top:16px;
}
.detailsBlock{
  margin-top:12px;
}
.detailsBlock summary{
  cursor:pointer;
  font-weight:700;
  color:#d6e3ff;
}
.detailsBlock[open] summary{
  margin-bottom:10px;
}
.qrWrap{
  background:#fff;
  padding:12px;
  border-radius:14px;
  display:inline-block;
}
.qrWrap img{
  display:block;
  width:160px;
  height:160px;
}
.small{
  font-size:12px;
  color:#aab7dd;
}
.teamSubHint{
  display:block;
  margin-top:3px;
  font-size:11px;
  color:#97a0b8;
  font-weight:400;
}
.linkText{
  word-break:break-all;
}
.revealBtn{
  min-width:64px;
  font-weight:800;
  background:#1f2d57;
  color:#fff;
  border:2px solid #4a5f9b;
}
.revealBtn.selected{
  outline:3px solid #88ffd0;
  background:#14503e;
  border-color:#88ffd0;
}
.controlGroup{
  margin-top:14px;
  padding:12px;
  border-radius:14px;
  border:1px solid #2f4270;
}
.controlGroup h3{
  margin:0 0 10px;
  font-size:14px;
  letter-spacing:.02em;
  color:#d6e3ff;
}
.roundGroup{
  background:#0d1b38;
  width:max-content;
  max-width:100%;
}
.roundGroup > .row,
.roundGroup > details{
  width:max-content;
  max-width:100%;
}
.answerGroup{
  background:#2b1a3f;
  border-color:#6c4c99;
  width:max-content;
  max-width:100%;
}
.answerGroup .row{
  flex-wrap:nowrap;
  width:max-content;
}
.roundBtn{
  font-weight:800;
  color:#fff;
}
.roundBtn:disabled{
  background:#6b7280 !important;
  color:#e5e7eb;
  cursor:not-allowed;
  opacity:.95;
}
.roundMainRow{
  flex-wrap:nowrap;
}
.roundMainRow .roundBtn{
  flex:1;
  min-width:0;
  white-space:nowrap;
}
.roundBtn.open{
  background:#0c8a4a;
}
.roundBtn.close{
  background:#c3561f;
}
.roundBtn.next{
  background:#3f5ee6;
}
.roundBtn.reset{
  background:#a12b47;
}
.roundBtn.prev{
  background:#4b5563;
  font-size:13px;
  padding:8px 10px;
}
.statsTitle{
  margin:18px 0 8px;
  font-size:18px;
  color:#d6e3ff;
}
.statsTable th,
.statsTable td{
  text-align:center;
}
.hostStatsTable th{
  padding-right:16px;
}
.statsTable th:first-child,
.statsTable td:first-child{
  text-align:left;
}
.hostStatsTable th:first-child,
.hostStatsTable td:first-child{
  width:56px;
  min-width:56px;
  max-width:56px;
  text-align:center;
}
.hostStatsTable th:nth-child(2),
.hostStatsTable td:nth-child(2){
  width:1%;
  white-space:nowrap;
  text-align:left;
}
.actionMenu{
  position:relative;
  display:inline-block;
}
.actionMenu summary{
  list-style:none;
  cursor:pointer;
  user-select:none;
  font-size:20px;
  line-height:1;
  padding:4px 8px;
  border-radius:8px;
  background:#1f2d57;
  color:#fff;
  border:1px solid #4a5f9b;
}
.actionMenu summary::-webkit-details-marker{
  display:none;
}
.actionMenuPanel{
  position:absolute;
  left:0;
  top:calc(100% + 6px);
  z-index:5;
  display:flex;
  flex-direction:column;
  gap:6px;
  min-width:140px;
  padding:8px;
  border-radius:10px;
  border:1px solid #2f4270;
  background:#0f1730;
  box-shadow:0 8px 24px rgba(0,0,0,.35);
}
.actionMenuBtn{
  color:#fff;
  padding:8px 10px;
  font-size:13px;
}
.actionMenuBtn.rename{
  background:#234566;
}
.actionMenuBtn.remove{
  background:#5a1f2d;
}
.statsCell.right{
  background:#123322;
}
.statsCell.wrong{
  background:#441c24;
}
.statsCell.pending{
  background:#3a2e11;
}
</style>
</head>
<body>
<div class="wrap">
  <div class="grid">
    <div class="card hostControlsCard">
      <details class="detailsBlock">
        <summary>Сетевые ссылки и QR</summary>
        <div>IP в локальной сети: <span id="lanIps" class="linkText">—</span></div>
        <div style="margin-top:8px">
          IP для QR:
          <select id="lanIpSelect" onchange="saveLanIPChoice()" style="margin-left:8px"></select>
        </div>
        <div>Игроки: <span id="playerLink" class="linkText">—</span></div>
        <div>Ведущий: <span id="hostLink" class="linkText">—</span></div>
        <div>Проектор: <span id="screenLink" class="linkText">—</span></div>

        <div class="qrBox">
          <div style="margin-bottom:8px">QR для игроков</div>
          <div class="qrWrap">
            <img id="qrImg" alt="QR code">
          </div>
          <div class="small" style="margin-top:8px">
            Игроки могут открыть страницу, отсканировав QR-код.
          </div>
        </div>

      </details>

      <div class="row checkRow" style="margin-top:10px">
        <label><input id="showScreenQR" type="checkbox" onchange="setScreenQRVisible()"> Показывать QR на экране /screen</label>
      </div>

      <div id="secretBox" class="hidden" style="margin-top:12px">
        <input id="secretInput" placeholder="Host secret">
        <button onclick="saveSecret()" style="margin-top:8px">OK</button>
      </div>

      <div style="margin-top:14px">Длительность (сек)</div>
      <input id="duration" type="number" value="60">

      <div class="row checkRow" style="margin-top:10px">
        <label><input id="allowChange" type="checkbox" checked> Можно менять ответ</label>
      </div>
      <div class="row checkRow" style="margin-top:10px">
        <label><input id="nonBurnMode" type="checkbox" onchange="setNonBurnMode()"> Раунд с несгораемыми суммами</label>
      </div>
      <div class="controlGroup roundGroup">
        <h3>Управление вопросом</h3>
        <div class="controlGroup answerGroup" style="margin-top:0">
          <h3>Правильный ответ</h3>
          <div class="row">
            <button class="revealBtn" data-choice="А" onclick="reveal('А')">А</button>
            <button class="revealBtn" data-choice="Б" onclick="reveal('Б')">Б</button>
            <button class="revealBtn" data-choice="В" onclick="reveal('В')">В</button>
            <button class="revealBtn" data-choice="Г" onclick="reveal('Г')">Г</button>
          </div>
        </div>

        <div class="row" style="margin-top:10px">
          <button id="nextRoundBtn" class="roundBtn next" onclick="nextRound()">Следующий вопрос</button>
        </div>
        <div class="row" style="margin-top:10px">
          <button class="roundBtn close" onclick="stopRound()">Остановить</button>
        </div>
        <div class="row" style="margin-top:10px">
          <button id="acceptLateBtn" class="roundBtn close" onclick="acceptLateAnswers()" disabled>Принять оставшиеся ответы</button>
        </div>
        <div class="row" style="margin-top:10px">
          <button class="roundBtn close" onclick="closeRound()">Завершить</button>
        </div>

        <details class="detailsBlock" style="margin-top:10px">
          <summary>Дополнительные действия</summary>
          <div class="row roundMainRow" style="margin-top:10px">
            <button class="roundBtn open" onclick="openRound()">Начать</button>
          </div>
          <div class="row" style="margin-top:10px">
            <button class="roundBtn prev" onclick="prevRound()">Предыдущий вопрос</button>
          </div>
          <div class="row" style="margin-top:10px">
            <button class="roundBtn close" onclick="replayRound()">Переиграть вопрос</button>
          </div>
          <div class="row" style="margin-top:10px">
            <button class="roundBtn reset" onclick="fullReset()">Сброс статистики и новый раунд</button>
          </div>
        </details>
      </div>

      <div id="hostStatus" class="status" style="margin-top:16px">Подключение...</div>

    </div>

    <div class="card">
      <div class="kpis">
        <div class="kpi">Вопрос<br><b id="roundNo">—</b></div>
        <div class="kpi">Онлайн<br><b id="onlineCount">0</b></div>
        <div class="kpi">Ответили<br><b id="answeredCount">0</b></div>
        <div class="kpi">Таймер<br><b id="timer">—</b></div>
      </div>

      <div id="roundStatus" style="margin-top:10px">—</div>
      <div id="correctBox" style="margin-top:6px"></div>

      <div class="statsTitle">Команды и статистика по вопросам</div>
      <table class="statsTable hostStatsTable">
        <thead id="statsHead"></thead>
        <tbody id="statsBody"></tbody>
      </table>
    </div>
  </div>

</div>

<script>
let state=null;
let es=null;
let hostSecret=localStorage.getItem('quiz_host_secret')||'';
let selectedLanIP=localStorage.getItem('quiz_lan_ip')||'';
let hostServerTimeOffsetMs=0;

const $=id=>document.getElementById(id);

function setHostStatus(text){
  $('hostStatus').textContent=text;
}

function syncHostServerClockOffset(data){
  if(!data || typeof data.serverTimeUnix!=='number') return;
  hostServerTimeOffsetMs = data.serverTimeUnix*1000 - Date.now();
}

function hostNowByServerClockMs(){
  return Date.now() + hostServerTimeOffsetMs;
}

function isLoopbackHost(){
  const host=window.location.hostname;
  return host==='localhost' || host==='127.0.0.1' || host==='::1';
}

function headers(){
  const h={'Content-Type':'application/json'};
  if(hostSecret) h['X-Host-Secret']=hostSecret;
  return h;
}

async function api(url, method='GET', body=null){
  const res=await fetch(url,{
    method,
    headers:headers(),
    body:body?JSON.stringify(body):null
  });

  if(res.status===403){
    $('secretBox').classList.remove('hidden');
    throw new Error('Нужен host secret');
  }

  if(!res.ok){
    throw new Error(await res.text());
  }

  const ct=res.headers.get('content-type')||'';
  if(ct.includes('application/json')) return res.json();
  return null;
}

function saveSecret(){
  hostSecret=$('secretInput').value.trim();
  localStorage.setItem('quiz_host_secret', hostSecret);
  $('secretBox').classList.add('hidden');
  connect();
  refresh();
}

async function refresh(){
  try{
    const qs=hostSecret?('?secret='+encodeURIComponent(hostSecret)):'';
    const res=await fetch('/api/state'+qs,{headers:headers()});

    if(res.status===403){
      $('secretBox').classList.remove('hidden');
      setHostStatus('Требуется host secret');
      return;
    }

    state=await res.json();
    syncHostServerClockOffset(state);
    render();
    setHostStatus('Подключено');
  }catch(e){
    console.error(e);
    setHostStatus('Ошибка получения состояния сервера');
  }
}

function connect(){
  if(es) es.close();

  const qs=hostSecret?('?secret='+encodeURIComponent(hostSecret)):'';
  es=new EventSource('/events'+qs);

  es.onopen=()=>{
    setHostStatus('Подключено');
  };

  es.onmessage=e=>{
    try{
      const data=JSON.parse(e.data);
      if(data.type==='keepalive') return;
      state=data;
      syncHostServerClockOffset(state);
      render();
      setHostStatus('Подключено');
    }catch(err){
      console.error(err);
    }
  };

  es.onerror=()=>{
    setHostStatus('Потеряно соединение с сервером');
  };
}

async function saveLanIPChoice(){
  selectedLanIP=$('lanIpSelect').value;
  localStorage.setItem('quiz_lan_ip', selectedLanIP);
  render();
  try{
    await api('/api/host/screen-qr','POST',{
      show:$('showScreenQR').checked,
      lanIP:selectedOrAutoLanIP()
    });
  }catch(e){
    alert('Ошибка: ' + (e.message || e));
  }
}

function syncLanIPSelect(){
  const sel=$('lanIpSelect');
  if(!sel) return;

  const ips=(state && Array.isArray(state.ipHints)) ? state.ipHints : [];
  const hasSelected=selectedLanIP && ips.includes(selectedLanIP);
  if(selectedLanIP && !hasSelected){
    selectedLanIP='';
    localStorage.setItem('quiz_lan_ip','');
  }

  const oldVal=sel.value;
  sel.innerHTML='';

  const autoOpt=document.createElement('option');
  autoOpt.value='';
  autoOpt.textContent='Авто (первый IP)';
  sel.appendChild(autoOpt);

  for(const ip of ips){
    const opt=document.createElement('option');
    opt.value=ip;
    opt.textContent=ip;
    sel.appendChild(opt);
  }

  if(selectedLanIP && ips.includes(selectedLanIP)){
    sel.value=selectedLanIP;
  }else if(oldVal && ips.includes(oldVal)){
    sel.value=oldVal;
  }else{
    sel.value='';
  }
}

function selectedOrAutoLanIP(){
  const ips=(state && Array.isArray(state.ipHints)) ? state.ipHints : [];
  if(selectedLanIP && ips.includes(selectedLanIP)) return selectedLanIP;
  if(ips.length>0) return ips[0];
  return '';
}

function shareBaseURL(){
  const origin=window.location.origin;
  if(!isLoopbackHost()){
    return origin;
  }

  const ip=selectedOrAutoLanIP();
  if(ip){
    const proto=window.location.protocol;
    const port=window.location.port ? (':'+window.location.port) : '';
    return proto+'//'+ip+port;
  }

  return origin;
}

function render(){
  if(!state) return;

  $('roundNo').textContent=state.round.number;
  $('onlineCount').textContent=state.onlineCount;
  $('answeredCount').textContent=state.answeredCount;
  const totalTeams=Array.isArray(state.teams) ? state.teams.length : 0;
  const allAnswered=totalTeams===0 || state.answeredCount>=totalTeams;
  const canAcceptLate = !state.round.open && !state.round.acceptLate && totalTeams>0 && !allAnswered;
  const canGoNextRound = !state.round.open && !state.round.acceptLate && !!state.round.revealed && !!state.round.correct;
  $('acceptLateBtn').disabled = !canAcceptLate;
  $('nextRoundBtn').disabled = !canGoNextRound;
  if(state.round.acceptLate){
    $('roundStatus').textContent='Приём оставшихся ответов (без таймера)';
  }else{
    $('roundStatus').textContent=state.round.open ? 'Вопрос открыт' : 'Вопрос закрыт';
  }
  if(state.round.revealed && state.round.correct){
    const counts={ 'А':0, 'Б':0, 'В':0, 'Г':0 };
    const teams=Array.isArray(state.teams) ? state.teams : [];
    for(const t of teams){
      const c=String((t && t.choice) || '').trim().toUpperCase();
      if(Object.prototype.hasOwnProperty.call(counts, c)) counts[c]++;
    }

    const stats=Object.entries(counts)
      .filter(([,n])=>n>0)
      .sort((a,b)=>b[1]-a[1] || a[0].localeCompare(b[0],'ru'))
      .map(([choice,n])=>choice+': '+n)
      .join(', ');

    $('correctBox').textContent='Правильный ответ: '+state.round.correct + (stats ? (' | Ответы: '+stats) : '');
  }else{
    $('correctBox').textContent='';
  }
  $('allowChange').checked=!!state.round.allowChange;
  $('nonBurnMode').checked=!!state.round.nonBurnMode;
  $('showScreenQR').checked=!!state.round.showScreenQR;

  const base=shareBaseURL();
  syncLanIPSelect();
  $('lanIps').textContent=(state.ipHints && state.ipHints.length)
    ? state.ipHints.join(', ')
    : 'не найден';
  const playerURL=base+'/';
  $('playerLink').textContent=playerURL;
  $('hostLink').textContent=base+'/host';
  $('screenLink').textContent=base+'/screen';
  $('qrImg').src='/qr.png?text='+encodeURIComponent(playerURL);

  document.querySelectorAll('.revealBtn').forEach(btn=>{
    const isSelected = !!state.round.correct && state.round.correct===btn.dataset.choice;
    btn.classList.toggle('selected', isSelected);
    btn.disabled = !state.round.open && !!state.round.revealed;
  });

  renderStats();
}

function renderStats(){
  const head=$('statsHead');
  const body=$('statsBody');
  head.innerHTML='';
  body.innerHTML='';

  const rounds=Array.isArray(state.statsRounds) ? state.statsRounds : [];
  const teamsSorted = Array.isArray(state.teams)
    ? [...state.teams].sort((a,b)=>String(a.name||'').localeCompare(String(b.name||''),'ru',{sensitivity:'base'}))
    : [];
  const teamStats=Array.isArray(state.teamStats) ? state.teamStats : [];
  const statsByTeamId=new Map(teamStats.map(ts=>[String(ts.teamId||''), ts]));

  const trHead=document.createElement('tr');
  trHead.innerHTML='<th>Действие</th><th>Команда</th><th>Ответ</th>' +
    rounds.map(r=>'<th>'+r+'</th>').join('') +
    '<th>'+(state && state.round && state.round.nonBurnMode ? 'Счёт / След.' : 'Счёт')+'</th>';
  head.appendChild(trHead);

  if(teamsSorted.length===0){
    const tr=document.createElement('tr');
    tr.innerHTML='<td colspan="'+(rounds.length+6)+'">Нет команд</td>';
    body.appendChild(tr);
    return;
  }

  for(const t of teamsSorted){
    const ts=statsByTeamId.get(String(t.id||'')) || null;
    const tr=document.createElement('tr');
    const teamColor=t.online ? '#4ade80' : '#9ca3af';
    const safeHint=(state && state.round && state.round.nonBurnMode)
      ? ((ts && Array.isArray(ts.safeSums) && ts.safeSums.length)
          ? ('<span class="teamSubHint">несгораемые: '+escapeHtml(ts.safeSums.join(', '))+'</span>')
          : '<span class="teamSubHint">несгораемые: —</span>')
      : '';
    tr.innerHTML=
      '<td><span style="color:'+teamColor+'">'+escapeHtml(t.name||'—')+'</span>'+safeHint+'</td>'+
      '<td>'+(t.choice?escapeHtml(t.choice):'—')+'</td>';

    const results=ts && Array.isArray(ts.roundResults) ? ts.roundResults : [];
    for(let i=0;i<rounds.length;i++){
      const status=results[i] || 'noanswer';
      const td=document.createElement('td');
      td.className='statsCell ' + (status==='right' || status==='wrong' || status==='pending' ? status : '');
      td.textContent = statusToMark(status);
      tr.appendChild(td);
    }

    const scoreTd=document.createElement('td');
    if(state && state.round && state.round.nonBurnMode){
      const current=Number(ts ? ts.totalScore : 0);
      const next=Number(ts ? ts.nextScore : 0);
      scoreTd.innerHTML='<b>'+current+'</b><div class="small">след.: '+next+'</div>';
    }else{
      scoreTd.innerHTML='<b>'+Number(ts ? ts.totalScore : 0)+'</b>';
    }
    tr.appendChild(scoreTd);

    const actionTd=document.createElement('td');
    actionTd.innerHTML=''+
      '<details class="actionMenu">'+
        '<summary title="Действия">⋮</summary>'+
        '<div class="actionMenuPanel"></div>'+
      '</details>';

    const panel=actionTd.querySelector('.actionMenuPanel');

    const renameBtn=document.createElement('button');
    renameBtn.className='actionMenuBtn rename';
    renameBtn.textContent='Переименовать';
    renameBtn.onclick=()=>renameTeam(t.id, t.name);
    panel.appendChild(renameBtn);

    const removeBtn=document.createElement('button');
    removeBtn.className='actionMenuBtn remove';
    removeBtn.textContent='Удалить';
    removeBtn.onclick=()=>removeTeam(t.id, t.name);
    panel.appendChild(removeBtn);

    if(state && state.round && state.round.nonBurnMode){
      const safeSumsBtn=document.createElement('button');
      safeSumsBtn.className='actionMenuBtn rename';
      safeSumsBtn.textContent='Несгораемые суммы';
      safeSumsBtn.onclick=()=>editSafeSums(t);
      panel.appendChild(safeSumsBtn);
    }

    tr.insertBefore(actionTd, tr.firstChild);

    body.appendChild(tr);
  }
}

function statusToMark(status){
  if(status==='right') return '✅';
  if(status==='wrong') return '❌';
  if(status==='pending') return '⏳';
  return '—';
}

function escapeHtml(s){
  return String(s)
    .replaceAll('&','&amp;')
    .replaceAll('<','&lt;')
    .replaceAll('>','&gt;')
    .replaceAll('"','&quot;')
    .replaceAll("'","&#039;");
}

async function openRound(){
  try{
    await api('/api/host/open','POST',{
      durationSec:parseInt($('duration').value||'0',10)||0,
      allowChange:$('allowChange').checked,
      showScreenQR:$('showScreenQR').checked
    });
  }catch(e){
    alert('Ошибка: ' + (e.message || e));
  }
}

async function setScreenQRVisible(){
  try{
    await api('/api/host/screen-qr','POST',{
	  show:$('showScreenQR').checked,
	  lanIP:selectedOrAutoLanIP()
    });
  }catch(e){
    alert('Ошибка: ' + (e.message || e));
  }
}

async function setNonBurnMode(){
  try{
    await api('/api/host/non-burn-mode','POST',{
      enabled:$('nonBurnMode').checked
    });
  }catch(e){
    alert('Ошибка: ' + (e.message || e));
  }
}

async function editSafeSums(team){
  if(!team || !team.id) return;
  const current=Array.isArray(team.safeSums) ? team.safeSums.join(', ') : '';
  const raw=prompt('До 3 несгораемых сумм через запятую (из таблицы: 100,200,300,500,700,1000,2000,3000,5000,10000,15000,20000,30000)', current);
  if(raw===null) return;

  const safeSums=String(raw)
    .split(',')
    .map(s=>parseInt(String(s).trim(),10))
    .filter(n=>Number.isFinite(n) && n>0);

  try{
    await api('/api/host/team/safe-sums','POST',{teamId:team.id,safeSums});
  }catch(e){
    alert('Ошибка сохранения несгораемых сумм: ' + (e.message || e));
  }
}

async function closeRound(){
  try{
    await api('/api/host/close','POST',{});
  }catch(e){
    alert('Ошибка: ' + (e.message || e));
  }
}

async function stopRound(){
  try{
    await api('/api/host/stop','POST',{});
  }catch(e){
    alert('Ошибка: ' + (e.message || e));
  }
}

async function acceptLateAnswers(){
  try{
    await api('/api/host/accept-late','POST',{});
  }catch(e){
    alert('Ошибка: ' + (e.message || e));
  }
}

async function nextRound(){
  try{
    await api('/api/host/reset','POST',{full:false});
  }catch(e){
    alert('Ошибка: ' + (e.message || e));
  }
}

async function prevRound(){
  try{
    await api('/api/host/prev-round','POST',{});
  }catch(e){
    alert('Ошибка: ' + (e.message || e));
  }
}

async function fullReset(){
  if(!confirm('Сбросить статистику, очистить историю и вернуть счётчик вопросов к 1?')) return;
  try{
    const res=await api('/api/host/reset','POST',{full:true});
    const csvPath=(res && res.csvPath) ? String(res.csvPath) : '';
    setHostStatus(csvPath ? ('Статистика сохранена: '+csvPath) : 'Статистика сброшена');
  }catch(e){
    alert('Ошибка: ' + (e.message || e));
  }
}

async function reveal(c){
  try{
    if(state && state.round && !state.round.open && state.round.revealed){
      alert('Вопрос уже сыгран. Нажмите «Переиграть вопрос», чтобы выбрать правильный ответ заново.');
      return;
    }
    if(state && state.round && !state.round.open){
      await api('/api/host/open','POST',{
        durationSec:parseInt($('duration').value||'0',10)||0,
        allowChange:$('allowChange').checked,
        showScreenQR:$('showScreenQR').checked
      });
    }
    await api('/api/host/reveal','POST',{correct:c});
  }catch(e){
    alert('Ошибка: ' + (e.message || e));
  }
}

async function replayRound(){
  if(!confirm('Переиграть текущий вопрос? Ответы и проверка этого вопроса будут очищены.')) return;
  try{
    await api('/api/host/replay-round','POST',{});
  }catch(e){
    alert('Ошибка: ' + (e.message || e));
  }
}

async function removeTeam(teamId, teamName){
  if(!teamId) return;
  if(!confirm('Удалить команду "'+teamName+'" из списка?')) return;
  try{
    await api('/api/host/team/remove','POST',{teamId});
  }catch(e){
    alert('Ошибка удаления команды: ' + (e.message || e));
  }
}

async function renameTeam(teamId, currentName){
  if(!teamId) return;
  const newName=prompt('Новое название команды:', String(currentName||''));
  if(newName===null) return;
  const teamName=String(newName).trim();
  if(!teamName){
    alert('Название команды не может быть пустым');
    return;
  }
  try{
    await api('/api/host/team/rename','POST',{teamId,teamName});
  }catch(e){
    alert('Ошибка переименования команды: ' + (e.message || e));
  }
}

setInterval(()=>{
  if(!state || !state.round.open || !state.round.closesAt){
    $('timer').textContent='—';
    return;
  }
  const end=new Date(state.round.closesAt).getTime();
  let left=Math.max(0,Math.floor((end-hostNowByServerClockMs())/1000));
  $('timer').textContent=
    String(Math.floor(left/60)).padStart(2,'0')+':' +
    String(left%60).padStart(2,'0');
},250);

connect();
refresh();
</script>
</body>
</html>`

const screenHTML = `<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>LAN Quiz — Экран</title>
<style>
body{
  margin:0;
  font-family:Arial,Helvetica,sans-serif;
  background:#050814;
  color:#fff;
  padding:24px;
}
.small{
  color:#aab7dd;
}
.hidden{
  display:none;
}
.statsTitle{
  margin:16px 0 10px;
  font-size:24px;
  color:#d6e3ff;
}
.statsTable{
  width:100%;
  border-collapse:collapse;
  background:#111a2f;
  border-radius:14px;
  overflow:hidden;
}
.statsTable th,
.statsTable td{
  border-bottom:1px solid #2b3c6a;
  padding:12px;
  text-align:center;
  font-size:22px;
}
.statsTable th:first-child,
.statsTable td:first-child{
  text-align:left;
}
.qrBlock{
  margin-top:18px;
  text-align:center;
}
.qrWrap{
  display:inline-block;
  background:#fff;
  border-radius:12px;
  padding:10px;
}
.qrWrap img{
  display:block;
  width:220px;
  height:220px;
}
</style>
</head>
<body>
<div class="statsTitle">Статистика команд</div>
<table class="statsTable">
  <thead id="screenStatsHead"></thead>
  <tbody id="screenStatsBody"></tbody>
</table>
<div class="qrBlock">
  <div class="small" style="font-size:20px;margin-bottom:8px">Подключение игроков</div>
  <div class="qrWrap">
    <img id="playerQr" alt="QR для страницы игроков">
  </div>
  <div id="playerUrl" class="small" style="font-size:16px;margin-top:8px">—</div>
</div>

<script>
let state=null;
let es=new EventSource('/events');

function isLoopbackHost(){
  const h=window.location.hostname;
  return h==='localhost' || h==='127.0.0.1' || h==='::1';
}

function playerURLForShare(){
  const origin=window.location.origin;
  if(!state || !Array.isArray(state.ipHints)) return origin + '/';

  if(state.round && state.round.lanIP && state.ipHints.includes(state.round.lanIP)){
    const proto=window.location.protocol;
    const port=window.location.port ? (':'+window.location.port) : '';
    return proto+'//'+state.round.lanIP+port+'/';
  }

  if(!isLoopbackHost()) return origin + '/';

  if(state.ipHints.length>0){
    const proto=window.location.protocol;
    const port=window.location.port ? (':'+window.location.port) : '';
    return proto+'//'+state.ipHints[0]+port+'/';
  }

  return origin + '/';
}

es.onmessage=e=>{
  const data=JSON.parse(e.data);
  if(data.type==='keepalive') return;
  state=data;
  render();
};

function escapeHtml(s){
  return String(s)
    .replaceAll('&','&amp;')
    .replaceAll('<','&lt;')
    .replaceAll('>','&gt;')
    .replaceAll('"','&quot;')
    .replaceAll("'","&#039;");
}

function render(){
  if(!state) return;

  renderScreenStats();

  const playerURL=playerURLForShare();
  document.getElementById('playerQr').src='/qr.png?text='+encodeURIComponent(playerURL);
  document.getElementById('playerUrl').textContent=playerURL;
  document.querySelector('.qrBlock').classList.toggle('hidden', !state.round.showScreenQR);
}

function renderScreenStats(){
  const head=document.getElementById('screenStatsHead');
  const body=document.getElementById('screenStatsBody');
  head.innerHTML='';
  body.innerHTML='';

  const rounds=Array.isArray(state.statsRounds) ? state.statsRounds : [];
  const teamStats=Array.isArray(state.teamStats)
    ? [...state.teamStats].sort((a,b)=>String(a.teamName||'').localeCompare(String(b.teamName||''),'ru',{sensitivity:'base'}))
    : [];

  const trHead=document.createElement('tr');
  trHead.innerHTML='<th>Команда</th>' + rounds.map(()=>'<th></th>').join('') + '<th>Счёт</th>';
  head.appendChild(trHead);

  if(teamStats.length===0){
    const tr=document.createElement('tr');
    tr.innerHTML='<td colspan="'+(rounds.length+2)+'">Нет данных</td>';
    body.appendChild(tr);
    return;
  }

  for(const ts of teamStats){
    const tr=document.createElement('tr');
    tr.innerHTML='<td>'+escapeHtml(ts.teamName||'—')+'</td>';

    const results=Array.isArray(ts.roundResults) ? ts.roundResults : [];
    for(let i=0;i<rounds.length;i++){
      const status=results[i] || 'noanswer';
      const td=document.createElement('td');
      td.textContent = statusMark(status);
      tr.appendChild(td);
    }

    const scoreTd=document.createElement('td');
    scoreTd.innerHTML='<b>'+Number(ts.totalScore||0)+'</b>';
    tr.appendChild(scoreTd);
    body.appendChild(tr);
  }
}

function statusMark(status){
  if(status==='right') return '✅';
  if(status==='wrong') return '❌';
  if(status==='pending') return '⏳';
  return '—';
}
</script>
</body>
</html>`
