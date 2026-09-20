import {reactive}from'vue';import type{VisitorPass}from'../types';export const visitorPassStore=reactive<{items:VisitorPass[]}>({items:[]});
