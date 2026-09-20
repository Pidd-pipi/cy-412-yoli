<template>
  <article class="card visitor-card">
    <div class="visitor-main">
      <div class="visitor-title">
        <h3>{{data.plate}}</h3>
        <el-tag :type="tagType" effect="light">{{visitorStatusText[data.status]}}</el-tag>
        <el-tag v-if="data.effective_state" size="small" :type="stateTagType">{{visitorStateText[data.effective_state]}}</el-tag>
      </div>
      <p class="visitor-meta">
        <span>{{data.building}} {{data.room}}</span>
        <span v-if="data.visitor_name"> · 访客 {{data.visitor_name}}</span>
        <span v-if="data.user?.nickname && staffView"> · 申请人 {{data.user.nickname}}</span>
      </p>
      <p class="visitor-window">有效时段：{{fmt(data.start_at)}} ~ {{fmt(data.end_at)}}</p>
      <p v-if="data.failure_reason" class="visitor-reason">失败原因：{{data.failure_reason}}</p>
    </div>
    <div class="visitor-actions">
      <template v-if="$slots.actions">
        <slot name="actions"/>
      </template>
    </div>
  </article>
</template>
<script setup lang="ts">
import{computed}from'vue';
import type{VisitorPass}from'../../types';
import{visitorStatusText,visitorStateText,visitorStatusTagType,visitorStateTagType}from'../../constants/visitor';
const p=defineProps<{data:VisitorPass;staffView?:boolean}>();
const fmt=(v:string)=>new Date(v).toLocaleString();
const tagType=computed(()=>visitorStatusTagType[p.data.status]);
const stateTagType=computed(()=>p.data.effective_state?visitorStateTagType[p.data.effective_state]:'info');
</script>
