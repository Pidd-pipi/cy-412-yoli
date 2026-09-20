<template>
  <section>
    <header class="page-head">
      <div>
        <p class="eyebrow">VISITOR PASS</p>
        <h2>访客通行</h2>
        <p>登记车牌、到访时间与楼栋房号生成待审申请；物业批准后同一车牌重叠时段仅一张有效通行证。</p>
      </div>
      <el-button type="primary" @click="dialog=true">登记访客</el-button>
    </header>

    <div class="toolbar">
      <el-select v-model="status" placeholder="全部状态" clearable @change="load" style="width:180px">
        <el-option v-for="(t,k) in visitorStatusText" :key="k" :label="t" :value="k"/>
      </el-select>
      <el-button @click="load">刷新</el-button>
    </div>

    <div class="visitor-list">
      <VisitorPassCard v-for="v in items" :key="v.id" :data="v" :staff-view="isStaff">
        <template #actions>
          <template v-if="isStaff && v.status==='pending'">
            <el-button size="small" type="success" :loading="acting===v.id" @click="approve(v.id)">批准</el-button>
            <el-button size="small" type="danger" @click="openReject(v.id)">拒绝</el-button>
          </template>
          <el-button
            v-if="canRevoke(v)"
            size="small"
            :loading="acting===v.id"
            @click="revoke(v.id)">撤销</el-button>
        </template>
      </VisitorPassCard>
      <EmptyState v-if="!items.length"/>
    </div>

    <el-dialog v-model="dialog" title="登记访客通行申请" width="460px">
      <el-form label-width="92px">
        <el-form-item label="车牌号" required>
          <el-input v-model="form.plate" placeholder="如 粤B8A888"/>
        </el-form-item>
        <el-form-item label="访客姓名">
          <el-input v-model="form.visitor_name" placeholder="可选"/>
        </el-form-item>
        <el-form-item label="楼栋" required>
          <el-input v-model="form.building" placeholder="如 1栋"/>
        </el-form-item>
        <el-form-item label="房号" required>
          <el-input v-model="form.room" placeholder="如 2单元802"/>
        </el-form-item>
        <el-form-item label="到访开始" required>
          <el-date-picker v-model="form.startAt" type="datetime" placeholder="选择开始时间" value-format="YYYY-MM-DDTHH:mm:ssZ"/>
        </el-form-item>
        <el-form-item label="到访结束" required>
          <el-date-picker v-model="form.endAt" type="datetime" placeholder="选择结束时间" value-format="YYYY-MM-DDTHH:mm:ssZ"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog=false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submit">提交申请</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="rejectVisible" title="拒绝访客申请" width="400px">
      <el-input v-model="rejectReason" type="textarea" :rows="3" placeholder="请输入拒绝原因（将展示给申请人）"/>
      <template #footer>
        <el-button @click="rejectVisible=false">取消</el-button>
        <el-button type="danger" :loading="acting===rejectId" @click="confirmReject">确认拒绝</el-button>
      </template>
    </el-dialog>
  </section>
</template>

<script setup lang="ts">
import{ref,reactive,computed,onMounted}from'vue';
import{ElMessage}from'element-plus';
import{listVisitorPasses,createVisitorPass,approveVisitorPass,rejectVisitorPass,revokeVisitorPass}from'../api/visitor';
import{visitorStore}from'../stores/visitorStore';
import{authStore}from'../stores/authStore';
import{visitorStatusText,VISITOR_STATUS}from'../constants/visitor';
import type{VisitorPass}from'../types';
import VisitorPassCard from'../components/common/VisitorPassCard.vue';
import EmptyState from'../components/common/EmptyState.vue';

const items=computed(()=>visitorStore.items);
const status=ref('');
const dialog=ref(false);
const submitting=ref(false);
const acting=ref<number|null>(null);
const isStaff=computed(()=>authStore.user?.role==='staff'||authStore.user?.role==='admin');
const form=reactive({plate:'',visitor_name:'',building:authStore.user?.building||'',room:authStore.user?.room||'',startAt:'',endAt:''});
const rejectVisible=ref(false);
const rejectId=ref<number>(0);
const rejectReason=ref('');

async function load(){visitorStore.items=await listVisitorPasses(status.value||undefined)}

async function submit(){
  if(!form.plate||!form.building||!form.room||!form.startAt||!form.endAt){
    ElMessage.warning('请完整填写车牌、楼栋房号与到访时段');return;
  }
  submitting.value=true;
  try{
    await createVisitorPass({plate:form.plate,visitor_name:form.visitor_name,building:form.building,room:form.room,start_at:form.startAt,end_at:form.endAt});
    ElMessage.success('访客通行申请已提交，等待物业审批');
    dialog.value=false;
    Object.assign(form,{plate:'',visitor_name:'',building:authStore.user?.building||'',room:authStore.user?.room||'',startAt:'',endAt:''});
    await load();
  }catch(e){ElMessage.error((e as Error).message)}finally{submitting.value=false}
}

async function approve(id:number){
  acting.value=id;
  try{
    await approveVisitorPass(id);
    ElMessage.success('通行证已批准');
  }catch(e){
    // 冲突/重复审批：后端消息即失败原因，页面回读列表保持待审
    ElMessage.error((e as Error).message);
  }finally{acting.value=null;await load()}
}

function openReject(id:number){rejectId.value=id;rejectReason.value='';rejectVisible.value=true}
async function confirmReject(){
  if(rejectReason.value.trim().length<2){ElMessage.warning('请填写拒绝原因');return}
  acting.value=rejectId.value;
  try{
    await rejectVisitorPass(rejectId.value,rejectReason.value.trim());
    ElMessage.success('申请已拒绝');rejectVisible.value=false;
  }catch(e){ElMessage.error((e as Error).message)}finally{acting.value=null;await load()}
}

async function revoke(id:number){
  acting.value=id;
  try{
    await revokeVisitorPass(id);
    ElMessage.success('通行证已撤销，到访时段已释放');
  }catch(e){ElMessage.error((e as Error).message)}finally{acting.value=null;await load()}
}

// 仅“已批准且到访尚未开始”可撤销（业主撤销本人、物业撤销任意），过期后撤销入口消失。
function canRevoke(v:VisitorPass){
  return v.status===VISITOR_STATUS.APPROVED&&v.effective_state==='upcoming'&&
    (isStaff.value||v.user_id===authStore.user?.id);
}

onMounted(load);
</script>
