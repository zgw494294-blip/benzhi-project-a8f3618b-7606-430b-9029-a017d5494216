const state={cases:[],current:null,checkFilter:'all',pendingOrder:null,pendingOrderKey:null};
const $=selector=>document.querySelector(selector);
const statusLabel={draft:'草拟',review:'待审查',remediation:'整改中',pending_approval:'待批准',effective:'已生效'};
const severityLabel={blocking:'阻断',major:'重大',minor:'轻微'};
const queueLabel={unlinked:'未关联整改',unchanged:'整改内容未变化',pending_review:'待复核',verified:'已验证',returned:'已退回'};
const noticeLabel={current:'当前有效',expired:'已过期',integrity_anomaly:'完整性异常',not_matched:'校验码未匹配'};
const key=()=>crypto.randomUUID();
const h=value=>String(value??'').replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));

async function api(path,options={}){
  const headers={...(options.body?{'Content-Type':'application/json'}:{}),...(options.headers||{})};
  const response=await fetch(path,{...options,headers});
  const body=await response.json();
  if(!response.ok){const error=new Error(body.error?.message||'请求失败');Object.assign(error,body.error||{});throw error;}
  return body;
}
function meta(idempotencyKey=key()){return{expectedVersion:state.current.case.version,idempotencyKey,actor:'浏览器工作台用户'};}
function flash(message,error=false){const element=$('#flash');if(!element)return;element.textContent=message;element.className=error?'error':'';}

async function loadCases(){
  const body=await api('/api/cases');state.cases=body.data||[];
  $('#case-list').innerHTML=state.cases.map(item=>`<button data-case-id="${h(item.id)}"><b>${h(item.revisionNumber)}</b><small>${h(item.manualNumber)} · ${h(statusLabel[item.status])}</small></button>`).join('')||'<p class="muted">暂无任务</p>';
  document.querySelectorAll('[data-case-id]').forEach(button=>button.onclick=()=>openCase(button.dataset.caseId));
}
async function openCase(id){const body=await api('/api/cases/'+encodeURIComponent(id));state.current=body.data;render();}

function render(){
  const view=state.current,revision=view.case,allowed=new Set(view.allowedActions);
  $('#empty').hidden=true;$('#detail').hidden=false;$('#case-number').textContent=revision.revisionNumber;$('#case-title').textContent=revision.manualNumber+' / '+revision.baselineEdition;$('#status').textContent=statusLabel[revision.status];
  $('#meta').innerHTML=`<p><b>机型：</b>${revision.aircraftModels.map(h).join('、')}</p><p><b>构型：</b>${h(revision.configurationScope)}</p><p><b>负责人：</b>${h(revision.owner)}</p><p><b>原因：</b>${h(revision.reason)}</p><p><b>有效期：</b>${new Date(revision.effectiveUntil).toLocaleString()}</p><p><b>并发版本：</b>${revision.version}　<b>修订轮次：</b>${revision.currentRevision}</p>`;
  const order=['draft','review','remediation','pending_approval','effective'],at=order.indexOf(revision.status);$('#steps').innerHTML=order.map((_,index)=>`<i class="step ${index<=at?'done':''}"></i>`).join('');
  renderChecks(view);renderBlocks(view,allowed);renderFindings(view,allowed);renderNotice(revision);
  document.querySelectorAll('[data-action]').forEach(button=>{button.hidden=!allowed.has(button.dataset.action);});
  document.querySelectorAll('[data-requires]').forEach(button=>{button.hidden=!allowed.has(button.dataset.requires);});
  const submit=$('[data-action="submit"]');if(submit)submit.disabled=!view.readiness.ready;
}

function renderChecks(view){
  const readiness=view.readiness;
  $('#readiness').innerHTML=`<div class="readiness ${readiness.ready?'pass':readiness.stale?'stale':'fail'}"><b>${readiness.ready?'可送审':readiness.stale?'校核结果已失效':'暂不可送审'}</b><span>${readiness.passedCount} 项通过 · ${readiness.failedCount} 项失败</span></div>`+(readiness.blockers||[]).map(item=>`<button class="blocker" data-focus-block="${h(item.blockId||'')}"><b>${h(item.code)}</b> ${item.blockSequence?`#${item.blockSequence} ${h(item.chapter)} / ${h(item.taskNumber)} · `:''}${h(item.target)}</button>`).join('');
  let checks=view.case.checks||[];if(state.checkFilter==='failed')checks=checks.filter(item=>!item.passed);
  $('#checks').innerHTML=checks.map(item=>`<button class="row check-row ${item.passed?'pass':'fail'}" data-focus-block="${h(item.blockId||'')}">${item.passed?'✓':'✕'} <b>${h(item.code)}</b> ${h(item.message)}${item.blockSequence?`<small>#${item.blockSequence} ${h(item.chapter)} / ${h(item.taskNumber)} · ${h(item.sourceLocator)}</small>`:''}</button>`).join('')||(view.case.checksStale?'<p class="stale">变更后原校核已清空，请重新校核。</p>':'<p class="muted">尚未执行校核</p>');
  document.querySelectorAll('[data-focus-block]').forEach(button=>button.onclick=()=>focusBlock(button.dataset.focusBlock));
}
function focusBlock(id){if(!id)return;const row=document.querySelector(`[data-block-id="${CSS.escape(id)}"]`);if(!row)return;row.scrollIntoView({behavior:'smooth',block:'center'});row.classList.add('focused');setTimeout(()=>row.classList.remove('focused'),1800);}

function renderBlocks(view,allowed){
  const editable=allowed.has('edit_change'),sortable=allowed.has('reorder_changes');
  $('#sort-hint').textContent=sortable?'拖动条款可一次性保存连续顺序':'';
  $('#blocks').innerHTML=view.currentBlocks.map(block=>`<div class="row block-row" data-block-id="${h(block.id)}" draggable="${sortable}"><div class="article-head"><b>${block.sequence}. ${h(block.chapter)} / ${h(block.taskNumber)}</b><span>${editable?`<button data-edit-block="${h(block.id)}">编辑</button><button data-delete-block="${h(block.id)}">删除</button>`:''}</span></div><span class="tag">${h(block.sourceLocator)}</span><p>${h(block.replacementText)}</p><small>${h(block.engineeringReference||'缺工程依据')} · ${h(block.approvalReference||'缺批准引用')} · ${h(block.configurationScope||'缺构型')}</small></div>`).join('')||'<p class="muted">本轮尚无变更块</p>';
  document.querySelectorAll('[data-edit-block]').forEach(button=>button.onclick=()=>openForm('edit',view.currentBlocks.find(item=>item.id===button.dataset.editBlock)));
  document.querySelectorAll('[data-delete-block]').forEach(button=>button.onclick=()=>deleteBlock(button.dataset.deleteBlock));
  if(sortable)enableDragSort();
}
function enableDragSort(){
  let dragged=null;
  document.querySelectorAll('.block-row').forEach(row=>{
    row.ondragstart=()=>{dragged=row;row.classList.add('dragging');};row.ondragend=()=>row.classList.remove('dragging');
    row.ondragover=event=>{event.preventDefault();if(dragged&&dragged!==row){const box=row.getBoundingClientRect();$('#blocks').insertBefore(dragged,event.clientY<box.top+box.height/2?row:row.nextSibling);}};
    row.ondrop=event=>{event.preventDefault();const ids=[...document.querySelectorAll('.block-row')].map(item=>item.dataset.blockId);reorderBlocks(ids);};
  });
}

function renderFindings(view,allowed){
  const revision=view.case;
  $('#findings').innerHTML=revision.findings.map(finding=>`<div class="row"><span class="tag">${h(severityLabel[finding.severity]||finding.severity)}</span><b>${h(finding.description)}</b><p>${h(finding.requiredAction)}</p><small>${h(finding.reviewDecision)}${finding.remediationNote?' · '+h(finding.remediationNote):''}${finding.rejectionReason?' · 退回：'+h(finding.rejectionReason):''}</small>${revision.status==='remediation'&&finding.reviewDecision!=='verified'?`<p><button data-link="${h(finding.id)}">关联新整改</button></p>`:''}</div>`).join('')||'<p class="muted">尚无技术审查问题</p>';
  document.querySelectorAll('[data-link]').forEach(button=>button.onclick=()=>linkFinding(button.dataset.link));
  const panel=$('#review-panel');panel.hidden=revision.status!=='remediation';if(panel.hidden)return;
  $('#review-queue').innerHTML=(view.findingReviewQueue||[]).map(item=>{const finding=item.finding,ready=item.status==='pending_review';return `<div class="row review-item severity-${h(finding.severity)}"><label><input type="checkbox" data-review-id="${h(finding.id)}" ${ready?'':'disabled'}> <b>${h(severityLabel[finding.severity])} · ${h(finding.id)}</b> <span class="tag">${h(queueLabel[item.status])}</span></label><p>${h(finding.description)}</p>${ready?`<select data-decision="${h(finding.id)}"><option value="verified">通过</option><option value="rejected">退回</option></select><input data-reason="${h(finding.id)}" placeholder="退回时填写具体原因">`:''}</div>`;}).join('')||'<p class="muted">当前整改轮没有复核问题</p>';
  $('#batch-review').hidden=!allowed.has('batch_review_findings');
}

function renderNotice(revision){
  $('#notice-card').hidden=!revision.notice;if(!revision.notice)return;
  const notice=revision.notice;$('#notice').innerHTML=`<h2>${h(notice.serialNumber)}</h2><p>${h(notice.contentSummary)}</p><p>${h(notice.scopeSummary)}</p><p>${new Date(notice.effectiveFrom).toLocaleString()} 至 ${new Date(notice.effectiveUntil).toLocaleString()}</p><p>批准人：${h(notice.approvedBy)} · 冻结第 ${notice.frozenRevisionIndex} 轮</p><p class="mono">校验码 ${h(notice.verificationCode)}</p><button id="verify-current-notice">现场核验</button>`;
  $('#verify-current-notice').onclick=()=>verifyNotice(notice.id,notice.verificationCode);
}

const changeFields=[['id','变更块 ID'],['chapter','章节'],['taskNumber','任务号'],['sourceLocator','原文定位'],['replacementText','替换内容','textarea'],['warningText','警告提示'],['affectedProcedure','受影响工序'],['engineeringReference','工程依据'],['approvalReference','批准引用'],['configurationScope','适用构型']];
const fields={case:[['manualNumber','手册编号'],['baselineEdition','基线版次'],['aircraftModels','机型（逗号分隔）'],['configurationScope','适用构型'],['reason','修订原因'],['owner','负责人'],['effectiveUntil','有效期','datetime-local']],change:changeFields,edit:changeFields,finding:[['id','问题 ID'],['changeBlockId','送审变更块 ID'],['severity','分级（minor/major/blocking）'],['description','问题描述'],['requiredAction','整改要求']]};
function openForm(type,values={}){
  const dialog=$('#editor');dialog.dataset.type=type;dialog.dataset.key=key();dialog.dataset.blockId=values.id||'';$('#form-title').textContent={case:'新建临时修订',change:'添加变更块',edit:'编辑变更块',finding:'登记审查问题'}[type];
  $('#form-fields').innerHTML=fields[type].map(([name,label,inputType])=>`<label for="${name}">${label}</label>${inputType==='textarea'?`<textarea id="${name}" name="${name}">${h(values[name]||'')}</textarea>`:`<input id="${name}" name="${name}" type="${inputType||'text'}" value="${h(values[name]||'')}" ${type==='edit'&&name==='id'?'readonly':''}>`}`).join('');dialog.showModal();
}
$('#editor-form').onsubmit=async event=>{
  event.preventDefault();const dialog=$('#editor'),type=dialog.dataset.type,data=Object.fromEntries(new FormData(event.target));data.idempotencyKey=dialog.dataset.key;data.actor='浏览器工作台用户';
  if(type==='case'){data.aircraftModels=data.aircraftModels.split(',').map(item=>item.trim());data.effectiveUntil=new Date(data.effectiveUntil).toISOString();}
  else Object.assign(data,meta(dialog.dataset.key));
  try{
    let path='/api/cases',method='POST';if(type!=='case'){path=`/api/cases/${encodeURIComponent(state.current.case.id)}/${type==='finding'?'findings':'changes'}`;}if(type==='edit'){path+=`/${encodeURIComponent(dialog.dataset.blockId)}`;method='PUT';}
    const body=await api(path,{method,body:JSON.stringify(data)});dialog.close();await loadCases();await openCase(body.data.id||body.data.caseId);flash('保存成功');
  }catch(error){flash(error.code==='version_conflict'?'并发冲突：表单内容已保留，请刷新任务后重试。':error.message,true);}
};

async function deleteBlock(id){
  if(!confirm(`确认删除变更块 ${id}？`))return;try{const body=await api(`/api/cases/${encodeURIComponent(state.current.case.id)}/changes/${encodeURIComponent(id)}`,{method:'DELETE',body:JSON.stringify(meta())});await openCase(body.data.id);flash('变更块已删除，原校核结果已失效');}catch(error){flash(error.message,true);}
}
async function reorderBlocks(ids){
  state.pendingOrder=ids;state.pendingOrderKey=state.pendingOrderKey||key();try{const body=await api(`/api/cases/${encodeURIComponent(state.current.case.id)}/changes/reorder`,{method:'POST',body:JSON.stringify({...meta(state.pendingOrderKey),blockIds:ids})});state.pendingOrder=null;state.pendingOrderKey=null;$('#retry-order').hidden=true;await openCase(body.data.id);flash('顺序已连续保存，原校核结果已失效');}catch(error){$('#retry-order').hidden=false;flash((error.code==='version_conflict'?'并发冲突；':'')+'当前拖动顺序已保留，可刷新任务后重试。',true);}
}

document.querySelectorAll('[data-action]').forEach(button=>button.onclick=async()=>{
  const action=button.dataset.action,revision=state.current.case;let path=action,body=meta();if(action==='start_remediation'){path='remediation';body={...body,reason:'按技术审查意见创建整改轮次'};}if(action==='request_approval')path='request-approval';if(action==='approve')body={...body,approver:revision.owner};
  try{const result=await api(`/api/cases/${encodeURIComponent(revision.id)}/${path}`,{method:'POST',body:JSON.stringify(body)});await loadCases();await openCase(result.data.id);flash('操作已完成');}catch(error){if(error.details)flash(error.details.map(item=>item.target).join('；'),true);else flash(error.message,true);}
});
document.querySelectorAll('[data-open]').forEach(button=>button.onclick=()=>openForm(button.dataset.open));
$('#new-case').onclick=()=>openForm('case');
$('#cancel-editor').onclick=()=>$('#editor').close();
$('#refresh-editor').onclick=async()=>{if(!state.current)return;try{await openCase(state.current.case.id);flash('已刷新任务版本，表单内容仍保留，可直接重试保存');}catch(error){flash(error.message,true);}};
$('#retry-order').onclick=async()=>{if(!state.pendingOrder)return;try{await openCase(state.current.case.id);await reorderBlocks(state.pendingOrder);}catch(error){flash(error.message,true);}};
$('#show-all-checks').onclick=()=>{state.checkFilter='all';renderChecks(state.current);};
$('#show-failed-checks').onclick=()=>{state.checkFilter='failed';renderChecks(state.current);};

async function linkFinding(id){const blockId=prompt('请输入当前整改轮次的新变更块 ID');if(!blockId)return;const note=prompt('请输入整改说明');if(!note)return;try{const result=await api(`/api/cases/${encodeURIComponent(state.current.case.id)}/link-remediation`,{method:'POST',body:JSON.stringify({...meta(),findingId:id,changeBlockId:blockId,note})});await openCase(result.data.id);flash('整改已关联');}catch(error){flash(error.message,true);}}
$('#batch-review').onclick=async()=>{const reviewer=$('#batch-reviewer').value.trim(),selected=[...document.querySelectorAll('[data-review-id]:checked')];const conclusions=selected.map(box=>{const id=box.dataset.reviewId;return{findingId:id,decision:document.querySelector(`[data-decision="${CSS.escape(id)}"]`).value,rejectionReason:document.querySelector(`[data-reason="${CSS.escape(id)}"]`).value};});try{const result=await api(`/api/cases/${encodeURIComponent(state.current.case.id)}/findings/batch-review`,{method:'POST',body:JSON.stringify({...meta(),reviewer,conclusions})});await loadCases();await openCase(result.data.id);flash('批量复核结论已原子保存');}catch(error){flash(error.message,true);}};

$('#notice-search').onsubmit=async event=>{event.preventDefault();const params=new URLSearchParams();for(const [name,value] of new FormData(event.target)){if(value.trim())params.set(name,value.trim());}try{const body=await api('/api/notices?'+params.toString());const items=body.data.items||[];$('#notice-results').innerHTML=items.map(item=>`<button data-notice-case="${h(item.caseId)}"><b>${h(item.notice.serialNumber)}</b><small>${h(item.manualNumber)} · ${item.aircraftModels.map(h).join('、')} · ${h(item.configurationScope)}</small></button><button data-verify-notice="${h(item.notice.id)}">核验</button>`).join('')||'<p class="muted">未匹配到生效通知</p>';document.querySelectorAll('[data-notice-case]').forEach(button=>button.onclick=()=>openCase(button.dataset.noticeCase));document.querySelectorAll('[data-verify-notice]').forEach(button=>button.onclick=()=>verifyNotice(button.dataset.verifyNotice,prompt('请输入现场校验码')||''));}catch(error){$('#notice-results').textContent=error.message;}};
async function verifyNotice(id,code){try{const body=await api(`/api/notices/${encodeURIComponent(id)}/verify?verificationCode=${encodeURIComponent(code)}`);const verification=body.data.verification;alert(`核验结论：${noticeLabel[verification.status]}\n摘要一致：${verification.digestMatches?'是':'否'}\n现场可用：${body.data.fieldUsable?'是':'否'}`);}catch(error){flash(error.message,true);}}

fetch('/healthz').then(response=>response.ok&&($('#health').textContent='● 服务正常'));
loadCases().catch(error=>flash(error.message,true));
