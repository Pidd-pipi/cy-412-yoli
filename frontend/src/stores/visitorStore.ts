import {reactive}from'vue';import type{VisitorPass}from'../types';export const visitorStore=reactive<{items:VisitorPass[]}>({items:[]});
